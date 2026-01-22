# Coders

Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with optional git worktrees.

## Packages

This is a monorepo containing:

| Package | Description | |
|---------|-------------|---|
| [`@jayphen/coders`](./packages/plugin) | Claude Code plugin for spawning AI sessions | [Install via Claude](https://github.com/Jayphen/coders#claude-code-plugin-recommended) |
| [`@jayphen/coders-tui`](./packages/tui) | Terminal UI for managing sessions | [![npm](https://img.shields.io/npm/v/@jayphen/coders-tui)](https://www.npmjs.com/package/@jayphen/coders-tui) |

## Quick Start

### Claude Code Plugin (Recommended)

```bash
# Install from marketplace
claude plugin marketplace add https://github.com/Jayphen/coders.git
claude plugin install coders@coders
```

**Available commands:**
```bash
/coders:spawn claude --task "Build auth" --worktree feature/auth
/coders:list
/coders:attach my-session
/coders:kill my-session
/coders:prune --force
/coders:dashboard
/coders:snapshot
/coders:restore
```

### Terminal UI (Optional)

```bash
# Install the TUI globally
npm install -g @jayphen/coders-tui

# Run it
coders-tui
```

```
┌─ Coders Session Manager ─────────────────────────────┐
│                                                       │
│   SESSION                    TOOL      TASK    STATUS │
│ ❯ 🎯 orchestrator            claude    -         ●    │
│   ├─ claude-fix-auth         claude    fix-auth  ●    │
│   └─ gemini-write-tests      gemini    tests     ●    │
│                                                       │
│ 3 sessions    ↑↓/jk navigate  a/↵ attach  K kill  q  │
└───────────────────────────────────────────────────────┘
```

## Prerequisites

- **tmux** - Required for session management
- **Redis** - Required for coordination and heartbeat monitoring

## Features

- **Multi-Tool Support**: Claude, Gemini, Codex, OpenCode
- **Interactive Sessions**: All spawned AIs stay in interactive mode
- **Git Worktrees**: Creates isolated branches for each task
- **PRD Priming**: Feeds context to the AI before it starts
- **Redis Heartbeat**: Session monitoring and inter-agent pub/sub
- **Web Dashboard**: Real-time monitoring at localhost:3030
- **Tmux Resurrect**: Snapshot/restore entire swarm

<img width="1505" height="1331" alt="Dashboard" src="https://github.com/user-attachments/assets/a9f46996-670c-4e13-975c-d8e381aaa0ab" />

## Project Structure

```
coders/
├── packages/
│   ├── plugin/                 # Claude Code plugin (distributed)
│   │   ├── .claude-plugin/
│   │   ├── commands/           # Slash commands
│   │   ├── skills/             # Core functionality
│   │   ├── bin/                # CLI wrapper
│   │   └── package.json
│   │
│   └── tui/                    # Terminal UI (distributed separately)
│       ├── src/
│       │   ├── components/     # Ink React components
│       │   └── app.tsx
│       └── package.json
│
├── dev/                        # Development only (not distributed)
│   ├── test/                   # Test files
│   ├── notes/                  # Dev documentation
│   └── hooks/                  # Git hooks
│
├── package.json                # Workspace root
└── pnpm-workspace.yaml
```

## Development

```bash
# Install dependencies
pnpm install

# Test the plugin locally
pnpm plugin:test

# Run TUI in dev mode
pnpm dev:tui

# Run tests
pnpm test
```

## Documentation

- [Plugin README](./packages/plugin/README.md) - Full plugin documentation
- [TUI README](./packages/tui/README.md) - Terminal UI documentation

## License

MIT
