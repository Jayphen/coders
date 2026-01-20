/**
 * Coder Spawner Skill for Claude Code
 * 
 * Spawn AI coding assistants in isolated tmux sessions with optional git worktrees.
 * Supports Redis heartbeat, pub/sub for inter-agent communication, and auto-respawn.
 * 
 * Usage:
 * import { coders } from '@jayphen/coders';
 * 
 * // Interactive mode - asks for missing info
 * await coders.spawn({ tool: 'claude' });
 * 
 * // Direct mode - all options upfront
 * await coders.spawn({
 *   tool: 'claude',
 *   task: 'Refactor the authentication module',
 *   worktree: 'feature/auth-refactor',
 *   prd: 'docs/auth-prd.md'
 * });
 * 
 * // With Redis heartbeat enabled
 * await coders.spawn({
 *   tool: 'claude',
 *   task: 'Fix the bug',
 *   redis: { url: 'redis://localhost:6379' }
 * });
 * 
 * // Quick helpers
 * await coders.claude('Fix the bug', { worktree: 'fix-auth' });
 * await coders.gemini('Research JWT approaches');
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import {
  RedisManager,
  DeadLetterListener,
  getPaneId,
  injectPaneIdContext,
  RedisConfig,
  HEARTBEAT_CHANNEL,
  DEAD_LETTER_KEY
} from './redis';
import { generateSessionName } from '../session-name.js';

const WORKTREE_BASE = '../worktrees';
const SESSION_PREFIX = 'coder-';

// Types
export interface SpawnOptions {
  tool: 'claude' | 'gemini' | 'codex' | 'opencode';
  task?: string;
  name?: string;
  worktree?: string;
  baseBranch?: string;
  prd?: string;
  interactive?: boolean;
  redis?: RedisConfig;
  enableHeartbeat?: boolean;
  enableDeadLetter?: boolean;
  paneId?: string;
}

export interface CoderSession {
  id: string;
  tool: string;
  worktree?: string;
  task: string;
  createdAt: Date;
  paneId?: string;
}

export interface CodersConfig {
  redis?: RedisConfig;
  snapshotDir?: string;
  deadLetterTimeout?: number;
}

/**
 * Global config for coders skill
 */
let globalConfig: CodersConfig = {
  snapshotDir: '~/.coders/snapshots',
  deadLetterTimeout: 120000 // 2 minutes
};

/**
 * Configure the coders skill globally
 */
export function configure(config: CodersConfig): void {
  globalConfig = { ...globalConfig, ...config };
}

/**
 * Get the git root directory
 */
function getGitRoot(): string {
  try {
    return execSync('git rev-parse --show-toplevel', { encoding: 'utf8' }).trim();
  } catch {
    return process.cwd();
  }
}

/**
 * Get the current git branch
 */
function getCurrentBranch(): string {
  try {
    return execSync('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8' }).trim();
  } catch {
    return 'main';
  }
}

/**
 * Read file content
 */
function readFile(filePath: string): string | null {
  try {
    const absPath = path.isAbsolute(filePath) ? filePath : path.join(getGitRoot(), filePath);
    return fs.readFileSync(absPath, 'utf8');
  } catch {
    return null;
  }
}

/**
 * Create a git worktree for the given branch
 */
