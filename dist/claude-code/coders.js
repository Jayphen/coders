"use strict";
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
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.coders = void 0;
exports.configure = configure;
exports.spawn = spawn;
exports.list = list;
exports.attach = attach;
exports.kill = kill;
exports.claude = claude;
exports.gemini = gemini;
exports.codex = codex;
exports.opencode = opencode;
exports.worktree = worktree;
exports.getActiveSessions = getActiveSessions;
exports.spawnWithHeartbeat = spawnWithHeartbeat;
exports.sendMessage = sendMessage;
exports.listenForMessages = listenForMessages;
const child_process_1 = require("child_process");
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const redis_1 = require("./redis");
const WORKTREE_BASE = '../worktrees';
const SESSION_PREFIX = 'coder-';
/**
 * Global config for coders skill
 */
let globalConfig = {
    snapshotDir: '~/.coders/snapshots',
    deadLetterTimeout: 120000 // 2 minutes
};
/**
 * Configure the coders skill globally
 */
function configure(config) {
    globalConfig = { ...globalConfig, ...config };
}
/**
 * Get the git root directory
 */
function getGitRoot() {
    try {
        return (0, child_process_1.execSync)('git rev-parse --show-toplevel', { encoding: 'utf8' }).trim();
    }
    catch {
        return process.cwd();
    }
}
/**
 * Get the current git branch
 */
function getCurrentBranch() {
    try {
        return (0, child_process_1.execSync)('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8' }).trim();
    }
    catch {
        return 'main';
    }
}
/**
 * Read file content
 */
function readFile(filePath) {
    try {
        const absPath = path.isAbsolute(filePath) ? filePath : path.join(getGitRoot(), filePath);
        return fs.readFileSync(absPath, 'utf8');
    }
    catch {
        return null;
    }
}
/**
 * Create a git worktree for the given branch
 */
