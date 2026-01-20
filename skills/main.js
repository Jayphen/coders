#!/usr/bin/env node

import { execSync, spawn } from 'child_process';
import fs from 'fs';
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

function buildSpawnCommand(tool, promptFile, extraEnv = {}) {
  let cmd;

  if (tool === 'claude' || tool === 'claude-code') {
    cmd = `CLAUDE=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} claude --dangerously-spawn-permission -f "${promptFile}"`;
  } else if (tool === 'gemini') {
    // Gemini uses positional arguments for prompt, with --yolo for auto-approval
    cmd = `GEMINI=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} gemini --yolo '$(cat ${promptFile})'`;
  } else if (tool === 'codex' || tool === 'openai-codex') {
    // Codex doesn't use -f, it takes prompt as an argument
    // Use single quotes around the command substitution to avoid quote conflicts
    cmd = `CODEX=${Object.entries(extraEnv).map(([k,v]) => `${k}="${v}"`).join(' ')} codex --dangerously-bypass-approvals-and-sandbox '$(cat ${promptFile})'`;
  }

  return cmd;
}

function spawnInNewTmuxWindow(tool, worktreePath, prompt, sessionName) {
  const sessionId = `${TMUX_SESSION_PREFIX}${sessionName}`;
  const promptFile = `/tmp/coders-prompt-${Date.now()}.txt`;
  fs.writeFileSync(promptFile, prompt);
  
  const cmd = buildSpawnCommand(tool, promptFile, { WORKSPACE_DIR: worktreePath });
  
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

function usage() {
  console.log(`
${colors.blue}🤖 Coder Spawner - Spawn AI coding assistants in NEW tmux windows${colors.reset}

${colors.green}Usage:${colors.reset}
  coders spawn <tool> [options]
  coders list
  coders attach <session>
  coders kill <session>
  coders help

${colors.green}Tools:${colors.reset}
  claude    - Anthropic Claude Code CLI (with --dangerously-spawn-permission)
  gemini    - Google Gemini CLI  
  codex     - OpenAI Codex CLI

${colors.green}Options:${colors.reset}
  --name <name>          Session name (auto-generated if omitted)
  --worktree <branch>    Create git worktree for this branch
  --base <branch>        Base branch for worktree (default: main)
  --prd <file>           Read PRD/spec file and prime the AI
  --spec <file>          Alias for --prd
  --task <description>   Task description

${colors.green}Examples:${colors.reset}
  coders spawn claude --worktree feature/auth --prd docs/prd.md
  coders spawn gemini --name my-session --task "Fix the login bug"
  coders list
  coders attach feature-auth
  coders kill feature-auth

${colors.green}How it works:${colors.reset}
  1. Creates NEW tmux window (visible!)
  2. Runs Claude/Gemini/Codex in it
  3. Claude has --dangerously-spawn-permission
  4. Attach with: coders attach <name>
`);
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
} else if (command === 'spawn') {
  const tool = args[1];
  
  if (!tool) {
    log('Usage: coders spawn <claude|gemini|codex> [options]', 'red');
    process.exit(1);
  }
  
  let sessionName = `${tool}-${Date.now()}`;
  let worktreeBranch = null;
  let baseBranch = 'main';
  let prdFile = null;
  let taskDesc = 'Complete the assigned task';
  
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
  spawnInNewTmuxWindow(tool, worktreePath || process.cwd(), prompt, sessionName);
  
  log(`\n✅ Created new window for session "${sessionName}"!`, 'green');
  log(`💡 Attach: coders attach ${sessionName}`, 'yellow');
} else {
  log(`Unknown command: ${command}`, 'red');
  usage();
}