function createWorktree(branchName: string, baseBranch: string = 'main'): string {
  const gitRoot = getGitRoot();
  const worktreePath = path.join(gitRoot, WORKTREE_BASE, branchName);
  
  try {
    fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
    execSync(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
    return `✅ Worktree created: ${worktreePath}`;
  } catch (e: any) {
    return `⚠️  Worktree: ${e.message}`;
  }
}

/**
 * Build the spawn command for a tool with optional pane ID injection
 */
function buildCommand(
  tool: string, 
  promptFile: string, 
  worktreePath?: string,
  paneId?: string,
  redisConfig?: RedisConfig,
  sessionId?: string
): string {
  const envVars: string[] = [];
  
  if (worktreePath) {
    envVars.push(`WORKSPACE_DIR="${worktreePath}"`);
  }
  
  if (paneId) {
    envVars.push(`CODERS_PANE_ID="${paneId}"`);
  }

  if (sessionId) {
    envVars.push(`CODERS_SESSION_ID="${sessionId}"`);
  }
  
  if (redisConfig?.url) {
    envVars.push(`REDIS_URL="${redisConfig.url}"`);
  }
  
  const env = envVars.length > 0 ? envVars.join(' ') + ' ' : '';
  
  if (tool === 'claude' || tool === 'claude-code') {
    return `${env}claude --dangerously-spawn-permission -f "${promptFile}"`;
  } else if (tool === 'gemini') {
    return `${env}gemini -f "${promptFile}"`;
  } else if (tool === 'codex') {
    return `${env}codex -f "${promptFile}"`;
  } else if (tool === 'opencode') {
    return `${env}opencode -f "${promptFile}"`;
  }
  throw new Error(`Unknown tool: ${tool}`);
}

/**
 * Generate a prompt file with task, context, and pane ID injection
 */
function createPrompt(task: string, contextFiles?: string[], paneId?: string, redisConfig?: RedisConfig): string {
  let prompt = '';
  
  // Inject pane ID context if provided
  if (paneId) {
    prompt += injectPaneIdContext('', paneId);
  }
  
  prompt += `TASK: ${task}\n\n`;
  
  if (contextFiles && contextFiles.length > 0) {
    prompt += 'CONTEXT:\n';
    contextFiles.forEach(file => {
      const content = readFile(file);
      if (content) {
        prompt += `\n--- ${file} ---\n${content}\n`;
      }
    });
    prompt += '\n';
  }
  
  // Add Redis info to prompt if configured
  if (redisConfig?.url) {
    prompt += `
<!-- REDIS CONFIG -->
<!-- HEARTBEAT_CHANNEL: ${HEARTBEAT_CHANNEL} -->
<!-- DEAD_LETTER_KEY: ${DEAD_LETTER_KEY} -->

Redis is configured for this session. Publish heartbeats to enable
auto-respawn if you become unresponsive for >2 minutes.

Heartbeat format:
{
  "paneId": "${paneId || '<your-pane-id)'}",
  "status": "alive",
  "timestamp": Date.now()
}

Publish to Redis channel: ${HEARTBEAT_CHANNEL}
`;
  }
  
  return prompt;
}

/**
 * Start the dead-letter listener for a session
 */
function startDeadLetterListener(redisConfig: RedisConfig): DeadLetterListener | null {
  if (!redisConfig?.url) return null;
  
  const listener = new DeadLetterListener(redisConfig);
  listener.start().catch(e => {
    console.error('Failed to start dead-letter listener:', e);
  });
  
  return listener;
}

/**
 * Spawn a new AI coding assistant in a tmux session
 */
export async function spawn(options: SpawnOptions): Promise<string> {
  const { 
    tool, 
    task = '', 
    name, 
    worktree, 
    baseBranch = 'main', 
    prd,
    interactive = true,
    redis: redisConfig,
    enableHeartbeat = !!redisConfig?.url,
    enableDeadLetter = !!redisConfig?.url,
    paneId: providedPaneId
  } = options;
  
  if (!task) {
    return '❌ Task description is required. Pass `task: "..."` or use interactive mode.';
  }
  
  const sessionName = name || generateSessionName(tool, task);
  const sessionId = `${SESSION_PREFIX}${sessionName}`;
  const paneId = providedPaneId || getPaneId();
  
  // Create worktree if requested
  let worktreePath: string | undefined;
  if (worktree) {
    const gitRoot = getGitRoot();
    worktreePath = path.join(gitRoot, WORKTREE_BASE, worktree);
    try {
      fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
      execSync(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
    } catch (e: any) {
      // Worktree might exist
    }
  }
  
  // Build prompt with optional PRD and Redis context
  const contextFiles = prd ? [prd] : [];
  const prompt = createPrompt(task, contextFiles, paneId, redisConfig);
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);
  
  // Build command with environment variables
  const cmd = buildCommand(tool, promptFile, worktreePath, paneId, redisConfig, sessionId);
  
  // Start dead-letter listener if enabled
  let deadLetterListener: DeadLetterListener | null = null;
  if (enableDeadLetter && redisConfig) {
    deadLetterListener = startDeadLetterListener(redisConfig);
  }
  
  try {
    // Clean up existing session if any
    try { execSync(`tmux kill-session -t ${sessionId}`); } catch {}
    
    // Create new tmux session
    execSync(`tmux new-session -s "${sessionId}" -d "${cmd}"`);
    
    // Store pane info for Redis if configured
    if (redisConfig?.url) {
      const redis = new RedisManager(redisConfig);
      await redis.connect();
      await redis.setPaneId(paneId);
      if (enableHeartbeat) {
        redis.startHeartbeat();
      }
    }
    
    return `
🤖 Spawned **${tool}** in new tmux window!

**Session:** ${sessionId}
**Pane ID:** ${paneId}
**Worktree:** ${worktreePath || 'main repo'}
**Task:** ${task}
**PRD:** ${prd || 'none'}
**Redis:** ${redisConfig?.url || 'disabled'}
**Heartbeat:** ${enableHeartbeat ? 'enabled' : 'disabled'}
**Auto-Respawn:** ${enableDeadLetter ? 'enabled (2min timeout)' : 'disabled'}

To attach:
\`coders attach ${sessionName}\`
or
\`tmux attach -t ${sessionId}\`
`;
  } catch (e: any) {
    return `❌ Failed: ${e.message}`;
  }
}

/**
 * List all active coder sessions
 */
export function list(): string {
  try {
    const output = execSync('tmux list-sessions 2>/dev/null', { encoding: 'utf8' });
    const sessions = output.split('\n').filter((s: string) => s.includes(SESSION_PREFIX));
    
    if (sessions.length === 0) {
      return 'No coder sessions active.';
    }
    
    return '📋 Active Coder Sessions:\n\n' + sessions.join('\n');
  } catch {
    return 'tmux not available or no sessions';
  }
}

/**
 * Attach to a coder session
 */
export function attach(sessionName: string): string {
  const sessionId = `${SESSION_PREFIX}${sessionName}`;
  return `Run: \`tmux attach -t ${sessionId}\``;
}

/**
 * Kill a coder session
 */
export function kill(sessionName: string): string {
  const sessionId = `${SESSION_PREFIX}${sessionName}`;
  try {
    execSync(`tmux kill-session -t ${sessionId}`);
    return `✅ Killed session: ${sessionId}`;
  } catch (e: any) {
    return `❌ Failed: ${e.message}`;
  }
}

/**
 * Quick spawn helpers - minimal options for speed
 */
export async function claude(
  task: string,
  options?: { 
    name?: string; 
    worktree?: string; 
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
  }
): Promise<string> {
  return spawn({ tool: 'claude', task, ...options });
}

export async function gemini(
  task: string,
  options?: { 
    name?: string; 
    worktree?: string; 
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
  }
): Promise<string> {
  return spawn({ tool: 'gemini', task, ...options });
}

export async function codex(
  task: string,
  options?: { 
    name?: string; 
    worktree?: string; 
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
  }
): Promise<string> {
  return spawn({ tool: 'codex', task, ...options });
}

export async function opencode(
  task: string,
  options?: { 
    name?: string; 
    worktree?: string; 
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
  }
): Promise<string> {
  return spawn({ tool: 'opencode', task, ...options });
}

/**
 * Alias for spawn with worktree - quick syntax
 */
export async function worktree(
  branchName: string,
  task: string,
  options?: { 
    tool?: 'claude' | 'gemini' | 'codex'; 
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
  }
): Promise<string> {
  return spawn({ 
    tool: options?.tool || 'claude', 
    task, 
    worktree: branchName, 
    prd: options?.prd,
    redis: options?.redis,
    enableHeartbeat: options?.enableHeartbeat,
    enableDeadLetter: options?.enableDeadLetter
  });
}

/**
 * Get all active coder sessions
 */
export function getActiveSessions(): CoderSession[] {
  try {
    const output = execSync('tmux list-sessions 2>/dev/null', { encoding: 'utf8' });
    return output.split('\n')
      .filter((s: string) => s.includes(SESSION_PREFIX))
      .map((s: string) => {
        const match = s.match(/coder-([^:]+):/);
        return {
          id: match ? match[1] : 'unknown',
          tool: 'unknown',
          task: '',
          createdAt: new Date()
        } as CoderSession;
      });
  } catch {
    return [];
  }
}

/**
 * Spawn with Redis heartbeat enabled
 */
export async function spawnWithHeartbeat(
  options: SpawnOptions
): Promise<string> {
  return spawn({
    ...options,
    redis: options.redis || globalConfig.redis,
    enableHeartbeat: true,
    enableDeadLetter: true
  });
}

/**
 * Send message to another agent via Redis pub/sub
 */
export async function sendMessage(
  channel: string,
  message: any,
  redisConfig?: RedisConfig
): Promise<void> {
  const config = redisConfig || globalConfig.redis;
  if (!config?.url) {
    throw new Error('Redis not configured. Pass redis config or set global config.');
  }
  
  const redis = new RedisManager(config);
  await redis.connect();
  await redis.publishMessage(channel, message);
  await redis.disconnect();
}

/**
 * Listen for messages from other agents via Redis pub/sub
 */
export async function listenForMessages(
  channel: string,
  callback: (message: any) => void,
  redisConfig?: RedisConfig
): Promise<void> {
  const config = redisConfig || globalConfig.redis;
  if (!config?.url) {
    throw new Error('Redis not configured. Pass redis config or set global config.');
  }
  
  const redis = new RedisManager(config);
  await redis.subscribeToChannel(channel, callback);
}

/**
 * Main export with all functions
 */
export const coders = {
  spawn,
  spawnWithHeartbeat,
  list,
  attach,
  kill,
  claude,
  gemini,
  codex,
  opencode,
  worktree,
  createWorktree,
  getActiveSessions,
  configure,
  sendMessage,
  listenForMessages,
  // Re-export from redis.ts
  RedisManager,
  DeadLetterListener,
  getPaneId,
  injectPaneIdContext,
  HEARTBEAT_CHANNEL,
  DEAD_LETTER_KEY
};

export default coders;
