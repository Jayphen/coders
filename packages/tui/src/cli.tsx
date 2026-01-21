#!/usr/bin/env node
import { render } from 'ink';
import { execSync, spawnSync } from 'child_process';
import * as tty from 'tty';
import { App } from './app.js';

const TUI_SESSION = 'coders-tui';

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
  // Get the path to this script
  const scriptPath = process.argv[1];

  // Determine how to run the script (tsx for .tsx files, node for .js)
  const isTsx = scriptPath.endsWith('.tsx') || scriptPath.endsWith('.ts');
  const runner = isTsx ? 'tsx' : 'node';

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
      // Create new session running the TUI and attach
      spawnSync('tmux', ['new-session', '-s', TUI_SESSION, '-n', 'tui', runner, scriptPath], {
        stdio: 'inherit',
      });
    } else {
      // No TTY - create detached session
      spawnSync('tmux', ['new-session', '-d', '-s', TUI_SESSION, '-n', 'tui', runner, scriptPath], {
        stdio: 'inherit',
      });
      console.log(`\x1b[32m✓ TUI session started\x1b[0m`);
      console.log(`  Attach with: tmux attach -t ${TUI_SESSION}`);
    }
  }
}

// If we're outside tmux, launch the TUI inside its own tmux session
// This allows us to use switch-client when selecting sessions,
// and the user can return with Ctrl-b L (last session)
if (!isInsideTmux()) {
  launchInTmuxSession();
} else {
  // We're inside tmux (either the TUI session or another) - render the app
  render(<App />);
}
