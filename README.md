# Coders

Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with optional git worktrees.

## Packages

This is a monorepo containing:

| Package | Description | |
|---------|-------------|---|
| [`@jayphen/coders`](./packages/plugin) | Claude Code plugin for spawning AI sessions | [Install via Claude](https://github.com/Jayphen/coders#claude-code-plugin-recommended) |
| [`coders-tui`](./packages/go) | Go-based CLI and TUI for managing sessions | Built from source |

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

### Go CLI & TUI (Optional)

The `coders` binary provides a terminal UI and CLI tools for managing sessions.

#### Install via Homebrew (macOS)

```bash
brew tap Jayphen/coders
brew install coders
```

#### Install via Script (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Jayphen/coders/go-rewrite/packages/go/install.sh | bash
```

This script:
- Detects your OS (macOS/Linux) and architecture (amd64/arm64)
- Downloads the latest release from GitHub
- Installs to `/usr/local/bin/coders` (customize with `INSTALL_DIR`)

#### Download from GitHub Releases

1. Visit [GitHub Releases](https://github.com/Jayphen/coders/releases/latest)
2. Download the binary for your platform:
   - `coders-darwin-amd64` (macOS Intel)
   - `coders-darwin-arm64` (macOS Apple Silicon)
   - `coders-linux-amd64` (Linux x86_64)
   - `coders-linux-arm64` (Linux ARM64)
3. Make it executable and move to your PATH:
   ```bash
   chmod +x coders-*
   sudo mv coders-* /usr/local/bin/coders
   ```

#### Build from Source

Requires Go 1.21+

```bash
cd packages/go
make build    # Creates ./coders-tui
make install  # Installs to /usr/local/bin
```

#### Usage

```bash
coders --help         # View available commands
coders tui            # Launch the TUI
coders spawn <tool>   # Spawn a new session
coders list           # List all sessions
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
│   └── go/                     # Go implementation (TUI, CLI, orchestrator)
│       ├── cmd/coders/         # Main entry point
│       ├── internal/           # Internal packages
│       │   ├── config/         # Configuration management
│       │   ├── logging/        # Structured logging
│       │   ├── tmux/           # Tmux integration
│       │   └── tui/            # Terminal UI (Bubble Tea)
│       ├── Makefile
│       └── go.mod
│
├── package.json                # Workspace root (plugin dependencies)
└── pnpm-workspace.yaml
```

## Development

```bash
# Install plugin dependencies
pnpm install

# Test the plugin locally
pnpm plugin:test

# Build and test the Go TUI/CLI
cd packages/go
make build
./coders-tui --help

# Run the TUI
./coders-tui tui

# Run tests
make test
```

## Documentation

- [Plugin README](./packages/plugin/README.md) - Full plugin documentation
- [Go TUI README](./packages/go/README.md) - Go implementation documentation
- [CLAUDE.md](./CLAUDE.md) - Project guide and deployment instructions

## License

MIT
