---
name: orchestrator
description: Start or attach to the orchestrator session for coordinating multiple coder sessions
---

# Orchestrator Session

The orchestrator is a special persistent session that can be used to coordinate and manage multiple coder sessions.

## Usage

```bash
coders orchestrator
```
CLI alternative:
```bash
./bin/coders orchestrator
```

## Features

- **Persistent Session ID**: Uses `coder-orchestrator` (not timestamp-based)
- **State Management**: Tracks orchestrator state in Redis (key: `coders:orchestrator:meta`)
- **Special Capabilities**: Can spawn and kill other coder sessions
- **Dashboard Integration**: Displayed at the top of the dashboard with special styling

## How it Works

1. On first run, creates a new tmux session with ID `coder-orchestrator`
2. If the session already exists, attaches to it
3. Spawns Claude Code with special orchestrator prompt
4. Enables heartbeat tracking for dashboard monitoring
5. Auto-attaches to the session after creation

## Orchestrator Capabilities

Within the orchestrator session, you can:

- Spawn new coder sessions: `coders spawn <tool> [options]`
- List active sessions: `coders list`
- Attach to sessions: `coders attach <session>`
- Kill sessions: `coders kill <session>`
- Open dashboard: `coders dashboard`

## Example

```bash
# Start or attach to orchestrator
coders orchestrator

# Inside orchestrator, spawn new sessions
coders spawn claude --task "Implement user authentication"
coders spawn gemini --task "Fix the API bug"

# List all sessions
coders list

# Attach to a specific session
coders attach claude-implement-user-authentication
```
