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
import { RedisManager, DeadLetterListener, getPaneId, injectPaneIdContext, RedisConfig } from './redis';
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
 * Configure the coders skill globally
 */
export declare function configure(config: CodersConfig): void;
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
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
}): Promise<string>;
export declare function gemini(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
}): Promise<string>;
export declare function codex(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
}): Promise<string>;
export declare function opencode(task: string, options?: {
    name?: string;
    worktree?: string;
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
}): Promise<string>;
/**
 * Alias for spawn with worktree - quick syntax
 */
export declare function worktree(branchName: string, task: string, options?: {
    tool?: 'claude' | 'gemini' | 'codex';
    prd?: string;
    redis?: RedisConfig;
    enableHeartbeat?: boolean;
    enableDeadLetter?: boolean;
}): Promise<string>;
/**
 * Get all active coder sessions
 */
export declare function getActiveSessions(): CoderSession[];
/**
 * Spawn with Redis heartbeat enabled
 */
export declare function spawnWithHeartbeat(options: SpawnOptions): Promise<string>;
/**
 * Send message to another agent via Redis pub/sub
 */
export declare function sendMessage(channel: string, message: any, redisConfig?: RedisConfig): Promise<void>;
/**
 * Listen for messages from other agents via Redis pub/sub
 */
export declare function listenForMessages(channel: string, callback: (message: any) => void, redisConfig?: RedisConfig): Promise<void>;
/**
 * Main export with all functions
 */
export declare const coders: {
    spawn: typeof spawn;
    spawnWithHeartbeat: typeof spawnWithHeartbeat;
    list: typeof list;
    attach: typeof attach;
    kill: typeof kill;
    claude: typeof claude;
    gemini: typeof gemini;
    codex: typeof codex;
    opencode: typeof opencode;
    worktree: typeof worktree;
    createWorktree: typeof createWorktree;
    getActiveSessions: typeof getActiveSessions;
    configure: typeof configure;
    sendMessage: typeof sendMessage;
    listenForMessages: typeof listenForMessages;
    RedisManager: typeof RedisManager;
    DeadLetterListener: typeof DeadLetterListener;
    getPaneId: typeof getPaneId;
    injectPaneIdContext: typeof injectPaneIdContext;
    HEARTBEAT_CHANNEL: string;
    DEAD_LETTER_KEY: string;
};
export default coders;
//# sourceMappingURL=coders.d.ts.map