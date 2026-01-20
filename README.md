# Coder Spawner

Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with optional git worktrees.

## Two Ways to Use

### 1. Claude Code Plugin (Recommended)

Install as a Claude Code plugin:

```bash
cd ~/code
git clone https://github.com/Jayphen/coders.git
cd coders
```

Claude Code will auto-discover the plugin from `.claude-plugin/plugin.json`. No npm install needed - TypeScript files are loaded directly!

**Available slash commands:**
```bash
/coders:spawn claude --task "Build auth" --worktree feature/auth
/coders:list
/coders:attach my-session
/coders:kill my-session
/coders:snapshot
/coders:restore
```

**Or use as a skill in Claude Code:**
```typescript
import { coders } from '@jayphen/coders';

// Spawn Claude with worktree
await coders.spawn({
  tool: 'claude',
  task: 'Refactor the authentication module',
  worktree: 'feature/auth-refactor',
  prd: 'docs/auth-prd.md'
});

// Quick helpers
await coders.claude('Fix the bug', { worktree: 'fix-auth' });
await coders.opencode('Research JWT approaches');

coders.list();
coders.attach('session-name');
coders.kill('session-name');
```

### 2. Standalone CLI

```bash
# Clone and use directly
cd ~/code
git clone https://github.com/Jayphen/coders.git
cd coders

# Use the CLI
node skills/main.js spawn claude --task "Hello world"
node skills/main.js list
node skills/main.js attach my-session
```

Or link the CLI:
```bash
ln -sf ~/code/coders/skills/main.js ~/bin/coders
coders spawn claude --task "Hello world"
```

## Features

- **Git Worktrees**: Creates isolated branches for each task
- **PRD Priming**: Feeds context to the AI before it starts
- **Tmux Sessions**: Runs in separate tmux windows
- **Redis Heartbeat** (optional): Auto-respawn dead panes, pub/sub for inter-agent communication
- **Tmux Resurrect**: Snapshot/restore entire swarm

### Redis Heartbeat & Auto-Respawn (Optional)

Enable Redis for heartbeat monitoring and auto-respawn:

```typescript
await coders.spawn({
  tool: 'claude',
  task: 'Build auth module',
  redis: { url: 'redis://localhost:6379' },
  enableHeartbeat: true,
  enableDeadLetter: true
});
```

This will:
- Publish heartbeats every 10s to Redis
- Auto-respawn panes that miss 2 heartbeats (2min timeout)
- Enable inter-agent pub/sub communication

### Inter-Agent Communication

Send messages between spawned agents:

```typescript
await coders.sendMessage('target-session', 'Found a bug in auth!', { url: 'redis://localhost:6379' });
```

### Tmux Resurrect

Snapshot your entire swarm:

```typescript
import { snapshot, restore } from '@jayphen/coders';

snapshot();  // Saves to ~/.coders/snapshots/
restore();    // Restores from latest snapshot
```

## Requirements

- tmux installed
- Redis (optional, for heartbeat/pub/sub)
- Claude Code CLI (`npm i -g @anthropic-ai/claude-code`) - optional
- Gemini CLI (`npm i -g @googlelabs/gemini-cli`) - optional
- OpenAI Codex CLI (`pip install openai-codex`) - optional
- OpenCode CLI (`npm i -g @opencode/ai/cli`) - optional

## Project Structure

```
coders/
├── .claude-plugin/
│   └── plugin.json        # Plugin manifest (Claude Code discovers this)
├── skills/
│   ├── main.js           # CLI entry point
│   ├── test.js           # Test runner
│   ├── claude-code/
│   │   ├── coders.ts     # Claude Code skill (TypeScript, loaded directly)
│   │   ├── coders.d.ts   # Type definitions
│   │   └── redis.ts      # Redis heartbeat & pub/sub
│   └── tmux-resurrect.ts # Snapshot/restore utility
├── .gitignore
├── package.json
└── README.md
```

**Note:** No build step required! Claude Code loads `.ts` files directly.

## License

MIT
