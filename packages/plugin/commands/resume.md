---
description: Resume a completed session, marking it as active again
---

# Resume a completed session

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js resume $ARGUMENTS
```

Clear a session's promise, marking it as active again. Use this when more work is needed on a session that was previously marked as completed.

## Usage

```
/coders:resume [session-name]
```

## Options

- `session-name` (optional): The session to resume. If omitted, resumes the current session.

## Examples

```
/coders:resume                    # Resume current session
/coders:resume auth-fix           # Resume the coder-auth-fix session
```

## When to use

- A completed task needs additional work
- A blocked session is now unblocked
- You accidentally marked a session as completed
