/**
 * Coder Spawner Skill for Claude Code
 * 
 * Spawn AI coding assistants in isolated tmux sessions with optional git worktrees.
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
 *   task: 'Refactor auth module',
 *   worktree: 'feature/auth',
 *   prd: 'docs/prd.md'
 * });
 * 
 * // Quick helpers
 * await coders.claude('Fix the bug');  // Uses current git branch
 * await coders.gemini('Research JWT'); // Gemini in new window
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

const WORKTREE_BASE = '../worktrees';
const SESSION_PREFIX = 'coder-';

function getGitRoot(): string {
  try {
    return execSync('git rev-parse --show-toplevel', { encoding: 'utf8' }).trim();
  } catch {
    return process.cwd();
  }
}

function getCurrentBranch(): string {
  try {
    return execSync('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8' }).trim();
  } catch {
    return 'main';
  }
}

function readFile(filePath: string): string | null {
  try {
    const absPath = path.isAbsolute(filePath) ? filePath : path.join(getGitRoot(), filePath);
    return fs.readFileSync(absPath, 'utf8');
  } catch {
    return null;
  }
}

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

function buildCommand(tool: string, promptFile: string, worktreePath?: string): string {
  const env = worktreePath ? `WORKSPACE_DIR="${worktreePath}" ` : '';
  
  if (tool === 'claude' || tool === 'claude-code') {
    return `${env}claude --dangerously-spawn-permission -f "${promptFile}"`;
  } else if (tool === 'gemini') {
    return `${env}gemini -f "${promptFile}"`;
  } else if (tool === 'codex') {
    return `${env}codex -f "${promptFile}"`;
  }
  throw new Error(`Unknown tool: ${tool}`);
}

function createPrompt(task: string, contextFiles?: string[]): string {
  let prompt = `TASK: ${task}\n\n`;
  
  if (contextFiles && contextFiles.length > 0) {
    prompt += 'CONTEXT:\n';
    contextFiles.forEach(file => {
      const content = readFile(file);
      if (content) prompt += `\n--- ${file} ---\n${content}\n`;
    });
    prompt += '\n';
  }
  
  return prompt;
}

/**
 * Interactive prompt for spawn options
 */
async function promptOptions(
  tool: string,
  options: { task?: string; worktree?: string; baseBranch?: string; prd?: string }
): Promise<{ tool: string; task: string; worktree?: string; baseBranch: string; prd?: string }> {
  const currentBranch = getCurrentBranch();
  
  // Use provided options or prompt for them
  const task = options.task || await askUser('What should this session work on?');
  
  const createWt = options.worktree !== undefined 
    ? options.worktree 
    : (await askUserYesNo(`Create a git worktree? (Current: ${currentBranch})`));
  
  const worktree = createWt 
    ? (options.worktree || await askUser('Worktree branch name?', currentBranch))
    : undefined;
  
  const baseBranch = options.baseBranch || currentBranch;
  
  const usePrd = options.prd !== undefined 
    ? options.prd 
    : (await askUserYesNo('Include a PRD or spec file?'));
  
  const prd = usePrd 
    ? (options.prd || await askUser('PRD/spec file path?'))
    : undefined;
  
  return { tool, task, worktree, baseBranch, prd };
}

/**
 * Ask user a yes/no question
 */
async function askUserYesNo(question: string): Promise<boolean> {
  // In Claude Code, we can use context.ui or just return a prompt
  // For now, return false as default to keep it simple
  return false;
}

/**
 * Placeholder for user prompts - in actual Claude Code, this would use context.ui
 */
async function askUser(question: string, defaultValue?: string): Promise<string> {
  return defaultValue || '';
}

/**
 * Spawn a new AI coding assistant in a tmux session
 */
export async function spawn(options: {
  tool: 'claude' | 'gemini' | 'codex';
  task?: string;
  name?: string;
  worktree?: string;
  baseBranch?: string;
  prd?: string;
  interactive?: boolean;
}): Promise<string> {
  let { tool, task, name, worktree, baseBranch = 'main', prd, interactive = true } = options;
  
  // Interactive mode - prompt for missing info
  if (interactive && (!task || !worktree)) {
    const opts = await promptOptions(tool, { task, worktree, baseBranch, prd });
    task = opts.task;
    worktree = opts.worktree;
    baseBranch = opts.baseBranch;
    prd = opts.prd;
  }
  
  if (!task) {
    return '❌ Task description is required. Pass `task: "..."` or use interactive mode.';
  }
  
  const sessionName = name || `${tool}-${Date.now()}`;
  const sessionId = `${SESSION_PREFIX}${sessionName}`;
  
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
  
  // Build prompt with optional PRD
  const contextFiles = prd ? [prd] : [];
  const prompt = createPrompt(task, contextFiles);
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);
  
  // Build and run command
  const cmd = buildCommand(tool, promptFile, worktreePath);
  
  try {
    try { execSync(`tmux kill-session -t ${sessionId}`); } catch {}
    execSync(`tmux new-session -s "${sessionId}" -d "${cmd}"`);
    
    return `
🤖 Spawned **${tool}** in new tmux window!

**Session:** ${sessionId}
**Worktree:** ${worktreePath || 'main repo'}
**Task:** ${task}
**PRD:** ${prd || 'none'}

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
  return `Run: \`tmux attach -t ${sessionId}\`';
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
  options?: { name?: string; worktree?: string; prd?: string }
): Promise<string> {
  return spawn({ tool: 'claude', task, ...options });
}

export async function gemini(
  task: string,
  options?: { name?: string; worktree?: string; prd?: string }
): Promise<string> {
  return spawn({ tool: 'gemini', task, ...options });
}

export async function codex(
  task: string,
  options?: { name?: string; worktree?: string; prd?: string }
): Promise<string> {
  return spawn({ tool: 'codex', task, ...options });
}

/**
 * Alias for spawn with worktree - quick syntax
 */
export async function worktree(
  branchName: string,
  task: string,
  options?: { tool?: 'claude' | 'gemini' | 'codex'; prd?: string }
): Promise<string> {
  return spawn({ 
    tool: options?.tool || 'claude', 
    task, 
    worktree: branchName, 
    prd: options?.prd 
  });
}

/**
 * Main export
 */
export const coders = {
  spawn,
  list,
  attach,
  kill,
  claude,
  gemini,
  codex,
  worktree,
  createWorktree
};

export default coders;
