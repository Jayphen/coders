---
description: Attach to a coder session
---

# Attach to a coder session

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js attach $ARGUMENTS
```

Attach to an existing tmux session spawned by the coders plugin.

## Usage

```
/coders:attach <session-name>
```

## Arguments

- `session-name` - The name of the session to attach to

## Examples

```
/coders:attach coder-claude-123456
/coders:attach feature-auth
```

## See Also

- `/coders:list` - List all active sessions
- `/coders:kill` - Kill a session
