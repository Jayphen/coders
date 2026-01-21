#!/usr/bin/env node

import { execSync, spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { generateSessionName } from './session-name.js';
import {
  loadOrchestratorState,
  markOrchestratorStarted,
  markOrchestratorStopped,
  getOrchestratorSessionId,
  isOrchestratorSession
} from './orchestrator.js';

const TMUX_SESSION_PREFIX = 'coder-';

const colors = {
  green: '\x1b[32m',
  blue: '\x1b[34m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  reset: '\x1b[0m'
};

function log(msg, color = 'reset') {
  console.log(`${colors[color]}${msg}${colors.reset}`);
}

function getGitRoot() {
  try {
    return execSync('git rev-parse --show-toplevel', { encoding: 'utf8' }).trim();
  } catch {
    return process.cwd();
  }
}

/**
 * Check if zoxide is installed on the system
 */
function isZoxideAvailable() {
  try {
    execSync('which zoxide', { encoding: 'utf8', stdio: 'pipe' });
    return true;
  } catch {
    return false;
  }
}

/**
 * Resolve a path using zoxide if available, otherwise return the path as-is
 * @param {string} pathArg - The path argument (could be partial like "vega")
 * @returns {{ resolved: string, method: 'zoxide' | 'direct' } | null} - Resolved path info or null if resolution failed
 */
function resolvePathWithZoxide(pathArg) {
  // First, check if it's already a valid absolute or relative path
  const directPath = path.isAbsolute(pathArg)
    ? pathArg
    : path.resolve(process.cwd(), pathArg);

  if (fs.existsSync(directPath) && fs.statSync(directPath).isDirectory()) {
    return { resolved: directPath, method: 'direct' };
  }

  // Try zoxide if available
  if (isZoxideAvailable()) {
    try {
      const zoxidePath = execSync(`zoxide query "${pathArg}"`, {
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'pipe']
      }).trim();

      if (zoxidePath && fs.existsSync(zoxidePath) && fs.statSync(zoxidePath).isDirectory()) {
        return { resolved: zoxidePath, method: 'zoxide' };
      }
    } catch {
      // zoxide query failed (no match found)
    }
  }

  return null;
}

/**
 * Validate that a path exists and is a directory
 * @param {string} resolvedPath - The path to validate
 * @returns {{ valid: boolean, error?: string }}
 */
function validatePath(resolvedPath) {
  if (!fs.existsSync(resolvedPath)) {
    return { valid: false, error: `Path does not exist: ${resolvedPath}` };
  }

  const stats = fs.statSync(resolvedPath);
  if (!stats.isDirectory()) {
    return { valid: false, error: `Path is not a directory: ${resolvedPath}` };
  }

  return { valid: true };
}

function createWorktree(branchName, baseBranch = 'main') {
  const gitRoot = getGitRoot();
  const worktreePath = path.join(gitRoot, '../worktrees', branchName);
  
  log(`Creating worktree at ${worktreePath}...`, 'blue');
  
  try {
    fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
    execSync(`git worktree add -b ${branchName} ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
    log(`✅ Worktree created: ${worktreePath}`, 'green');
    return worktreePath;
  } catch (e) {
    log(`⚠️  Worktree exists or failed: ${e.message}`, 'yellow');
    return worktreePath;
  }
}

function readFileContent(filePath) {
  try {
    const absPath = path.isAbsolute(filePath) 
      ? filePath 
      : path.join(getGitRoot(), filePath);
    return fs.readFileSync(absPath, 'utf8');
  } catch (e) {
    return null;
  }
}

function generateInitialPrompt(tool, taskDescription, contextFiles = []) {
  let prompt = `TASK: ${taskDescription}\n\n`;
  
  if (contextFiles.length > 0) {
    prompt += 'CONTEXT FILES:\n';
    contextFiles.forEach(file => {
      const content = readFileContent(file);
      if (content) {
        prompt += `\n--- ${file} ---\n${content}\n`;
      }
    });
    prompt += '\n';
  }
  
  prompt += '\nYou have full permissions. Complete the task.';
  return prompt;
}

function buildSpawnCommand(tool, promptFile, prompt, extraEnv = {}) {
  let cmd;
  // Escape single quotes in prompt for shell safety
  const escapedPrompt = prompt.replace(/'/g, "'\\''");

  // Build environment variable string
  const envStr = Object.entries(extraEnv)
    .filter(([_, v]) => v !== undefined && v !== null)
    .map(([k, v]) => `${k}="${v}"`)
    .join(' ');
  const envPrefix = envStr ? envStr + ' ' : '';

  if (tool === 'claude' || tool === 'claude-code') {
    // Claude stays interactive by default - pass prompt via stdin
    // Add --dangerously-skip-permissions to auto-approve file operations
    cmd = `${envPrefix}claude --dangerously-skip-permissions < "${promptFile}"`;
  } else if (tool === 'gemini') {
    // Gemini: use --prompt-interactive with the prompt text to execute and stay interactive
    cmd = `${envPrefix}gemini --yolo --prompt-interactive '${escapedPrompt}'`;
  } else if (tool === 'codex' || tool === 'openai-codex') {
    // Codex: provide initial prompt as positional argument, stays interactive by default
    cmd = `${envPrefix}codex --dangerously-bypass-approvals-and-sandbox '${escapedPrompt}'`;
  } else if (tool === 'opencode' || tool === 'open-code') {
    // OpenCode: pass prompt via stdin like Claude
    cmd = `${envPrefix}opencode < "${promptFile}"`;
  }

  return cmd;
}

function spawnInNewTmuxWindow(tool, worktreePath, prompt, sessionName, enableHeartbeat = false, parentSessionId = null, customCwd = null) {
  const sessionId = `${TMUX_SESSION_PREFIX}${sessionName}`;
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);

  // Auto-detect parent session if running inside a coder session
  const effectiveParentSessionId = parentSessionId || process.env.CODERS_SESSION_ID || null;

  // Determine the effective working directory: customCwd > worktreePath > process.cwd()
  const effectiveCwd = customCwd || worktreePath || process.cwd();

  // Get the user's shell from the environment (e.g., fish, zsh, bash)
  const userShell = process.env.SHELL || '/bin/bash';

  const cmd = buildSpawnCommand(tool, promptFile, prompt, {
    WORKSPACE_DIR: effectiveCwd,
    CODERS_SESSION_ID: sessionId,
    CODERS_PARENT_SESSION_ID: effectiveParentSessionId
  });

  log(`Creating NEW tmux window for: ${sessionId}`, 'blue');

  // Kill existing session if it exists
  try {
    execSync(`tmux kill-session -t ${sessionId} 2>/dev/null`);
  } catch {}

  // Create new session (this opens a WINDOW)
  // Use shell command that keeps session alive after codex exits
  // Use the user's shell explicitly instead of relying on $SHELL expansion
  const fullCmd = `tmux new-session -s "${sessionId}" -d "cd ${effectiveCwd}; ${cmd}; exec ${userShell}"`;

  try {
    execSync(fullCmd);
    log(`✅ Created tmux window: ${sessionId}`, 'green');

    // Start heartbeat in background if enabled
    if (enableHeartbeat) {
      // Check if redis is available before starting heartbeat
      if (!checkRedisDependency()) {
        log(`⚠️  Heartbeat requires redis dependency.`, 'yellow');
        log(`   Run 'coders dashboard' to auto-install dependencies.`, 'yellow');
      } else {
        try {
          const scriptDir = path.dirname(new URL(import.meta.url).pathname);
          const heartbeatScript = path.join(scriptDir, '../../assets/heartbeat.js');
          execSync(`SESSION_ID="${sessionId}" nohup node ${heartbeatScript} "${sessionId}" > /dev/null 2>&1 &`);
          log(`💓 Heartbeat enabled (dashboard will show status)`, 'green');
        } catch (e) {
          log(`⚠️  Heartbeat failed to start: ${e.message}`, 'yellow');
        }
      }
    }

    log(`💡 Attach: coders attach ${sessionName}`, 'yellow');
    log(`💡 Or: tmux attach -t ${sessionId}`, 'yellow');
  } catch (e) {
    log(`❌ Failed: ${e.message}`, 'red');
  }
}

function spawnInITerm(tool, worktreePath, prompt, sessionName) {
  const cmd = buildSpawnCommand(tool, `/tmp/coders-prompt-${Date.now()}.txt`, { WORKSPACE_DIR: worktreePath });
  fs.writeFileSync(`/tmp/coders-prompt-${Date.now()}.txt`, prompt);
  
  log(`Creating NEW iTerm2 window for: ${sessionName}`, 'blue');
  
  // AppleScript to open new iTerm2 window with command
  const applescript = `
    tell application "iTerm2"
      create window with profile "Default"
      tell current session of first window
        write text "${cmd.replace(/"/g, '\\"')}"
      end tell
    end tell
  `;
  
  try {
    execSync(`osascript -e '${applescript}'`);
    log(`✅ Created iTerm2 window!`, 'green');
  } catch (e) {
    log(`❌ iTerm2 not available: ${e.message}`, 'red');
  }
}

function listSessions() {
  try {
    const sessions = execSync('tmux list-sessions 2>/dev/null || echo "No tmux sessions"')
      .toString()
      .split('\n')
      .filter(s => s.includes(TMUX_SESSION_PREFIX));
    
    if (sessions.length === 0) {
      log('No coder sessions found.', 'yellow');
    } else {
      log('\n📋 Active Coder Sessions:\n', 'blue');
      sessions.forEach(s => console.log(s));
    }
  } catch {
    log('tmux not available', 'yellow');
  }
}

function attachSession(sessionName) {
  const fullName = `${TMUX_SESSION_PREFIX}${sessionName}`;
  
  if (process.env.TERM_PROGRAM === 'iTerm.app') {
    // Open iTerm and attach to tmux session
    const applescript = `
      tell application "iTerm2"
        create window with profile "Default"
        tell current session of first window
          write text "tmux attach-session -t ${fullName}"
        end tell
      end tell
    `;
    try {
      execSync(`osascript -e '${applescript}'`);
    } catch {
      execSync(`tmux attach-session -t ${fullName}`);
    }
  } else {
    execSync(`tmux attach-session -t ${fullName}`);
  }
}

function killSession(sessionName) {
  const fullName = `${TMUX_SESSION_PREFIX}${sessionName}`;

  // Kill associated heartbeat process first
  try {
    execSync(`pkill -f "heartbeat.js.*${fullName}"`);
  } catch {} // May not exist

  try {
    execSync(`tmux kill-session -t ${fullName}`);
    log(`✅ Killed session: ${fullName}`, 'green');
  } catch (e) {
    log(`❌ Failed: ${e.message}`, 'red');
  }
}

/**
 * Get list of active coder sessions (tmux sessions with the coder- prefix)
 * @returns {string[]} Array of session names (without the coder- prefix)
 */
function getActiveCoderSessions() {
  try {
    const output = execSync('tmux list-sessions -F "#{session_name}" 2>/dev/null || echo ""', { encoding: 'utf8' });
    return output
      .split('\n')
      .filter(s => s.startsWith(TMUX_SESSION_PREFIX))
      .map(s => s.trim());
  } catch {
    return [];
  }
}

/**
 * Send a plugin update command to all active coder sessions
 * @param {string} pluginName - Name of the plugin to update (default: 'coders')
 */
function updatePluginInSessions(pluginName = 'coders') {
  const sessions = getActiveCoderSessions();

  if (sessions.length === 0) {
    log('No active coder sessions found.', 'yellow');
    return;
  }

  log(`\n📦 Broadcasting plugin update to ${sessions.length} session(s)...\n`, 'blue');

  const updated = [];
  const failed = [];

  for (const sessionId of sessions) {
    try {
      // Send the /plugin update command followed by Enter key
      const command = `/plugin update ${pluginName}`;
      execSync(`tmux send-keys -t "${sessionId}" '${command}' Enter`, { encoding: 'utf8' });
      updated.push(sessionId);
      log(`  ✅ ${sessionId}`, 'green');
    } catch (e) {
      failed.push({ sessionId, error: e.message });
      log(`  ❌ ${sessionId}: ${e.message}`, 'red');
    }
  }

  log('', 'reset');

  if (updated.length > 0) {
    log(`✅ Successfully sent update command to ${updated.length} session(s)`, 'green');
  }

  if (failed.length > 0) {
    log(`⚠️  Failed to update ${failed.length} session(s)`, 'yellow');
  }

  // Show short session names for convenience
  if (updated.length > 0) {
    const shortNames = updated.map(s => s.replace(TMUX_SESSION_PREFIX, ''));
    log(`\n📋 Updated sessions: ${shortNames.join(', ')}`, 'blue');
  }
}

async function startOrAttachOrchestrator() {
  const sessionId = getOrchestratorSessionId();

  // Check if orchestrator session already exists
  try {
    const sessions = execSync('tmux list-sessions 2>/dev/null || echo ""', { encoding: 'utf8' });
    const exists = sessions.includes(sessionId);

    if (exists) {
      log(`Orchestrator session already exists, attaching...`, 'blue');
      try {
        execSync(`tmux attach-session -t ${sessionId}`, { stdio: 'inherit' });
      } catch (e) {
        log(`Failed to attach: ${e.message}`, 'red');
      }
      return;
    }
  } catch (e) {
    // tmux might not be available
  }

  // Create new orchestrator session
  log(`Creating orchestrator session: ${sessionId}`, 'blue');

  const orchestratorPrompt = `
╔════════════════════════════════════════════════════════════════════════════╗
║                      CODER ORCHESTRATOR SESSION                            ║
║                                                                            ║
║  This is a special persistent session for coordinating other coder        ║
║  sessions. You can use the following commands:                            ║
║                                                                            ║
║  - coders spawn <tool> [options]  : Spawn a new coder session             ║
║  - coders list                    : List all active sessions               ║
║  - coders attach <session>        : Attach to a session                    ║
║  - coders kill <session>          : Kill a session                         ║
║  - coders dashboard               : Open the dashboard                     ║
║                                                                            ║
║  Use Claude Code to orchestrate your AI coding sessions!                  ║
╚════════════════════════════════════════════════════════════════════════════╝

Welcome to the Orchestrator session. You have full permissions to spawn and manage
other coder sessions. Start by spawning your first session or listing existing ones.
`;

  const promptFile = `/tmp/coders-orchestrator-prompt.txt`;
  fs.writeFileSync(promptFile, orchestratorPrompt);

  // Spawn orchestrator with Claude Code
  // Get the user's shell from the environment (e.g., fish, zsh, bash)
  const userShell = process.env.SHELL || '/bin/bash';
  const cmd = `claude --dangerously-skip-permissions < "${promptFile}"`;
  const fullCmd = `tmux new-session -s "${sessionId}" -d "cd ${process.cwd()}; ${cmd}; exec ${userShell}"`;

  try {
    execSync(fullCmd);
    await markOrchestratorStarted();
    log(`✅ Created orchestrator session: ${sessionId}`, 'green');
    log(`💡 Attach: coders orchestrator`, 'yellow');
    log(`💡 Or: tmux attach -t ${sessionId}`, 'yellow');

    // Start heartbeat for orchestrator
    if (!checkRedisDependency()) {
      log(`⚠️  Heartbeat requires redis dependency.`, 'yellow');
      log(`   Run 'coders dashboard' to auto-install dependencies.`, 'yellow');
    } else {
      try {
        const scriptDir = path.dirname(new URL(import.meta.url).pathname);
        const heartbeatScript = path.join(scriptDir, '../../assets/heartbeat.js');
        execSync(`SESSION_ID="${sessionId}" nohup node ${heartbeatScript} "${sessionId}" > /dev/null 2>&1 &`);
        log(`💓 Heartbeat enabled for orchestrator`, 'green');
      } catch (e) {
        log(`⚠️  Heartbeat failed to start: ${e.message}`, 'yellow');
      }
    }

    // Auto-attach to the new session only if running in a TTY
    if (process.stdout.isTTY) {
      setTimeout(() => {
        try {
          execSync(`tmux attach-session -t ${sessionId}`, { stdio: 'inherit' });
        } catch (e) {
          log(`Failed to attach: ${e.message}`, 'red');
        }
      }, 500);
    }
  } catch (e) {
    log(`❌ Failed to create orchestrator: ${e.message}`, 'red');
  }
}

function usage() {
  console.log(`
${colors.blue}🤖 Coder Spawner - Spawn AI coding assistants in NEW tmux windows${colors.reset}

${colors.green}Usage:${colors.reset}
  coders spawn <tool> [options]
  coders orchestrator
  coders list
  coders attach <session>
  coders kill <session>
  coders dashboard
  coders restart-dashboard
  coders update-plugin [--plugin <name>]
  coders help

${colors.green}Tools:${colors.reset}
  claude    - Anthropic Claude Code CLI
  gemini    - Google Gemini CLI
  codex     - OpenAI Codex CLI
  opencode  - OpenCode CLI

${colors.green}Orchestrator:${colors.reset}
  coders orchestrator    - Start/attach to the orchestrator session
                          (persistent session for coordinating other coders)

${colors.green}Plugin Update:${colors.reset}
  coders update-plugin   - Broadcast '/plugin update' to all active sessions
    --plugin <name>      Plugin name to update (default: coders)

${colors.green}Options:${colors.reset}
  --name <name>          Session name (auto-generated if omitted)
  --worktree <branch>    Create git worktree for this branch
  --base <branch>        Base branch for worktree (default: main)
  --prd <file>           Read PRD/spec file and prime the AI
  --spec <file>          Alias for --prd
  --task <description>   Task description
  --cwd <path>           Working directory for the session (default: git root)
  --dir <path>           Alias for --cwd
  --no-heartbeat         Disable heartbeat tracking (enabled by default)

${colors.green}Examples:${colors.reset}
  coders orchestrator
  coders spawn claude --worktree feature/auth --prd docs/prd.md
  coders spawn gemini --name my-session --task "Fix the login bug"
  coders spawn claude --cwd ~/projects/myapp --task "Refactor the API"
  coders list
  coders attach feature-auth
  coders kill feature-auth
  coders dashboard
  coders restart-dashboard
  coders update-plugin
  coders update-plugin --plugin beads

${colors.green}How it works:${colors.reset}
  1. Creates NEW tmux window (visible!)
  2. Runs Claude/Gemini/Codex in it
  3. Attach with: coders attach <name>
`);
}

async function isDashboardRunning(port) {
  try {
    const response = await fetch(`http://localhost:${port}/api/sessions`);
    return response.ok;
  } catch {
    return false;
  }
}

