---
description: Spawn an AI coding assistant in a new tmux session
---

# Spawn an AI coding assistant in a new tmux session

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js spawn $ARGUMENTS
```

Spawn Claude, Gemini, Codex, or OpenCode in an isolated tmux window with optional git worktree.

## Usage

```
/coders:spawn [tool] --task "description" --worktree branch --prd file.md --redis url
```

## Options

- `tool` - AI tool: claude, gemini, codex, opencode (default: claude)
- `task` - Task description for the AI
- `worktree` - Git branch for worktree (optional)
- `prd` - PRD/spec file path (optional)
- `redis` - Redis URL for heartbeat (optional)

## Examples

```
/coders:spawn claude --task "Build the authentication module"
/coders:spawn gemini --task "Research JWT vs Session auth" --worktree research/jwt
/coders:spawn opencode --task "Fix the login bug" --prd docs/auth-prd.md
```

## With Redis Heartbeat

```
/coders:spawn claude --task "Build auth" --redis redis://localhost:6379 --enable-heartbeat
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
./scripts/send-to-session.sh coder-SESSION_ID "your message"

# Manually (two-step required for TUI CLIs)
tmux send-keys -t coder-SESSION_ID "your message"
sleep 0.5
tmux send-keys -t coder-SESSION_ID C-m
```

### Check session output
```bash
tmux capture-pane -t coder-SESSION_ID -p | tail -20
```
