---
description: Kill a coder session
---

# Kill a coder session

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js kill $ARGUMENTS
```

Terminate an active tmux session spawned by the coders plugin.

## Usage

```
/coders:kill <session-name>
```

## Arguments

- `session-name` - The name of the session to kill

## Examples

```
/coders:kill coder-claude-123456
/coders:kill feature-auth
```

## Warning

This will terminate the tmux session and stop the AI process. This cannot be undone.

## See Also

- `/coders:list` - List all active sessions
- `/coders:attach` - Attach to a session
