# Coders API Reference

Complete API documentation for the Coders plugin. If you're launching from the terminal instead of Claude Code, use the CLI wrapper: `./bin/coders <command>`.

## Core Functions

### `coders.spawn(options)`

Spawn a new AI coding assistant in an isolated tmux session.

**Parameters:**
- `options.tool` (required): `'claude' | 'gemini' | 'codex' | 'opencode'`
- `options.task` (required): Task description for the AI
- `options.name` (optional): Custom session name (auto-generated if omitted)
- `options.worktree` (optional): Git branch name for worktree
- `options.baseBranch` (optional): Base branch for worktree (default: `'main'`)
- `options.prd` (optional): Path to PRD/spec file to include as context
- `options.interactive` (optional): Whether to run interactively (default: `true`)
- `options.redis` (optional): Redis configuration object
- `options.enableHeartbeat` (optional): Enable Redis heartbeat (default: `false`)
- `options.enableDeadLetter` (optional): Enable auto-respawn on timeout (default: `false`)
- `options.paneId` (optional): Tmux pane ID for coordination

**Returns:** `Promise<string>` - Success message with session details

**Example:**
```typescript
await coders.spawn({
  tool: 'claude',
  task: 'Refactor the authentication module',
  worktree: 'feature/auth-refactor',
  prd: 'docs/auth-prd.md'
});
```

---

### `coders.claude(task, options?)`

Quick spawn helper for Claude.

**Parameters:**
- `task` (required): Task description
- `options` (optional): Same as `spawn()` except `tool` is preset

**Returns:** `Promise<string>`

**Example:**
```typescript
await coders.claude('Fix the login bug', { worktree: 'fix-auth' });
```

---

### `coders.gemini(task, options?)`

Quick spawn helper for Gemini.

**Parameters:**
- `task` (required): Task description
- `options` (optional): Same as `spawn()` except `tool` is preset

**Returns:** `Promise<string>`

---

### `coders.codex(task, options?)`

Quick spawn helper for Codex.

**Parameters:**
- `task` (required): Task description
- `options` (optional): Same as `spawn()` except `tool` is preset

**Returns:** `Promise<string>`

---

### `coders.opencode(task, options?)`

Quick spawn helper for OpenCode.

**Parameters:**
- `task` (required): Task description
- `options` (optional): Same as `spawn()` except `tool` is preset

**Returns:** `Promise<string>`

---

### `coders.list()`

List all active coder sessions.

**Returns:** `string` - Formatted list of active sessions

**Example:**
```typescript
const sessions = coders.list();
console.log(sessions);
// Output:
// 📋 Active Coder Sessions:
// coder-claude-123456: 1 windows (created today at 10:30)
// coder-gemini-789012: 1 windows (created today at 11:15)
```

---

### `coders.attach(sessionName)`

Attach to an active coder session.

**Parameters:**
- `sessionName` (required): Name of the session (without `coder-` prefix)

**Returns:** `string` - Command to run for attaching

**Example:**
```typescript
const cmd = coders.attach('claude-123456');
// Returns: "Run: `tmux attach -t coder-claude-123456`"
```

---

### `coders.kill(sessionName)`

Kill an active coder session.

**Parameters:**
- `sessionName` (required): Name of the session (without `coder-` prefix)

**Returns:** `string` - Success or error message

**Example:**
```typescript
const result = coders.kill('claude-123456');
// Returns: "✅ Killed session: coder-claude-123456"
```

---

### `coders.worktree(branchName, task, options?)`

Quick helper to spawn with a git worktree.

**Parameters:**
- `branchName` (required): Branch name for worktree
- `task` (required): Task description
- `options` (optional):
  - `tool`: `'claude' | 'gemini' | 'codex'` (default: `'claude'`)
  - `prd`: Path to PRD file
  - `redis`: Redis configuration
  - `enableHeartbeat`: Enable heartbeat
  - `enableDeadLetter`: Enable auto-respawn

**Returns:** `Promise<string>`

**Example:**
```typescript
await coders.worktree('feature/new-auth', 'Implement OAuth', {
  tool: 'claude',
  prd: 'docs/oauth-spec.md'
});
```

---

### `coders.getActiveSessions()`

Get structured data about active sessions.

**Returns:** `CoderSession[]` - Array of session objects

**Session Object:**
```typescript
interface CoderSession {
  id: string;
  tool: string;
  task: string;
  worktree?: string;
  createdAt: Date;
  paneId?: string;
}
```

