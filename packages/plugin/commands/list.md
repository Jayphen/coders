---
description: List all active coder sessions
---

# List all active coder sessions

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js list
```
CLI alternative:
```bash
./bin/coders list
```

Show all tmux sessions spawned by the coders plugin.

## Usage

```
/coders:list
```

## Example Output

```
coder-claude-123456: 1 windows (created today at 10:30)
coder-gemini-789012: 1 windows (created today at 11:15)
```
