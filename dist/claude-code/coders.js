"use strict";
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
exports.spawn = spawn;
exports.list = list;
exports.attach = attach;
exports.kill = kill;
exports.claude = claude;
exports.gemini = gemini;
exports.codex = codex;
exports.worktree = worktree;
exports.getActiveSessions = getActiveSessions;
const child_process_1 = require("child_process");
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const WORKTREE_BASE = '../worktrees';
const SESSION_PREFIX = 'coder-';
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
 * Build the spawn command for a tool
 */
function buildCommand(tool, promptFile, worktreePath) {
    const env = worktreePath ? `WORKSPACE_DIR="${worktreePath}" ` : '';
    if (tool === 'claude' || tool === 'claude-code') {
        return `${env}claude --dangerously-spawn-permission -f "${promptFile}"`;
    }
    else if (tool === 'gemini') {
        return `${env}gemini -f "${promptFile}"`;
    }
    else if (tool === 'codex') {
        return `${env}codex -f "${promptFile}"`;
    }
    throw new Error(`Unknown tool: ${tool}`);
}
/**
 * Generate a prompt file with task and context
 */
function createPrompt(task, contextFiles) {
    let prompt = `TASK: ${task}\n\n`;
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
    return prompt;
}
/**
 * Spawn a new AI coding assistant in a tmux session
 */
async function spawn(options) {
    const { tool, task = '', name, worktree, baseBranch = 'main', prd, interactive = true } = options;
    if (!task) {
        return '❌ Task description is required. Pass `task: "..."` or use interactive mode.';
    }
    const sessionName = name || `${tool}-${Date.now()}`;
    const sessionId = `${SESSION_PREFIX}${sessionName}`;
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
    // Build prompt with optional PRD
    const contextFiles = prd ? [prd] : [];
    const prompt = createPrompt(task, contextFiles);
    const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
    fs.writeFileSync(promptFile, prompt);
    // Build and run command
    const cmd = buildCommand(tool, promptFile, worktreePath);
    try {
        try {
            (0, child_process_1.execSync)(`tmux kill-session -t ${sessionId}`);
        }
        catch { }
        (0, child_process_1.execSync)(`tmux new-session -s "${sessionId}" -d "${cmd}"`);
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
/**
 * Alias for spawn with worktree - quick syntax
 */
async function worktree(branchName, task, options) {
    return spawn({
        tool: options?.tool || 'claude',
        task,
        worktree: branchName,
        prd: options?.prd
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
 * Main export with all functions
 */
exports.coders = {
    spawn,
    list,
    attach,
    kill,
    claude,
    gemini,
    codex,
    worktree,
    createWorktree,
    getActiveSessions
};
exports.default = exports.coders;
//# sourceMappingURL=coders.js.map