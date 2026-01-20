#!/usr/bin/env node

import { execSync, spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';

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

function createWorktree(branchName, baseBranch = 'main') {
  const gitRoot = getGitRoot();
  const worktreePath = path.join(gitRoot, '../worktrees', branchName);
  
  log(`Creating worktree at ${worktreePath}...`, 'blue');
  
  try {
    fs.mkdirSync(path.dirname(worktreePath), { recursive: true });
    execSync(`git worktree add ${worktreePath} ${baseBranch}`, { cwd: gitRoot });
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

  if (tool === 'claude' || tool === 'claude-code') {
    // Claude stays interactive by default - pass prompt via stdin
    // Add --dangerously-skip-permissions to auto-approve file operations
    cmd = `CLAUDE=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} claude --dangerously-skip-permissions < "${promptFile}"`;
  } else if (tool === 'gemini') {
    // Gemini: use --prompt-interactive with the prompt text to execute and stay interactive
    cmd = `GEMINI=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} gemini --yolo --prompt-interactive '${escapedPrompt}'`;
  } else if (tool === 'codex' || tool === 'openai-codex') {
    // Codex: provide initial prompt as positional argument, stays interactive by default
    cmd = `CODEX=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} codex --dangerously-bypass-approvals-and-sandbox '${escapedPrompt}'`;
  }

  return cmd;
}

function spawnInNewTmuxWindow(tool, worktreePath, prompt, sessionName, enableHeartbeat = false) {
  const sessionId = `${TMUX_SESSION_PREFIX}${sessionName}`;
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);

  const cmd = buildSpawnCommand(tool, promptFile, prompt, { WORKSPACE_DIR: worktreePath });

  log(`Creating NEW tmux window for: ${sessionId}`, 'blue');

  // Kill existing session if it exists
  try {
    execSync(`tmux kill-session -t ${sessionId} 2>/dev/null`);
  } catch {}

  // Create new session (this opens a WINDOW)
  // Use shell command that keeps session alive after codex exits
  const fullCmd = `tmux new-session -s "${sessionId}" -d "cd ${worktreePath}; ${cmd}; exec $SHELL"`;

  try {
    execSync(fullCmd);
    log(`✅ Created tmux window: ${sessionId}`, 'green');

    // Start heartbeat in background if enabled
    if (enableHeartbeat) {
      try {
        const scriptDir = path.dirname(new URL(import.meta.url).pathname);
        const heartbeatScript = path.join(scriptDir, '../scripts/heartbeat.js');
        execSync(`SESSION_ID="${sessionId}" nohup node ${heartbeatScript} "${sessionId}" > /dev/null 2>&1 &`);
        log(`💓 Heartbeat enabled (dashboard will show status)`, 'green');
      } catch (e) {
        log(`⚠️  Heartbeat failed to start: ${e.message}`, 'yellow');
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
  try {
    execSync(`tmux kill-session -t ${fullName}`);
    log(`✅ Killed session: ${fullName}`, 'green');
  } catch (e) {
    log(`❌ Failed: ${e.message}`, 'red');
  }
}

function generateSessionName(tool, taskDesc) {
  // Extract first meaningful phrase from task description
  const match = taskDesc.match(/(?:Review|Build|Fix|Create|Update|Implement|Analyze|Test|Debug)\s+(?:the\s+)?([^.,:;]+)/i);

  if (match && match[1]) {
    // Create slug from the extracted phrase
    const slug = match[1]
      .toLowerCase()
      .replace(/\s+/g, '-')
      .replace(/[^a-z0-9-]/g, '')
      .substring(0, 40); // Limit length

    return `${tool}-${slug}`;
  }

  // Fallback to timestamp if no match
  return `${tool}-${Date.now()}`;
}

function usage() {
  console.log(`
${colors.blue}🤖 Coder Spawner - Spawn AI coding assistants in NEW tmux windows${colors.reset}

${colors.green}Usage:${colors.reset}
  coders spawn <tool> [options]
  coders list
  coders attach <session>
  coders kill <session>
  coders dashboard
  coders help

${colors.green}Tools:${colors.reset}
  claude    - Anthropic Claude Code CLI
  gemini    - Google Gemini CLI  
  codex     - OpenAI Codex CLI

${colors.green}Options:${colors.reset}
  --name <name>          Session name (auto-generated if omitted)
  --worktree <branch>    Create git worktree for this branch
  --base <branch>        Base branch for worktree (default: main)
  --prd <file>           Read PRD/spec file and prime the AI
  --spec <file>          Alias for --prd
  --task <description>   Task description
  --no-heartbeat         Disable heartbeat tracking (enabled by default)

${colors.green}Examples:${colors.reset}
  coders spawn claude --worktree feature/auth --prd docs/prd.md
  coders spawn gemini --name my-session --task "Fix the login bug"
  coders list
  coders attach feature-auth
  coders kill feature-auth
  coders dashboard

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
  const dashboardScript = path.join(scriptDir, '../scripts/dashboard-server.js');
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

async function launchDashboard() {
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

// Main
const args = process.argv.slice(2);
const command = args[0];

if (command === 'help' || !command) {
  usage();
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
} else if (command === 'spawn') {
  const tool = args[1];

  if (!tool) {
    log('Usage: coders spawn <claude|gemini|codex> [options]', 'red');
    process.exit(1);
  }

  let sessionName = null; // Will be generated from task if not provided
  let worktreeBranch = null;
  let baseBranch = 'main';
  let prdFile = null;
  let taskDesc = 'Complete the assigned task';
  let enableHeartbeat = true; // Enabled by default for dashboard tracking

  for (let i = 2; i < args.length; i++) {
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
  spawnInNewTmuxWindow(tool, worktreePath || process.cwd(), prompt, sessionName, enableHeartbeat);

  log(`\n✅ Created new window for session "${sessionName}"!`, 'green');
  log(`💡 Attach: coders attach ${sessionName}`, 'yellow');
  if (enableHeartbeat) {
    log(`💡 View dashboard: coders dashboard`, 'yellow');
  }
} else {
  log(`Unknown command: ${command}`, 'red');
  usage();
}