async function waitForDashboard(port, timeoutMs = 5000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (await isDashboardRunning(port)) return true;
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  return false;
}

function startDashboardServer(port) {
  const scriptDir = path.dirname(new URL(import.meta.url).pathname);
  const dashboardScript = path.join(scriptDir, '../../assets/dashboard-server.js');
  const logPath = path.join(os.tmpdir(), 'coders-dashboard.log');
  const logHandle = fs.openSync(logPath, 'a');

  const child = spawn('node', [dashboardScript], {
    detached: true,
    stdio: ['ignore', logHandle, logHandle],
    env: { ...process.env, DASHBOARD_PORT: String(port) }
  });
  child.unref();
  return logPath;
}

function openDashboard(url) {
  try {
    if (process.platform === 'darwin') {
      execSync(`open "${url}"`);
    } else if (process.platform === 'win32') {
      execSync(`start "" "${url}"`, { shell: 'cmd.exe' });
    } else {
      execSync(`xdg-open "${url}"`);
    }
  } catch (e) {
    log(`⚠️  Failed to open browser: ${e.message}`, 'yellow');
    log(`Open manually: ${url}`, 'yellow');
  }
}

function checkRedisDependency() {
  try {
    // Get plugin root directory and check if redis exists in node_modules
    const scriptDir = path.dirname(new URL(import.meta.url).pathname);
    const pluginRoot = path.resolve(scriptDir, '../../..');
    const redisPath = path.join(pluginRoot, 'node_modules', 'redis');

    return fs.existsSync(redisPath);
  } catch {
    return false;
  }
}

