---
description: Restore tmux sessions from the latest snapshot
---

# Restore tmux sessions

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js restore
```

Restore tmux sessions from the latest snapshot.

## Usage

```
/coders:restore
```

## Description

Restores tmux sessions from the most recent snapshot saved in `~/.coders/snapshots/`.

If multiple snapshots exist, use the latest one.

## See Also

- `/coders:snapshot` - Create a snapshot
