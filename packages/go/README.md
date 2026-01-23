# Coders Go CLI

High-performance CLI for managing AI coding sessions in tmux.

## Features

- **Fast startup** - Native Go binary, ~20x faster than Node.js
- **Single binary** - No runtime dependencies
- **TUI** - Interactive terminal UI built with Bubbletea
- **Session management** - Spawn, list, attach, kill sessions
- **Redis integration** - Real-time status via heartbeats and promises

## Installation

### From Source

```bash
cd packages/go
make build
./bin/coders --help
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/Jayphen/coders/releases).

## Usage

### TUI Mode

```bash
coders tui
```

Interactive terminal UI for managing sessions:
- `↑↓` / `jk` - Navigate
- `Enter` / `a` - Attach to session
- `s` - Spawn new session
- `K` - Kill selected session
- `C` - Kill all completed sessions
- `R` - Resume completed session
- `r` - Refresh
- `q` - Quit (switches to orchestrator and kills TUI session if orchestrator exists)

### Spawn Sessions

```bash
coders spawn claude --task "Fix the login bug"
coders spawn claude --task "Add tests" --cwd ~/projects/myapp
coders spawn claude --model sonnet --attach  # Spawn and attach immediately
```

#### Ollama Backend

Run sessions using Ollama instead of Anthropic's API:

```bash
# Set environment variables
export CODERS_OLLAMA_BASE_URL="https://ollama.example.com"
export CODERS_OLLAMA_AUTH_TOKEN="your-token"

# Spawn with --ollama flag
coders spawn claude --ollama --model qwen3-coder:30b --task "Fix lint errors"
```

The `--ollama` flag maps `CODERS_OLLAMA_*` env vars to `ANTHROPIC_*` vars for that session only, so you can run Anthropic and Ollama sessions side by side.

### List Sessions

```bash
coders list                  # Pretty print
coders list --json           # JSON output
coders list --status active  # Filter by status
```

### Version

```bash
coders version
```

## Development

```bash
# Build and run TUI
make run

# Build and run list
make list

# Run tests
make test

# Build for all platforms
make build-all

# Watch and rebuild on changes (requires watchexec)
make watch
```

## Project Structure

```
packages/go/
├── cmd/coders/          # CLI entry points
│   ├── main.go          # Root command
│   ├── tui.go           # TUI subcommand
│   ├── list.go          # List subcommand
│   └── version.go       # Version subcommand
├── internal/
│   ├── tui/             # Bubbletea TUI implementation
│   │   ├── model.go     # Main model and update logic
│   │   ├── views.go     # View rendering
│   │   └── styles.go    # Lipgloss styles
│   ├── tmux/            # Tmux integration
│   ├── redis/           # Redis integration
│   └── types/           # Shared types
├── Makefile
└── go.mod
```

## Architecture

The Go CLI is designed to work alongside the TypeScript Claude plugin:

```
┌─────────────────────┐    ┌─────────────────────┐
│   Go Binary         │◄───│ Claude Code Plugin  │
│   (coders)          │    │ (TypeScript)        │
│                     │    │                     │
│ • Fast CLI commands │    │ • /coders:spawn     │
│ • TUI               │    │ • /coders:promise   │
│ • Background tasks  │    │                     │
└──────────┬──────────┘    └─────────────────────┘
           │
           ▼
    ┌──────┴──────┐
    │   tmux      │    Redis
    │  sessions   │   (state)
    └─────────────┘
```

The plugin calls the Go binary for operations, providing:
- Instant command response (Go startup: ~2ms)
- Single binary distribution
- Shared state via Redis