async function ensureDependencies() {
  // Check if redis is installed
  if (checkRedisDependency()) {
    return true;
  }

  log(`📦 Redis dependency not found. Installing dependencies...`, 'yellow');

  // Get plugin root directory (go up from scripts/ -> coders/ -> skills/ -> root)
  const scriptDir = path.dirname(new URL(import.meta.url).pathname);
  const pluginRoot = path.resolve(scriptDir, '../../..');

  try {
    log(`   Running npm install in ${pluginRoot}...`, 'blue');
    execSync('npm install', {
      cwd: pluginRoot,
      stdio: 'inherit'
    });

    log(`✅ Dependencies installed successfully!`, 'green');
    return true;
  } catch (e) {
    log(`❌ Failed to install dependencies: ${e.message}`, 'red');
    log(`   Please run 'npm install' manually in: ${pluginRoot}`, 'yellow');
    return false;
  }
}

async function launchDashboard() {
  // Ensure dependencies are installed before starting dashboard
  const depsOk = await ensureDependencies();
  if (!depsOk) {
    log(`❌ Cannot start dashboard without dependencies.`, 'red');
    return;
  }

  const port = process.env.DASHBOARD_PORT || '3030';
  const url = `http://localhost:${port}`;

  if (!(await isDashboardRunning(port))) {
    const logPath = startDashboardServer(port);
    const started = await waitForDashboard(port);
    if (started) {
      log(`✅ Dashboard server started on ${url}`, 'green');
      log(`📝 Logs: ${logPath}`, 'yellow');
    } else {
      log(`⚠️  Dashboard server may not have started yet.`, 'yellow');
      log(`📝 Logs: ${logPath}`, 'yellow');
    }
  } else {
    log(`✅ Dashboard already running on ${url}`, 'green');
  }

  openDashboard(url);
}

