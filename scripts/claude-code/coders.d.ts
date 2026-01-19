/**
 * Type definitions for Coder Spawner
 */

export interface SpawnOptions {
  /** AI tool to spawn: claude, gemini, or codex */
  tool: 'claude' | 'gemini' | 'codex';
  /** Task description for the AI to work on */
  task?: string;
  /** Custom session name */
  name?: string;
  /** Git branch name for worktree (optional) */
  worktree?: string;
  /** Base branch for worktree (default: main) */
  baseBranch?: string;
  /** PRD or spec file path to include as context */
  prd?: string;
  /** Enable interactive prompts for missing info (default: true) */
  interactive?: boolean;
}

export interface CoderSession {
  /** Session ID */
  id: string;
  /** AI tool (claude, gemini, codex) */
  tool: string;
  /** Worktree path if applicable */
  worktree?: string;
  /** Task description */
  task: string;
  /** When the session was created */
  createdAt: Date;
}

/**
 * Coder Spawner - Spawn AI coding assistants in isolated tmux sessions
 */
export const coders: {
  /** Spawn a new AI coding assistant in a tmux session */
  spawn: (options: SpawnOptions) => Promise<string>;
  /** List all active coder sessions */
  list: () => string;
  /** Get instructions to attach to a session */
  attach: (sessionName: string) => string;
  /** Kill a coder session */
  kill: (sessionName: string) => string;
  /** Quick spawn Claude */
  claude: (task: string, options?: Omit<SpawnOptions, 'tool'>) => Promise<string>;
  /** Quick spawn Gemini */
  gemini: (task: string, options?: Omit<SpawnOptions, 'tool'>) => Promise<string>;
  /** Quick spawn Codex */
  codex: (task: string, options?: Omit<SpawnOptions, 'tool'>) => Promise<string>;
  /** Quick spawn with worktree */
  worktree: (branchName: string, task: string, options?: { tool?: 'claude' | 'gemini' | 'codex'; prd?: string }) => Promise<string>;
  /** Create a git worktree */
  createWorktree: (branchName: string, baseBranch?: string) => string;
  /** Get all active sessions */
  getActiveSessions: () => CoderSession[];
};

export default coders;

/**
 * Quick spawn helpers
 */
export function claude(task: string, options?: Omit<SpawnOptions, 'tool'>): Promise<string>;
export function gemini(task: string, options?: Omit<SpawnOptions, 'tool'>): Promise<string>;
export function codex(task: string, options?: Omit<SpawnOptions, 'tool'>): Promise<string>;
export function worktree(branchName: string, task: string, options?: { tool?: 'claude' | 'gemini' | 'codex'; prd?: string }): Promise<string>;
