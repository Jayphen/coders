# Coder Spawner

Spawn AI coding assistants (Claude, Gemini, Codex) in isolated tmux sessions with optional git worktrees.

## Two Ways to Use

### 1. CLI (any terminal)

```bash
# Spawn Claude with worktree and PRD
coders spawn claude --worktree feature/auth --prd docs/prd.md

# Spawn Gemini
coders spawn gemini --name my-session --task "Fix the login bug"

# List sessions
coders list

# Attach to session
coders attach feature-auth
```

### 2. Claude Code Skill

Install as a Claude Code skill:

```bash
cd ~/code
git clone https://github.com/Jayphen/coders.git
cd coders
npm install
```

#### Interactive Mode (prompts for missing info)

```typescript
import { coders } from '@jayphen/coders';

// Just say "spawn Claude" and it prompts for task/worktree
await coders.spawn({ tool: 'claude' });
// Prompts:
// - "What should this session work on?"  
// - "Create a git worktree?" 
// - "Include a PRD file?"
```

#### Direct Mode (all options upfront)

```typescript
import { coders } from '@jayphen/coders';

// Spawn Claude in new tmux window
await coders.spawn({
  tool: 'claude',
  task: 'Refactor the authentication module',
  worktree: 'feature/auth-refactor',
  prd: 'docs/auth-prd.md'
});

// Quick helpers - minimal typing
await coders.claude('Fix the bug', { worktree: 'fix-auth' });
await coders.gemini('Research JWT approaches');

// List, attach, kill
coders.list();
coders.attach('session-name');
coders.kill('session-name');

// Quick worktree syntax
await coders.worktree('feature/new', 'Build the new feature');
```

## Features

- **Git Worktrees**: Creates isolated branches for each task
- **PRD Priming**: Feeds context to the AI before it starts
- **Tmux Sessions**: Runs in separate tmux windows
- **Recursive Spawning**: Claude instances can spawn more Claudes (`--dangerously-spawn-permission`)
- **Interactive Prompts**: Asks for missing info when options aren't provided

---

## Redis Heartbeat & Auto-Respawn

The coders skill supports Redis-powered heartbeat monitoring and automatic respawn for unresponsive agents.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Redis Pub/Sub                             │
├─────────────────────────────────────────────────────────────┤
│  HEARTBEAT_CHANNEL: coders:heartbeats                        │
│  DEAD_LETTER_KEY:   coders:dead-letter                       │
│  PANE_ID_PREFIX:    coders:pane:                             │
└─────────────────────────────────────────────────────────────┘
         │                    │
         ▼                    ▼
   ┌──────────┐         ┌──────────────┐
   │  Agent   │──publish│  Dead-Letter │
   │Heartbeat │──BLPOP──│   Listener   │
   └──────────┘         └──────────────┘
         │                    │
         ▼                    ▼
   ┌─────────────────┐  ┌─────────────┐
   │   Redis Set     │  │ respawn-    │
   │   (150s TTL)    │  │ pane -k -t  │
   └─────────────────┘  └─────────────┘
```

### Enable Redis Heartbeat

```typescript
import { coders, RedisManager } from '@jayphen/coders';

// Spawn with Redis heartbeat enabled
await coders.spawn({
  tool: 'claude',
  task: 'Build the new feature',
  redis: {
    url: 'redis://localhost:6379',
    // or individual config:
    // host: 'localhost',
    // port: 6379,
    // password: 'secret'
  },
  enableHeartbeat: true,
  enableDeadLetter: true
});

// Or configure globally
coders.configure({
  redis: { url: 'redis://localhost:6379' }
});

await coders.spawn({
  tool: 'claude',
  task: 'Quick task with heartbeat'
});
```

### Agent Heartbeat Integration

Agents can publish heartbeats to Redis for monitoring:

```typescript
import { RedisManager, getPaneId, injectPaneIdContext } from '@jayphen/coders';

// Get or create pane ID
const paneId = getPaneId();

// Inject pane ID into system context (for Claude prompts)
const systemPrompt = injectPaneIdContext(originalPrompt, paneId);