function killDashboardServer() {
  try {
    // Find and kill the dashboard server process
    const scriptDir = path.dirname(new URL(import.meta.url).pathname);
    const dashboardScript = path.join(scriptDir, '../../assets/dashboard-server.js');

    // Kill by script name pattern
    execSync(`pkill -f "node.*dashboard-server.js" 2>/dev/null || true`, { encoding: 'utf8' });
    log(`✅ Stopped existing dashboard server`, 'green');
    return true;
  } catch (e) {
    // pkill returns non-zero if no processes matched, which is fine
    return true;
  }
}

async function restartDashboard() {
  log(`🔄 Restarting dashboard server...`, 'blue');

  // Kill existing dashboard
  killDashboardServer();

  // Wait a moment for the port to be released
  await new Promise(resolve => setTimeout(resolve, 500));

  // Ensure dependencies are installed before starting dashboard
  const depsOk = await ensureDependencies();
  if (!depsOk) {
    log(`❌ Cannot start dashboard without dependencies.`, 'red');
    return;
  }

  const port = process.env.DASHBOARD_PORT || '3030';
  const url = `http://localhost:${port}`;

  // Start fresh dashboard
  const logPath = startDashboardServer(port);
  const started = await waitForDashboard(port);

  if (started) {
    log(`✅ Dashboard server restarted on ${url}`, 'green');
    log(`📝 Logs: ${logPath}`, 'yellow');
    openDashboard(url);
  } else {
    log(`⚠️  Dashboard server may not have started yet.`, 'yellow');
    log(`📝 Check logs: ${logPath}`, 'yellow');
  }
}

