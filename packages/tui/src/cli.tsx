#!/usr/bin/env node
import { render } from 'ink';
import { execSync, spawnSync } from 'child_process';
import * as tty from 'tty';
import { fileURLToPath } from 'url';
import path from 'path';
import fs from 'fs';
import { App } from './app.js';

const TUI_SESSION = 'coders-tui';

// Read version from package.json
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const packageJsonPath = path.resolve(__dirname, '../package.json');
let version = 'unknown';
try {
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  version = packageJson.version;
} catch (e) {
  // Ignore
}

function isInsideTmux(): boolean {
  return !!process.env.TMUX;
}

function hasTTY(): boolean {
  return tty.isatty(process.stdin.fd) && tty.isatty(process.stdout.fd);
}

function tuiSessionExists(): boolean {
  try {
    execSync(`tmux has-session -t "${TUI_SESSION}" 2>/dev/null`);
    return true;
  } catch {
    return false;
  }
}

function launchInTmuxSession(): void {
  if (tuiSessionExists()) {
    if (hasTTY()) {
      // Session exists and we have a TTY, attach to it
      spawnSync('tmux', ['attach', '-t', TUI_SESSION], {
        stdio: 'inherit',
      });
    } else {
      // No TTY (running from Claude Code, etc.) - tell user how to attach
      console.log(`\x1b[32m✓ TUI session already running\x1b[0m`);
      console.log(`  Attach with: tmux attach -t ${TUI_SESSION}`);
    }
  } else {
    if (hasTTY()) {
      // Get the path to this script
      const scriptPath = process.argv[1];
      const isTsx = scriptPath.endsWith('.tsx') || scriptPath.endsWith('.ts');
      const runner = isTsx ? 'tsx' : 'node';

      // Create new session running the TUI and attach
      spawnSync('tmux', ['new-session', '-s', TUI_SESSION, '-n', 'tui', runner, scriptPath], {
        stdio: 'inherit',
      });
    } else {
      // No TTY - create detached session with a shell that waits for TTY
      // The TUI will start when the user attaches
      const scriptPath = process.argv[1];
      const isTsx = scriptPath.endsWith('.tsx') || scriptPath.endsWith('.ts');
      const runner = isTsx ? 'tsx' : 'node';

      // Wait for a client to attach to the tmux session before starting the TUI
      const cmd = `while [ $(tmux list-clients -t ${TUI_SESSION} 2>/dev/null | wc -l) -eq 0 ]; do sleep 0.1; done; ${runner} ${scriptPath}`;

      spawnSync('tmux', ['new-session', '-d', '-s', TUI_SESSION, '-n', 'tui', 'sh', '-c', cmd], {
        stdio: 'inherit',
      });
      console.log(`\x1b[32m✓ TUI session started\x1b[0m`);
      console.log(`  Attach with: tmux attach -t ${TUI_SESSION}`);
    }
  }
}

// If we're outside tmux OR don't have a TTY, launch the TUI in its own tmux session
// This allows us to use switch-client when selecting sessions,
// and the user can return with Ctrl-b L (last session)
if (!isInsideTmux() || !hasTTY()) {
  launchInTmuxSession();
} else {
  // We're inside tmux with a proper TTY - render the app
  render(<App version={version} />);
}