---

### `coders.configure(config)`

Configure global plugin settings.

**Parameters:**
- `config.redis` (optional): Default Redis configuration
- `config.snapshotDir` (optional): Directory for snapshots (default: `~/.coders/snapshots`)
- `config.deadLetterTimeout` (optional): Timeout in ms (default: `120000`)

**Example:**
```typescript
coders.configure({
  redis: { url: 'redis://localhost:6379' },
  snapshotDir: '~/my-snapshots',
  deadLetterTimeout: 180000 // 3 minutes
});
```

---

## Redis Integration

### `coders.spawnWithHeartbeat(options)`

Spawn a session with Redis heartbeat enabled.

**Parameters:**
- Same as `spawn()`, but `enableHeartbeat` and `enableDeadLetter` are set to `true`

**Returns:** `Promise<string>`

---

### `coders.sendMessage(channel, message, redisConfig?)`

Send a message to another agent via Redis pub/sub.

**Parameters:**
- `channel` (required): Redis channel name
- `message` (required): Any JSON-serializable object
- `redisConfig` (optional): Redis configuration (uses global config if omitted)

**Returns:** `Promise<void>`

**Example:**
```typescript
await coders.sendMessage('agent-coordination', {
  from: 'orchestrator',
  action: 'start_build',
  params: { branch: 'main' }
});
```

---

### `coders.listenForMessages(channel, callback, redisConfig?)`

Listen for messages from other agents via Redis pub/sub.

**Parameters:**
- `channel` (required): Redis channel name
- `callback` (required): Function to handle received messages
- `redisConfig` (optional): Redis configuration

**Returns:** `Promise<void>`

**Example:**
```typescript
await coders.listenForMessages('agent-coordination', (message) => {
  console.log('Received:', message);
  // Handle message
});
```

---

## Tmux Resurrect

### `coders.snapshot()`

Save a snapshot of all tmux sessions.

**Returns:** `Promise<string>` - Path to snapshot file

**Example:**
```typescript
const snapshotPath = await coders.snapshot();
// Returns: "Snapshot saved to ~/.coders/snapshots/2024-01-20-10-30-45.json"
```

---

### `coders.restore()`

Restore tmux sessions from the latest snapshot.

**Returns:** `Promise<string>` - Restoration result

**Example:**
```typescript
const result = await coders.restore();
// Returns: "Restored 3 sessions from snapshot"
```

---

### `coders.listSnapshots()`

List available snapshots.

**Returns:** `string[]` - Array of snapshot file paths

---

## Redis Exports

### `RedisManager`

Low-level Redis client manager.

**Methods:**
- `connect()`: Connect to Redis
- `disconnect()`: Disconnect from Redis
- `setPaneId(paneId)`: Store pane ID
- `getPaneId()`: Retrieve pane ID
- `publishMessage(channel, message)`: Publish message
- `subscribeToChannel(channel, callback)`: Subscribe to channel
- `startHeartbeat()`: Start heartbeat publishing
- `stopHeartbeat()`: Stop heartbeat publishing

---

### `DeadLetterListener`

Auto-respawn listener for dead agents.

**Methods:**
- `start()`: Start listening for dead agents
- `stop()`: Stop listening

---

## Constants

- `HEARTBEAT_CHANNEL`: Default Redis channel for heartbeats
- `DEAD_LETTER_KEY`: Default Redis key for dead letter queue
- `SESSION_PREFIX`: Tmux session name prefix (`'coder-'`)
- `WORKTREE_BASE`: Default worktree location (`'../worktrees'`)

---

## Types

### `SpawnOptions`
```typescript
interface SpawnOptions {
  tool: 'claude' | 'gemini' | 'codex' | 'opencode';
  task?: string;
  name?: string;
  worktree?: string;
  baseBranch?: string;
  prd?: string;
  interactive?: boolean;
  redis?: RedisConfig;
  enableHeartbeat?: boolean;
  enableDeadLetter?: boolean;
  paneId?: string;
}
```

### `RedisConfig`
```typescript
interface RedisConfig {
  url: string;
}
```

### `CoderSession`
```typescript
interface CoderSession {
  id: string;
  tool: string;
  worktree?: string;
  task: string;
  createdAt: Date;
  paneId?: string;
}
```

### `CodersConfig`
```typescript
interface CodersConfig {
  redis?: RedisConfig;
  snapshotDir?: string;
  deadLetterTimeout?: number;
}
```