// Main
const args = process.argv.slice(2);
const command = args[0];

if (command === 'help' || !command) {
  usage();
} else if (command === 'orchestrator') {
  startOrAttachOrchestrator().catch((err) => {
    log(`❌ Failed to start orchestrator: ${err.message}`, 'red');
  });
} else if (command === 'list') {
  listSessions();
} else if (command === 'attach') {
  const sessionName = args[1];
  if (!sessionName) {
    log('Usage: coders attach <session-name>', 'red');
  } else {
    attachSession(sessionName);
  }
} else if (command === 'kill') {
  const sessionName = args[1];
  if (!sessionName) {
    log('Usage: coders kill <session-name>', 'red');
  } else {
    killSession(sessionName);
  }
} else if (command === 'dashboard') {
  launchDashboard().catch((err) => {
    log(`❌ Failed to launch dashboard: ${err.message}`, 'red');
  });
} else if (command === 'restart-dashboard') {
  restartDashboard().catch((err) => {
    log(`❌ Failed to restart dashboard: ${err.message}`, 'red');
  });
} else if (command === 'update-plugin') {
  // Parse --plugin flag, default to 'coders'
  let pluginName = 'coders';
  for (let i = 1; i < args.length; i++) {
    if (args[i] === '--plugin' && args[i + 1]) {
      pluginName = args[i + 1];
      i++;
    }
  }
  updatePluginInSessions(pluginName);
} else if (command === 'spawn') {
  // Default to 'claude' if no tool specified or if tool looks like a flag
  let tool = args[1];
  let argStartIndex = 2; // Default: args start after tool name

  if (!tool || tool.startsWith('--')) {
    tool = 'claude';
    argStartIndex = 1; // No tool specified, args start at index 1
  }

  let sessionName = null; // Will be generated from task if not provided
  let worktreeBranch = null;
  let baseBranch = 'main';
  let prdFile = null;
  let taskDesc = 'Complete the assigned task';
  let enableHeartbeat = true; // Enabled by default for dashboard tracking
  let customCwd = null; // Optional working directory

  for (let i = argStartIndex; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--name' && args[i+1]) {
      sessionName = args[i+1];
      i++;
    } else if (arg === '--worktree' && args[i+1]) {
      worktreeBranch = args[i+1];
      i++;
    } else if (arg === '--base' && args[i+1]) {
      baseBranch = args[i+1];
      i++;
    } else if ((arg === '--prd' || arg === '--spec') && args[i+1]) {
      prdFile = args[i+1];
      i++;
    } else if (arg === '--task' && args[i+1]) {
      taskDesc = args[i+1];
      i++;
    } else if ((arg === '--cwd' || arg === '--dir') && args[i+1]) {
      // Store raw path arg - will be resolved later with zoxide support
      customCwd = args[i+1];
      i++;
    } else if (arg === '--heartbeat' || arg === '--dashboard') {
      enableHeartbeat = true;
    } else if (arg === '--no-heartbeat') {
      enableHeartbeat = false;
    }
  }

  // Generate session name from task description if not explicitly provided
  if (!sessionName) {
    sessionName = generateSessionName(tool, taskDesc);
  }

  // Resolve and validate custom working directory if provided
  let resolvedCwd = null;
  if (customCwd) {
    const pathResult = resolvePathWithZoxide(customCwd);

    if (!pathResult) {
      // Path resolution failed
      const hasZoxide = isZoxideAvailable();
      log(`❌ Invalid path: "${customCwd}"`, 'red');
      log(`   Path does not exist and could not be resolved.`, 'red');
      if (hasZoxide) {
        log(`   Note: zoxide was checked but found no match.`, 'yellow');
      } else {
        log(`   Tip: Install zoxide (https://github.com/ajeetdsouza/zoxide) for smart directory jumping.`, 'yellow');
      }
      process.exit(1);
    }

    // Validate the resolved path
    const validation = validatePath(pathResult.resolved);
    if (!validation.valid) {
      log(`❌ ${validation.error}`, 'red');
      process.exit(1);
    }

    resolvedCwd = pathResult.resolved;

    // Log resolution method
    if (pathResult.method === 'zoxide') {
      log(`🔍 Resolved "${customCwd}" → ${resolvedCwd} (via zoxide)`, 'blue');
    } else if (customCwd !== resolvedCwd) {
      log(`📁 Working directory: ${resolvedCwd}`, 'blue');
    }
  }

  // Create worktree if requested
  let worktreePath = null;
  if (worktreeBranch) {
    worktreePath = createWorktree(worktreeBranch, baseBranch);
  }

  // Build context from PRD
  const contextFiles = prdFile ? [prdFile] : [];
  const prompt = generateInitialPrompt(tool, taskDesc, contextFiles);

  // Spawn in new tmux window
  // Always use tmux for reliability
  spawnInNewTmuxWindow(tool, worktreePath, prompt, sessionName, enableHeartbeat, null, resolvedCwd);

  log(`\n✅ Created new window for session "${sessionName}"!`, 'green');
  // Show parent info if spawned from another session
  const effectiveParentSessionId = process.env.CODERS_SESSION_ID || null;
  if (effectiveParentSessionId) {
    log(`👪 Parent session: ${effectiveParentSessionId}`, 'blue');
  }
  log(`💡 Attach: coders attach ${sessionName}`, 'yellow');
  if (enableHeartbeat) {
    log(`💡 View dashboard: coders dashboard`, 'yellow');
  }
} else {
  log(`Unknown command: ${command}`, 'red');
  usage();
}
