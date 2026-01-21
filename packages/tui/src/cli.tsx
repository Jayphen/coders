#!/usr/bin/env node
import { render } from 'ink';
import { execSync, spawnSync } from 'child_process';
import { App } from './app.js';

const TUI_SESSION = 'coders-tui';

function isInsideTmux(): boolean {
  return !!process.env.TMUX;
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
    // Session exists, attach to it
    spawnSync('tmux', ['attach', '-t', TUI_SESSION], {
      stdio: 'inherit',
    });
  } else {
    // Create new session running the TUI
    spawnSync('tmux', ['new-session', '-s', TUI_SESSION, '-n', 'tui', runner, scriptPath], {
      stdio: 'inherit',
    });
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
