# Snapshot tmux sessions

Save a snapshot of all tmux sessions to ~/.coders/snapshots/

## Usage

```
/coders:snapshot
```

## Description

Creates a backup of all current tmux sessions including:
- Session state and window layout
- Pane commands and working directories
- Environment variables

The snapshot is saved to `~/.coders/snapshots/` with a timestamp.

## See Also

- `/coders:restore` - Restore from a snapshot
