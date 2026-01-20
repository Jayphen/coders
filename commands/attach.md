---
description: Attach to a coder session
execute: node skills/main.js attach
---

# Attach to a coder session

**Execute with:** `node skills/main.js attach <session-name>` from the plugin root directory

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
