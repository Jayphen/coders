---
description: Spawn an AI coding assistant in a new tmux session
---

# Spawn an AI coding assistant in a new tmux session

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js spawn $ARGUMENTS
```
CLI alternative:
```bash
./bin/coders spawn $ARGUMENTS
```

Spawn Claude, Gemini, Codex, or OpenCode in an isolated tmux window with optional git worktree.

## Usage

```
/coders:spawn [tool] --task "description" [--cwd path] [--worktree branch] [--prd file.md]
```

## Options

- `tool` - AI tool: claude, gemini, codex, opencode (default: claude)
- `--name` - Session name (auto-generated from task if omitted)
- `--task` - Task description for the AI
- `--cwd`, `--dir` - Working directory for the session (default: git root). Supports [zoxide](https://github.com/ajeetdsouza/zoxide) for smart path resolution (e.g., `--cwd myproj` will resolve to your most frecent matching directory)
- `--worktree` - Git branch for worktree (optional)
- `--base` - Base branch for worktree (default: main)
- `--prd`, `--spec` - PRD/spec file path (optional)
- `--no-heartbeat` - Disable heartbeat tracking (enabled by default)

## Examples

```
/coders:spawn claude --task "Build the authentication module"
/coders:spawn gemini --task "Research JWT vs Session auth" --worktree research/jwt
/coders:spawn opencode --task "Fix the login bug" --prd docs/auth-prd.md
/coders:spawn claude --cwd ~/projects/myapp --task "Refactor the API layer"
/coders:spawn claude --cwd myapp --task "Fix bug"  # Uses zoxide to resolve "myapp"
```

## Dashboard Integration

Heartbeat tracking is enabled by default. Sessions will appear in the dashboard.

```
/coders:spawn claude --task "Build auth"         # Heartbeat enabled (default)
/coders:spawn claude --task "Quick test" --no-heartbeat  # Disable heartbeat
```

## Communicating with Spawned Sessions

All spawned sessions run in **interactive mode** and persist until you kill them.

### Attach to session (recommended)
```bash
tmux attach -t coder-SESSION_ID
# Press Ctrl+B then D to detach
```

### Send messages remotely
```bash
# Using helper script
./bin/send-to-session.sh coder-SESSION_ID "your message"

# Manually (two-step required for TUI CLIs)
tmux send-keys -t coder-SESSION_ID "your message"
sleep 0.5
tmux send-keys -t coder-SESSION_ID C-m
```

### Check session output
```bash
tmux capture-pane -t coder-SESSION_ID -p | tail -20
```
