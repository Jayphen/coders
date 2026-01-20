---
description: Spawn an AI coding assistant in a new tmux session
execute: node skills/main.js spawn
---

# Spawn an AI coding assistant in a new tmux session

**Execute with:** `node skills/main.js spawn [tool] [options]` from the plugin root directory

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
