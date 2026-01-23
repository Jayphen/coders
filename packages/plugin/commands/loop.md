---
description: Start a recursive loop that auto-spawns sessions based on a todolist, using promises to signal task completion
---

# Start Recursive Task Loop

Execute:
```bash
${CLAUDE_PLUGIN_ROOT}/bin/coders loop $ARGUMENTS
```

Start a recursive workflow loop that automatically spawns coder sessions to complete tasks from a todolist file. Each session publishes a promise when complete, triggering the next task.

## Usage

```
/coders:loop --todolist path/to/todolist.txt --cwd ~/project [options]
```

## Options

- `--todolist` (required) - Path to todolist file with tasks
- `--cwd` (required) - Working directory for all spawned sessions
- `--tool` - AI tool to use (default: claude)
- `--model` - Model to use for all sessions
- `--max-concurrent` - Maximum concurrent sessions (default: 1)
- `--stop-on-blocked` - Stop loop if any task is blocked (default: false)
- `--dry-run` - Show what would be spawned without actually spawning

## Todolist Format

The todolist file should have tasks in this format:

```markdown
# Project Todolist

## IMMEDIATE TASKS
[ ] Task 1 description
[ ] Task 2 description

## PHASE 1
[ ] Task 3 description
[ ] Task 4 description
```

Tasks are executed in order. Checked tasks `[x]` are skipped.

## Examples

```bash
# Start recursive loop for beach-crowd project
/coders:loop --todolist ~/dev/beach-crowd/todolist.txt --cwd ~/dev/beach-crowd

# Use opus for complex tasks
/coders:loop --todolist tasks.txt --cwd ~/project --model claude-opus-4

# Run 2 tasks in parallel
/coders:loop --todolist tasks.txt --cwd ~/project --max-concurrent 2

# Dry run to see execution plan
/coders:loop --todolist tasks.txt --cwd ~/project --dry-run
```

## How It Works

1. **Parse todolist** - Reads the todolist file and extracts uncompleted tasks
2. **Check usage** - Before each task, checks if usage cap is approaching
3. **Auto-switch tools** - If Claude usage is above 90%, automatically switches to codex
4. **Spawn session** - Creates coder session for the task with the selected tool
5. **Monitor promises** - Watches Redis for completion promise
6. **Auto-spawn next** - When promise received, spawns next task
7. **Repeat** - Continues until all tasks complete or error occurs
8. **Background mode** - Runs in background, orchestrator remains interactive

## Usage Cap Management

The loop automatically monitors Claude usage and switches to codex when approaching limits:

- After each task completes, checks the session output for Claude's usage warning message
- Detects patterns like "⚠️ You're approaching your usage limit" or "90% of your weekly limit"
- If warning detected, automatically switches to codex for all remaining tasks
- Prevents workflow interruption due to rate limits
- Seamlessly continues work with alternative tool
- Shows in log: "⚠️ Detected Claude usage warning - switching to codex for remaining tasks"

## Loop Control

The loop can be controlled while running:

```bash
# Check loop status
/coders:loop-status

# Pause the loop (finish current tasks but don't spawn new)
/coders:loop-pause

# Resume paused loop
/coders:loop-resume

# Stop loop (kills all spawned sessions)
/coders:loop-stop
```

## Promise Integration

Each spawned session automatically:
- Receives task from todolist
- Works on the task
- Commits and pushes changes
- Publishes completion promise
- Loop detects promise and spawns next task

## Error Handling

If a session publishes a **blocked** promise:
- Loop pauses (if `--stop-on-blocked`)
- Notifies orchestrator
- Waits for manual intervention
- Can be resumed after unblocking

## Monitoring

```bash
# View all promises from loop
/coders:promises

# See active sessions from loop
/coders:list

# Attach to current working session
/coders:attach <session-name>
```

## Use Cases

### 1. Complete Full Project from PRD
```bash
# Generate todolist from PRD first
# Then run recursive loop
/coders:loop --todolist project-tasks.txt --cwd ~/my-project
```

### 2. Automated Feature Development
```bash
# Break feature into tasks
# Loop handles implementation
/coders:loop --todolist feature-tasks.txt --cwd ~/app
```

### 3. Multi-Phase Projects
```bash
# Execute 100+ tasks automatically
# Review after each phase
/coders:loop --todolist phases.txt --cwd ~/app --stop-on-blocked
```

## Notes

- Loop state is stored in Redis
- Survives orchestrator restarts
- Can run multiple loops in different projects
- Each loop has a unique ID
- Background process doesn't block orchestrator
