---
description: Open the terminal UI for managing coder sessions
---

# Open the Coders TUI

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js tui
```
CLI alternative:
```bash
./bin/coders tui
```

Opens the terminal UI for managing coder sessions. The TUI runs in its own tmux session (`coders-tui`) so you can easily switch between it and your coding sessions.

## Usage

```
/coders:tui
```

## Features

- Real-time session list with auto-refresh
- Keyboard navigation for quick session management
- Visual session status indicators
- Parent-child session hierarchy display

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` / `a` | Attach to selected session |
| `K` | Kill selected session |
| `r` | Refresh session list |
| `q` | Quit TUI |

## Notes

- The TUI spawns in a dedicated tmux session named `coders-tui`
- Use `Ctrl-b L` (tmux last-session) to quickly switch back after attaching
- If already running, the command will attach to the existing TUI session
