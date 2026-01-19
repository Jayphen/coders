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
 *   task: 'Refactor the authentication module',
 *   worktree: 'feature/auth-refactor',
 *   prd: 'docs/auth-prd.md'
 * });
 *
 * // Quick helpers
 * await coders.claude('Fix the bug', { worktree: 'fix-auth' });
 * await coders.gemini('Research JWT approaches');
 */
interface SpawnOptions {
    tool: 'claude' | 'gemini' | 'codex';
    task?: string;
    name?: string;
    worktree?: string;
    baseBranch?: string;
    prd?: string;
    interactive?: boolean;
}
interface CoderSession {
    id: string;
    tool: string;
    worktree?: string;
    task: string;
    createdAt: Date;
}
/**
 * Create a git worktree for the given branch
 */
declare function createWorktree(branchName: string, baseBranch?: string): string;
/**
 * Spawn a new AI coding assistant in a tmux session
 */
export declare function spawn(options: SpawnOptions): Promise<string>;
/**
 * List all active coder sessions
 */
export declare function list(): string;
/**
 * Attach to a coder session
 */
export declare function attach(sessionName: string): string;
/**
 * Kill a coder session
 */
export declare function kill(sessionName: string): string;
/**
 * Quick spawn helpers - minimal options for speed
 */
export declare function claude(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
}): Promise<string>;
export declare function gemini(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
}): Promise<string>;
export declare function codex(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
}): Promise<string>;
/**
 * Alias for spawn with worktree - quick syntax
 */
export declare function worktree(branchName: string, task: string, options?: {
    tool?: 'claude' | 'gemini' | 'codex';
    prd?: string;
}): Promise<string>;
/**
 * Get all active coder sessions
 */
export declare function getActiveSessions(): CoderSession[];
/**
 * Main export with all functions
 */
export declare const coders: {
    spawn: typeof spawn;
    list: typeof list;
    attach: typeof attach;
    kill: typeof kill;
    claude: typeof claude;
    gemini: typeof gemini;
    codex: typeof codex;
    worktree: typeof worktree;
    createWorktree: typeof createWorktree;
    getActiveSessions: typeof getActiveSessions;
};
export default coders;
//# sourceMappingURL=coders.d.ts.map