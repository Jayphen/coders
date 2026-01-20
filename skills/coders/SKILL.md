---
name: coders
description: Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with git worktrees, Redis coordination, and session persistence. Use when you need to run multiple AI agents in parallel, coordinate multi-agent workflows, or isolate work in separate git branches.
allowed-tools: Bash(node:*), Bash(tmux:*)
---

# Coders - Multi-Agent Development Assistant

Spawn and manage AI coding assistants in isolated tmux sessions with optional git worktrees, Redis-based coordination, and automatic session recovery.

## Quick Start

### Spawn a new session

```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js spawn claude --task "Implement OAuth authentication"
```

CLI alternative (outside Claude Code):
```bash
./bin/coders spawn claude --task "Implement OAuth authentication"
```

### With a git worktree

```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js spawn claude --task "Fix login bug" --worktree fix/login-bug
```

### List active sessions

```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js list
```

### Attach to a session

```bash
tmux attach -t coder-SESSION_ID
```

Press `Ctrl+B` then `D` to detach.

## Features

✨ **Multi-Agent Support**: Spawn Claude, Gemini, Codex, or OpenCode in parallel
🌲 **Git Worktrees**: Isolate work in separate branches without switching
🔄 **Session Persistence**: Save and restore sessions across reboots
📡 **Redis Coordination**: Inter-agent messaging and heartbeat monitoring
🤖 **Auto-Respawn**: Automatically restart unresponsive agents
📊 **Web Dashboard**: Monitor all sessions in real-time
🎯 **Smart Naming**: Auto-generate session names from task descriptions

## Available Commands

All commands use the skill syntax:

- `/coders:spawn` - Spawn a new AI session
- `/coders:list` - List active sessions
- `/coders:attach` - Attach to a session
- `/coders:kill` - Kill a session
- `/coders:snapshot` - Save all sessions
- `/coders:restore` - Restore saved sessions
- `/coders:orchestrator` - Start orchestrator session
- `/coders:dashboard` - Open web dashboard

## Common Workflows

### Basic Usage

```bash
# Spawn Claude for a simple task
/coders:spawn claude --task "Review the authentication code"

# Spawn Gemini for research
/coders:spawn gemini --task "Research WebSocket best practices"

# Check what's running
/coders:list
```

### Multi-Agent Development

```bash
# Start orchestrator to coordinate agents
/coders:orchestrator

# Inside orchestrator, spawn specialized agents
/coders:spawn claude --task "Implement API endpoints" --worktree backend/api
/coders:spawn gemini --task "Research API security"
/coders:spawn claude --task "Write tests" --worktree backend/tests

# Monitor everything
/coders:dashboard
```

### Git Worktree Isolation

```bash
# Work on multiple features in parallel
/coders:spawn claude --task "Add OAuth" --worktree feature/oauth
/coders:spawn claude --task "Build dashboard" --worktree feature/dashboard

# Each session has its own isolated git worktree
```

### Session Persistence

```bash
# Before shutting down
/coders:snapshot

# After reboot
/coders:restore
```

## Redis Coordination

Enable Redis for inter-agent messaging and auto-respawn:

```bash
/coders:spawn claude --task "Build frontend" --redis redis://localhost:6379 --enable-heartbeat
```

Agents can:
- Send messages to each other via pub/sub
- Publish heartbeats for monitoring
- Auto-respawn if they become unresponsive (>2min timeout)

## Documentation

- **[API Reference](reference.md)** - Complete API documentation for programmatic usage
- **[Examples](examples.md)** - Real-world scenarios and code examples
- **Command docs** - Each command has its own detailed documentation

## Session Management

### Auto-Generated Names

Session names are automatically generated from task descriptions:

```bash
/coders:spawn claude --task "Review the Linear project"
# Creates: coder-claude-linear-project
```

### Manual Naming

```bash
/coders:spawn claude --task "Fix bug" --name my-custom-name
# Creates: coder-my-custom-name
```

### Communicating with Sessions

**Attach (recommended):**
```bash
tmux attach -t coder-SESSION_ID
# Work inside the session
# Ctrl+B then D to detach
```

**Send messages remotely:**
```bash
tmux send-keys -t coder-SESSION_ID "your message"
sleep 0.5
tmux send-keys -t coder-SESSION_ID C-m
```

**Check output:**
```bash
tmux capture-pane -t coder-SESSION_ID -p | tail -20
```

## Options Reference

### spawn command options:

- `tool` - AI tool: `claude`, `gemini`, `codex`, `opencode` (default: `claude`)
- `--task` - Task description (required)
- `--name` - Custom session name (optional, auto-generated if omitted)
- `--worktree` - Git branch for worktree (optional)
- `--base-branch` - Base branch for worktree (default: `main`)
- `--prd` - PRD/spec file path to include as context (optional)
- `--redis` - Redis URL for coordination (optional)
- `--enable-heartbeat` - Enable heartbeat publishing (optional)
- `--enable-dead-letter` - Enable auto-respawn on timeout (optional)

## Requirements

- **tmux** - Required for session management
- **Redis** - Required for coordination and heartbeat monitoring
- **git** - For worktree support (optional)
- **Node.js 18+** - Runtime

## Tips

1. **Use the orchestrator** for complex multi-agent workflows
2. **Save snapshots regularly** to preserve session state
3. **Use worktrees** to avoid branch switching conflicts
4. **Enable Redis heartbeat** for production-like agent monitoring
5. **Use the dashboard** to monitor multiple sessions visually

## Troubleshooting

### Session not found
```bash
/coders:list  # Verify session exists
tmux list-sessions | grep coder-  # Check tmux directly
```

### Worktree already exists
```bash
git worktree list  # List worktrees
git worktree remove ../worktrees/BRANCH  # Remove stale worktree
```

### Redis connection failed
```bash
redis-server  # Start Redis
# or
docker run -d -p 6379:6379 redis:latest
```

## See Also

- [Reference](reference.md) - Full API documentation
- [Examples](examples.md) - Usage examples and patterns
