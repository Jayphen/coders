---
description: Publish a completion promise for this coder session
---

# Publish a completion promise

Execute:
```bash
${CLAUDE_PLUGIN_ROOT}/bin/coders promise $ARGUMENTS
```

Mark the current session as completed with a summary of what was accomplished. This notifies the orchestrator and updates the dashboard/TUI to show this session as "completed".

## Usage

```
/coders:promise "summary of what was done" [--status completed|blocked|needs-review] [--blockers "reason"]
```

## Options

- First argument (required): Summary of what was accomplished
- `--status` - Status: completed (default), blocked, or needs-review
- `--blockers` - Reason for being blocked (used with --status blocked)

## Examples

```
/coders:promise "Fixed the auth bug and added unit tests"
/coders:promise "Implemented responsive navbar with dark mode support"
/coders:promise "Waiting on API credentials" --status blocked --blockers "Need API key from ops team"
/coders:promise "Ready for code review" --status needs-review
```

## Integration

Once a promise is published:
- The session appears in the "Completed" section of dashboard/TUI
- The orchestrator is notified and can spawn follow-up tasks
- Completed sessions can be bulk-killed to clean up

Use `/coders:resume` to mark a completed session as active again.
