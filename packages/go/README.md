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

## Loop Runner

The loop runner automatically processes tasks from a todolist file, spawning a fresh coder session for each task.

### Basic Usage

```bash
# Create a todolist
cat > tasks.txt << 'EOF'
[ ] Add input validation to the API endpoints
[ ] Write unit tests for the auth module
[ ] Update README with API documentation
EOF

# Run the loop
coders loop --todolist tasks.txt --cwd ~/project
```

The loop:
1. Reads uncompleted tasks (`[ ]` format)
2. Spawns a coder session for each task
3. Waits for the session to publish a completion promise
4. Marks the task complete (`[x]`) in the file
5. Moves to the next task
6. Auto-switches from Claude to Codex if usage limits are hit

### Recursive Loops

The `--wait` flag enables recursive task decomposition. A coder can spawn sub-loops and wait for them to complete:

```bash
# From within a coder session working on a complex task:
coders loop --wait --todolist subtasks.txt --cwd .
# Blocks until all subtasks complete
# Then the parent coder continues its work
```

This creates task decomposition trees:

```
Orchestrator
  └── Loop A (feature tasks)
        ├── Coder A1 (simple task) → completes
        ├── Coder A2 (complex task)
        │     └── Loop A2 --wait (subtasks)
        │           ├── Coder A2a → completes
        │           └── Coder A2b → completes
        │     # A2 continues after sub-loop completes
        └── Coder A3 → completes
```

### Monitoring

```bash
# Check loop status
coders loop-status
coders loop-status --loop-id loop-1234567890

# View log
tail -f /tmp/coders-loop-loop-1234567890.log
```

## Coders Loops vs Ralph Loops

[Ralph loops](https://ghuntley.com/ralph/) are a popular technique for iterative AI development using a bash while loop that repeatedly feeds Claude the same prompt until completion. Coders loops take a fundamentally different approach that's more powerful for complex work.

### Key Differences

| Aspect | Ralph Loop | Coders Loop |
|--------|------------|-------------|
| **Session model** | Single session, same prompt repeated | Fresh session per task |
| **Context** | Accumulates over iterations, eventually hits limits | Clean context for each task |
| **Task structure** | One monolithic prompt | Multiple discrete tasks from todolist |
| **Parallelization** | Sequential only | Parallel support (planned) |
| **Delegation** | Cannot spawn sub-agents | Recursive loops with `--wait` |
| **Tool switching** | Manual | Auto-switches on rate limits |
| **State** | Files only | Redis + files, survives crashes |
| **Visibility** | None | TUI, dashboard, loop-status |
| **Completion** | String matching in output | Explicit promise system |

### When to Use Each

**Ralph loops are good for:**
- Single, well-defined tasks with clear completion criteria
- Tasks where context accumulation helps (iterative refinement)
- Simple "keep trying until it works" scenarios

**Coders loops are better for:**
- Multiple related tasks (feature implementation with tests, docs, etc.)
- Complex work requiring task decomposition
- Long-running work where context limits matter
- Work requiring different tools for different subtasks
- Team/orchestration scenarios with visibility needs
- Work that might hit API rate limits

### The Power of Recursive Decomposition

The real power of coders loops comes from recursive decomposition. When a coder encounters a complex task, it can:

1. Analyze the task and identify subtasks
2. Write a subtask todolist
3. Spawn a sub-loop with `--wait`
4. Each subtask gets a fresh coder with full context
5. Sub-loop completes, parent coder continues
6. Parent coder integrates results and completes its own task

This is impossible with Ralph loops, which are limited to a single session repeatedly executing the same prompt. Coders loops enable true hierarchical task decomposition with clean context boundaries at each level.
