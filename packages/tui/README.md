# @jayphen/coders-tui

Terminal UI for managing Coders sessions.

## Installation

```bash
npm install -g @jayphen/coders-tui
```

## Usage

```bash
coders-tui
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` / `a` | Attach to selected session |
| `s` | Spawn a new session |
| `K` (shift+k) | Kill selected session |
| `r` | Refresh session list |
| `q` | Quit |

## Features

- Real-time session list with auto-refresh (5s)
- Visual indicators for session health (heartbeat status)
- Parent-child session hierarchy display
- Tool-specific color coding (Claude, Gemini, Codex, OpenCode)
- Orchestrator sessions highlighted with special styling

## Development

```bash
# From the monorepo root
pnpm dev:tui

# Or from this directory
pnpm dev
```

## License

MIT
