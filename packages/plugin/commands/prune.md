---
description: Prune orphaned coder processes
---

# Prune orphaned processes

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js prune $ARGUMENTS
```
CLI alternative:
```bash
./bin/coders prune $ARGUMENTS
```

List or terminate orphaned `claude` and `heartbeat.js` processes that are no longer attached to tmux sessions.

## Usage

```
/coders:prune [--force] [--no-heartbeat] [--no-claude]
```

## Options

- `--force` - Terminate the orphaned processes (default is dry-run only)
- `--no-heartbeat` - Skip heartbeat.js processes
- `--no-claude` - Skip claude processes
- `--tmux-socket <path>` - Use a specific tmux socket
- `--allow-empty-tmux` - Allow prune even if tmux panes are not visible

## Examples

```
/coders:prune
/coders:prune --force
/coders:prune --no-heartbeat
/coders:prune --tmux-socket /private/tmp/tmux-501/default
```

## Warning

`--force` will terminate processes that are not attached to tmux. Use dry-run first to confirm the list. If tmux panes are not visible, prune will refuse unless you pass `--allow-empty-tmux` or supply the correct `--tmux-socket`.

## See Also

- `/coders:list` - List all active sessions
- `/coders:kill` - Kill a session