// Publish heartbeats
const redis = new RedisManager({ url: 'redis://localhost:6379' });
await redis.connect();
await redis.startHeartbeat(30000); // Every 30 seconds

// Or publish manually
await redis.publishHeartbeat('processing', 'Writing tests for auth module');
```

### Auto-Respawn Behavior

- Heartbeats expire after 150 seconds (2.5 min)
- Dead-letter listener uses BLPOP with no timeout
- When a pane times out, it gets added to dead-letter queue
- Listener triggers `respawn-pane -k -t $pane` to restart the agent

### Environment Variables

```bash
# Auto-set by spawn:
CODERS_PANE_ID=Pane-ID-Here
CODERS_SESSION_ID=coder-claude-1234567890
REDIS_URL=redis://localhost:6379

# Manual override:
export REDIS_PASSWORD=your-password
```

---

## Inter-Agent Communication

Agents can communicate via Redis pub/sub:

```typescript
import { coders } from '@jayphen/coders';

// Send message to another agent
await coders.sendMessage('coordination', {
  type: 'task-complete',
  taskId: 'auth-module',
  result: 'completed'
});

// Listen for messages from other agents
await coders.listenForMessages('coordination', (message) => {
  console.log('Received:', message);
  if (message.type === 'task-complete') {
    // Handle completion
  }
});
```

---

## Tmux Resurrect (Snapshot & Restore)

Backup and restore tmux sessions for disaster recovery.

### Quick Usage

```typescript
import { tmuxResurrect, snapshot, restore, listSnapshots } from '@jayphen/coders';

// Snapshot all coder sessions
const snap = snapshot();

// Restore from latest snapshot
const result = restore();
console.log(`Restored: ${result.restored}, Failed: ${result.failed}`);

// List available snapshots
const snapshots = listSnapshots({ sortBy: 'date', reverse: true });
/*
[
  { filename: 'snapshot-2024-01-15T10-30-00.json', timestamp: 1705313400000, size: 1234 },
  { filename: 'snapshot-2024-01-15T10-00-00.json', timestamp: 1705310400000, size: 1180 }
]
*/

// Restore from specific snapshot
restore('/home/user/.coders/snapshots/snapshot-2024-01-15T10-30-00.json');
```

### CLI Usage

Snapshots are saved to `~/.coders/snapshots/`:

```bash
# Snapshots are created automatically when using spawnWithHeartbeat
ls ~/.coders/snapshots/

# Manual snapshot
node -e "require('./dist/claude-code/tmux-resurrect').snapshot()"

# Restore
node -e "require('./dist/claude-code/tmux-resurrect').restore()"
```

### Snapshot Format

```json
{
  "metadata": {
    "version": 1,
    "timestamp": 1705313400000,
    "hostname": "pi3",
    "platform": "linux",
    "sessionCount": 3
  },
  "sessions": [
    {
      "sessionName": "coder-claude-abc123",
      "sessionId": "coder-1705313400000",
      "createdAt": "2024-01-15T10:30:00.000Z",
      "windows": [
        {
          "windowId": "%0",
          "windowName": "claude",
          "layout": "dd46,240x60,0,0[240x40,0,0,240x19,0,41{120x19,0,41,120x19,121,41}]",
          "panes": [
            {
              "paneId": "%0",
              "windowId": "%0",
              "sessionName": "coder-claude-abc123",
              "command": "bash",
              "workingDirectory": "/home/pi/clawd",
              "environment": {
                "CODERS_PANE_ID": "pane-12345-abc",
                "CODERS_SESSION_ID": "coder-claude-abc123"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

---

## Requirements

- tmux installed
- Redis (optional, for heartbeat/pub/sub)
- Claude Code CLI (`npm i -g @anthropic-ai/claude-code`) - optional
- Gemini CLI (`npm i -g @googlelabs/gemini-cli`) - optional
- OpenAI Codex CLI (`pip install openai-codex`) - optional

### Redis Installation

```bash
# Ubuntu/Debian
sudo apt install redis-server
sudo systemctl enable redis-server
sudo systemctl start redis-server

# macOS
brew install redis
brew services start redis

# Verify
redis-cli ping
# Should return: PONG
```

## License

MIT
