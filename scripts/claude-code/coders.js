/**
 * Coder Spawner Skill for Claude Code
 * 
 * Spawn AI coding assistants in isolated tmux sessions with optional git worktrees.
 * 
 * Usage:
 * import { coders } from '@jayphen/coders';
 * await coders.spawn({ tool: 'claude', task: '...', worktree: 'feature-x', prd: 'docs/prd.md' });
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
    return `Worktree: ${worktreePath}`;
  } catch (e: any) {
    return `Worktree exists or failed: ${e.message}`;
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

export async function spawn(options: {
  tool: 'claude' | 'gemini' | 'codex';
  task: string;
  name?: string;
  worktree?: string;
  baseBranch?: string;
  prd?: string;
}): Promise<string> {
  const { tool, task, name, worktree, baseBranch = 'main', prd } = options;
  
  const sessionName = name || `${tool}-${Date.now()}`;
  const sessionId = `${SESSION_PREFIX}${sessionName}`;
  
  let worktreePath: string | undefined;
  if (worktree) {
    const gitRoot = getGitRoot();
    worktreePath = path.join(gitRoot, WORKTREE_BASE, worktree);
    try {
      fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
      execSync(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
    } catch {}
  }
  
  const contextFiles = prd ? [prd] : [];
  const prompt = createPrompt(task, contextFiles);
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);
  
  const cmd = buildCommand(tool, promptFile, worktreePath);
  
  try {
    try { execSync(`tmux kill-session -t ${sessionId}`); } catch {}
    execSync(`tmux new-session -s "${sessionId}" -d "${cmd}"`);
    
    return `
Spawned ${tool} in tmux session: ${sessionId}
Worktree: ${worktreePath || 'main repo'}
Task: ${task}

Attach: \`tmux attach -t ${sessionId}\`
`;
  } catch (e: any) {
    return `Failed: ${e.message}`;
  }
}

export function list(): string {
  try {
    const sessions = execSync('tmux list-sessions 2>/dev/null', { encoding: 'utf8' })
      .split('\n').filter((s: string) => s.includes(SESSION_PREFIX));
    return sessions.length ? 'Sessions:\n' + sessions.join('\n') : 'No sessions.';
  } catch {
    return 'tmux not available';
  }
}

export function attach(sessionName: string): string {
  return `Run: tmux attach -t ${SESSION_PREFIX}${sessionName}`;
}

export function kill(sessionName: string): string {
  try {
    execSync(`tmux kill-session -t ${SESSION_PREFIX}${sessionName}`);
    return `Killed: ${sessionName}`;
  } catch (e: any) {
    return `Failed: ${e.message}`;
  }
}

export const claude = (task: string, options?: { name?: string; worktree?: string; prd?: string }) => 
  spawn({ tool: 'claude', task, ...options });

export const gemini = (task: string, options?: { name?: string; worktree?: string; prd?: string }) => 
  spawn({ tool: 'gemini', task, ...options });

export const codex = (task: string, options?: { name?: string; worktree?: string; prd?: string }) => 
  spawn({ tool: 'codex', task, ...options });

export const coders = { spawn, list, attach, kill, claude, gemini, codex, createWorktree };
export default coders;