function createWorktree(branchName, baseBranch = 'main') {
    const gitRoot = getGitRoot();
    const worktreePath = path.join(gitRoot, WORKTREE_BASE, branchName);
    try {
        fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
        (0, child_process_1.execSync)(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
        return `✅ Worktree created: ${worktreePath}`;
    }
    catch (e) {
        return `⚠️  Worktree: ${e.message}`;
    }
}
/**
 * Build the spawn command for a tool with optional pane ID injection
 */
function buildCommand(tool, promptFile, worktreePath, paneId, redisConfig) {
    const envVars = [];
    if (worktreePath) {
        envVars.push(`WORKSPACE_DIR="${worktreePath}"`);
    }
    if (paneId) {
        envVars.push(`CODERS_PANE_ID="${paneId}"`);
        envVars.push(`CODERS_SESSION_ID="${SESSION_PREFIX}${tool}-${Date.now()}"`);
    }
    if (redisConfig?.url) {
        envVars.push(`REDIS_URL="${redisConfig.url}"`);
    }
    const env = envVars.length > 0 ? envVars.join(' ') + ' ' : '';
    if (tool === 'claude' || tool === 'claude-code') {
        return `${env}claude --dangerously-spawn-permission -f "${promptFile}"`;
    }
    else if (tool === 'gemini') {
        return `${env}gemini -f "${promptFile}"`;
    }
    else if (tool === 'codex') {
        return `${env}codex -f "${promptFile}"`;
    }
    else if (tool === 'opencode') {
        return `${env}opencode -f "${promptFile}"`;
    }
    throw new Error(`Unknown tool: ${tool}`);
}
/**
 * Generate a prompt file with task, context, and pane ID injection
 */
function createPrompt(task, contextFiles, paneId, redisConfig) {
    let prompt = '';
    // Inject pane ID context if provided
    if (paneId) {
        prompt += (0, redis_1.injectPaneIdContext)('', paneId);
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
<!-- HEARTBEAT_CHANNEL: ${redis_1.HEARTBEAT_CHANNEL} -->
<!-- DEAD_LETTER_KEY: ${redis_1.DEAD_LETTER_KEY} -->

Redis is configured for this session. Publish heartbeats to enable
auto-respawn if you become unresponsive for >2 minutes.

Heartbeat format:
{
  "paneId": "${paneId || '<your-pane-id)'}",
  "status": "alive",
  "timestamp": Date.now()
}

Publish to Redis channel: ${redis_1.HEARTBEAT_CHANNEL}
`;
    }
    return prompt;
}
/**
 * Start the dead-letter listener for a session
 */
function startDeadLetterListener(redisConfig) {
    if (!redisConfig?.url)
        return null;
    const listener = new redis_1.DeadLetterListener(redisConfig);
    listener.start().catch(e => {
        console.error('Failed to start dead-letter listener:', e);
    });
    return listener;
}
/**
 * Spawn a new AI coding assistant in a tmux session
 */
async function spawn(options) {
    const { tool, task = '', name, worktree, baseBranch = 'main', prd, interactive = true, redis: redisConfig, enableHeartbeat = !!redisConfig?.url, enableDeadLetter = !!redisConfig?.url, paneId: providedPaneId } = options;
    if (!task) {
        return '❌ Task description is required. Pass `task: "..."` or use interactive mode.';
    }
    const sessionName = name || `${tool}-${Date.now()}`;
    const sessionId = `${SESSION_PREFIX}${sessionName}`;
    const paneId = providedPaneId || (0, redis_1.getPaneId)();
    // Create worktree if requested
    let worktreePath;
    if (worktree) {
        const gitRoot = getGitRoot();
        worktreePath = path.join(gitRoot, WORKTREE_BASE, worktree);
        try {
            fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
            (0, child_process_1.execSync)(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
        }
        catch (e) {
            // Worktree might exist
        }
    }
    // Build prompt with optional PRD and Redis context
    const contextFiles = prd ? [prd] : [];
    const prompt = createPrompt(task, contextFiles, paneId, redisConfig);
    const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
    fs.writeFileSync(promptFile, prompt);
    // Build command with environment variables
    const cmd = buildCommand(tool, promptFile, worktreePath, paneId, redisConfig);
    // Start dead-letter listener if enabled
    let deadLetterListener = null;
    if (enableDeadLetter && redisConfig) {
        deadLetterListener = startDeadLetterListener(redisConfig);
    }
    try {
        // Clean up existing session if any
        try {
            (0, child_process_1.execSync)(`tmux kill-session -t ${sessionId}`);
        }
        catch { }
        // Create new tmux session
        (0, child_process_1.execSync)(`tmux new-session -s "${sessionId}" -d "${cmd}"`);
        // Store pane info for Redis if configured
        if (redisConfig?.url) {
            const redis = new redis_1.RedisManager(redisConfig);
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
    }
    catch (e) {
        return `❌ Failed: ${e.message}`;
    }
}
/**
 * List all active coder sessions
 */
function list() {
    try {
        const output = (0, child_process_1.execSync)('tmux list-sessions 2>/dev/null', { encoding: 'utf8' });
        const sessions = output.split('\n').filter((s) => s.includes(SESSION_PREFIX));
        if (sessions.length === 0) {
            return 'No coder sessions active.';
        }
        return '📋 Active Coder Sessions:\n\n' + sessions.join('\n');
    }
    catch {
        return 'tmux not available or no sessions';
    }
}
/**
 * Attach to a coder session
 */
function attach(sessionName) {
    const sessionId = `${SESSION_PREFIX}${sessionName}`;
    return `Run: \`tmux attach -t ${sessionId}\``;
}
/**
 * Kill a coder session
 */
function kill(sessionName) {
    const sessionId = `${SESSION_PREFIX}${sessionName}`;
    try {
        (0, child_process_1.execSync)(`tmux kill-session -t ${sessionId}`);
        return `✅ Killed session: ${sessionId}`;
    }
    catch (e) {
        return `❌ Failed: ${e.message}`;
    }
}
/**
 * Quick spawn helpers - minimal options for speed
 */
async function claude(task, options) {
    return spawn({ tool: 'claude', task, ...options });
}
async function gemini(task, options) {
    return spawn({ tool: 'gemini', task, ...options });
}
async function codex(task, options) {
    return spawn({ tool: 'codex', task, ...options });
}
async function opencode(task, options) {
    return spawn({ tool: 'opencode', task, ...options });
}
/**
 * Alias for spawn with worktree - quick syntax
 */
async function worktree(branchName, task, options) {
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
function getActiveSessions() {
    try {
        const output = (0, child_process_1.execSync)('tmux list-sessions 2>/dev/null', { encoding: 'utf8' });
        return output.split('\n')
            .filter((s) => s.includes(SESSION_PREFIX))
            .map((s) => {
            const match = s.match(/coder-([^:]+):/);
            return {
                id: match ? match[1] : 'unknown',
                tool: 'unknown',
                task: '',
                createdAt: new Date()
            };
        });
    }
    catch {
        return [];
    }
}
/**
 * Spawn with Redis heartbeat enabled
 */
async function spawnWithHeartbeat(options) {
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
async function sendMessage(channel, message, redisConfig) {
    const config = redisConfig || globalConfig.redis;
    if (!config?.url) {
        throw new Error('Redis not configured. Pass redis config or set global config.');
    }
    const redis = new redis_1.RedisManager(config);
    await redis.connect();
    await redis.publishMessage(channel, message);
    await redis.disconnect();
}
/**
 * Listen for messages from other agents via Redis pub/sub
 */
async function listenForMessages(channel, callback, redisConfig) {
    const config = redisConfig || globalConfig.redis;
    if (!config?.url) {
        throw new Error('Redis not configured. Pass redis config or set global config.');
    }
    const redis = new redis_1.RedisManager(config);
    await redis.subscribeToChannel(channel, callback);
}
/**
 * Main export with all functions
 */
exports.coders = {
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
    RedisManager: redis_1.RedisManager,
    DeadLetterListener: redis_1.DeadLetterListener,
    getPaneId: redis_1.getPaneId,
    injectPaneIdContext: redis_1.injectPaneIdContext,
    HEARTBEAT_CHANNEL: redis_1.HEARTBEAT_CHANNEL,
    DEAD_LETTER_KEY: redis_1.DEAD_LETTER_KEY
};
exports.default = exports.coders;
//# sourceMappingURL=coders.js.map