# Coders Usage Examples

Practical examples for common workflows using the Coders plugin.

Note: Examples use Claude Code slash commands. CLI alternative: replace `/coders:<command>` with `./bin/coders <command>` (or `coders <command>` if `bin/` is on your PATH).

## Basic Usage

### Spawn a Simple Claude Session

```bash
/coders:spawn claude --task "Review the authentication code and suggest improvements"
```

This creates a new tmux session with Claude and gives it the task.

---

### Spawn with a Worktree

```bash
/coders:spawn claude --task "Implement OAuth login" --worktree feature/oauth-login
```

Creates a git worktree at `../worktrees/feature/oauth-login` and spawns Claude there.

---

### Spawn with a PRD

```bash
/coders:spawn claude --task "Build the user dashboard" --prd docs/dashboard-spec.md
```

Includes the PRD file as context for the AI.

---

## Working with Multiple Sessions

### List Active Sessions

```bash
/coders:list
```

Output:
```
📋 Active Coder Sessions:
coder-claude-oauth-login: 1 windows (created today at 10:30)
coder-gemini-api-research: 1 windows (created today at 11:15)
```

---

### Attach to a Session

```bash
/coders:attach claude-oauth-login
```

Tells you how to attach:
```
Run: `tmux attach -t coder-claude-oauth-login`
```

Then press `Ctrl+B` then `D` to detach.

---

### Kill a Session

```bash
/coders:kill claude-oauth-login
```

Terminates the tmux session.

---

## Advanced Workflows

### Multi-Agent Coordination

**Scenario:** Use an orchestrator to coordinate multiple specialized agents.

1. **Start the orchestrator:**
```bash
/coders:orchestrator
```

2. **Inside the orchestrator, spawn specialized agents:**
```bash
/coders:spawn claude --task "Implement backend API endpoints" --worktree backend/api
/coders:spawn gemini --task "Research best practices for API authentication"
/coders:spawn claude --task "Write integration tests" --worktree backend/tests
```

3. **Monitor all sessions:**
```bash
/coders:dashboard
```

Opens a web dashboard showing all active sessions with live status.

---

### Working with Git Worktrees

**Scenario:** Develop multiple features in parallel without branch switching.

```bash
# Feature 1: Authentication
/coders:spawn claude --task "Add OAuth support" --worktree feature/oauth

# Feature 2: Dashboard
/coders:spawn claude --task "Build user dashboard" --worktree feature/dashboard

# Feature 3: API
/coders:spawn gemini --task "Implement REST API" --worktree feature/api
```

Each session works in its own isolated worktree.

---

### Using Redis for Inter-Agent Communication

**Scenario:** Coordinate agents using Redis pub/sub.

1. **Configure Redis:**
```typescript
coders.configure({
  redis: { url: 'redis://localhost:6379' }
});
```

2. **Spawn with heartbeat:**
```bash
/coders:spawn claude --task "Build the frontend" --redis redis://localhost:6379 --enable-heartbeat
```

3. **Spawn a monitoring agent:**
```bash
/coders:spawn claude --task "Monitor all agents and report status"
```

Inside the monitoring agent:
```typescript
await coders.listenForMessages('agent-heartbeat', (msg) => {
  console.log('Agent heartbeat:', msg);
});
```

---

## Session Persistence

### Save a Snapshot

Before shutting down your machine, save all sessions:

```bash
/coders:snapshot
```

Output:
```
Snapshot saved to ~/.coders/snapshots/2024-01-20-10-30-45.json
```

---

### Restore from Snapshot

After reboot, restore all sessions:

```bash
/coders:restore
```

Output:
```
Restored 3 sessions from snapshot:
- coder-claude-oauth-login
- coder-gemini-api-research
- coder-claude-dashboard
```

---

## Real-World Scenarios

### Scenario 1: Code Review Workflow

```bash
# Spawn a reviewer agent
/coders:spawn claude --task "Review PR #123 and provide feedback"

# Spawn a fixer agent in a worktree
/coders:spawn claude --task "Fix issues found in PR #123" --worktree fix/pr-123

# Monitor both
/coders:dashboard
```

---

### Scenario 2: Research + Implementation

```bash
# Research agent
/coders:spawn gemini --task "Research best practices for implementing WebSockets in Node.js"

# Implementation agent (spawned after research is done)
/coders:spawn claude --task "Implement WebSocket server based on research" --worktree feature/websockets

# Attach to research agent to read findings
tmux attach -t coder-gemini-websockets-research
```

---

### Scenario 3: Parallel Testing

```bash
# Unit tests
/coders:spawn claude --task "Write unit tests for auth module" --worktree tests/unit

# Integration tests
/coders:spawn claude --task "Write integration tests for API" --worktree tests/integration

# E2E tests
/coders:spawn claude --task "Write E2E tests for user flows" --worktree tests/e2e
```

---

## Tips and Tricks

### Auto-Generated Session Names

Session names are automatically generated from task descriptions:

```bash
/coders:spawn claude --task "Review the Linear project implementation"
# Creates: coder-claude-linear-project
```

### Custom Session Names

Override the auto-generated name:

```bash
/coders:spawn claude --task "Fix the bug" --name bug-fix-session
# Creates: coder-bug-fix-session
```

### Send Messages to Sessions Remotely

Without attaching to the session:

```bash
# Send a message
tmux send-keys -t coder-claude-oauth-login "Please commit your changes"
sleep 0.5
tmux send-keys -t coder-claude-oauth-login C-m

# Check output
tmux capture-pane -t coder-claude-oauth-login -p | tail -20
```

### Check Session Output

View recent output without attaching:

```bash
tmux capture-pane -t coder-claude-oauth-login -p | tail -50
```

---

## Programmatic API Examples

### TypeScript/JavaScript Usage

```typescript
import { coders } from '@jayphen/coders';

// Quick spawn
await coders.claude('Fix authentication bug', {
  worktree: 'fix/auth'
});

// With full options
await coders.spawn({
  tool: 'claude',
  task: 'Implement user dashboard',
  worktree: 'feature/dashboard',
  prd: 'docs/dashboard-spec.md',
  redis: { url: 'redis://localhost:6379' },
  enableHeartbeat: true
});

// List and manage
const sessions = coders.getActiveSessions();
console.log(`Active sessions: ${sessions.length}`);

// Kill a session
coders.kill('claude-dashboard');

// Inter-agent communication
await coders.sendMessage('build-status', {
  status: 'complete',
  branch: 'feature/dashboard'
});

await coders.listenForMessages('build-status', (msg) => {
  console.log('Build status:', msg);
});
```

---

## Common Issues

### Session Not Found

If `tmux attach` says session not found:

```bash
# List all sessions
/coders:list

# Verify the session exists
tmux list-sessions | grep coder-
```

### Worktree Already Exists

If you get "worktree already exists":

```bash
# List worktrees
git worktree list

# Remove stale worktree
git worktree remove ../worktrees/feature/oauth-login

# Then try again
/coders:spawn claude --task "..." --worktree feature/oauth-login
```

### Redis Connection Failed

Ensure Redis is running:

```bash
# Start Redis
redis-server

# Or with Docker
docker run -d -p 6379:6379 redis:latest
```

---

## See Also

- [API Reference](reference.md) - Complete API documentation
- [SKILL.md](SKILL.md) - Quick start guide
