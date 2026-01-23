# Coder Spawner

Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with optional git worktrees.

## Prerequisites

- **tmux** - Required for session management
- **Redis** - Required for coordination and heartbeat monitoring

## Two Ways to Use

### 1. Claude Code Plugin (Recommended)

Install as a Claude Code plugin using the marketplace:

```bash
# Add the marketplace
claude plugin marketplace add https://github.com/Jayphen/coders.git

# Install the plugin
claude plugin install coders@coders
```

No npm install needed - TypeScript files are loaded directly!

**Available slash commands:**
```bash
/coders:spawn claude --task "Build auth" --worktree feature/auth
/coders:loop --todolist tasks.txt --cwd ~/project
/coders:list
/coders:attach my-session
/coders:kill my-session
/coders:prune --force
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
  model: 'claude-3-5-sonnet',
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

### 2. Standalone CLI (Optional)

```bash
# Clone and use directly
cd ~/code
git clone https://github.com/Jayphen/coders.git
cd coders

# Optional (required for dashboard/Redis features)
npm install

# Use the CLI via the bundled wrapper
./bin/coders spawn claude --task "Hello world"
./bin/coders list
./bin/coders attach my-session
```

Add it to your PATH:
```bash
export PATH="$PATH:$HOME/code/coders/bin"
coders spawn claude --task "Hello world"
```

Or symlink it:
```bash
ln -sf ~/code/coders/bin/coders ~/bin/coders
coders spawn claude --task "Hello world"
```

## Features

- **Interactive Sessions**: All spawned AIs stay in interactive mode for continuous communication
- **Git Worktrees**: Creates isolated branches for each task
- **PRD Priming**: Feeds context to the AI before it starts
- **Tmux Sessions**: Runs in separate tmux windows
- **Redis Heartbeat**: Session monitoring, pub/sub for inter-agent communication
- **Tmux Resurrect**: Snapshot/restore entire swarm
- **Recursive Loop**: Automatically execute tasks from todolist with promise-based coordination and smart tool switching

<img width="1505" height="1331" alt="Dashboard" src="https://github.com/user-attachments/assets/a9f46996-670c-4e13-975c-d8e381aaa0ab" />

### Communicating with Spawned Sessions

All sessions run in **interactive mode** and persist until you explicitly kill them.

**Attach directly (recommended):**
```bash
tmux attach -t coder-SESSION_ID
# Press Ctrl+B then D to detach without killing
```

**Send messages remotely:**
```bash
# Using helper script
./bin/send-to-session.sh coder-SESSION_ID "your message"

# Check response
tmux capture-pane -t coder-SESSION_ID -p | tail -20
```

**Why two-step for remote messaging:**
TUI applications (Gemini, Codex) require text and Enter to be sent separately:
```bash
tmux send-keys -t SESSION "message"
sleep 0.5  # Let TUI process input
tmux send-keys -t SESSION C-m  # Submit
```

### Redis Heartbeat & Monitoring

Enable Redis for heartbeat monitoring and inter-agent communication:

```typescript
await coders.spawn({
  tool: 'claude',
  task: 'Build auth module',
  redis: { url: 'redis://localhost:6379' },
  enableHeartbeat: true
});
```

This will:
- Publish heartbeats every 30s to Redis for dashboard monitoring
- Enable inter-agent pub/sub communication
- Clean up resources automatically when sessions end

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

- **tmux** - Required
- **Redis** - Required (for coordination/heartbeat)
- Claude Code CLI (`npm i -g @anthropic-ai/claude-code`) - optional
- Gemini CLI (`npm i -g @googlelabs/gemini-cli`) - optional
- OpenAI Codex CLI (`pip install openai-codex`) - optional
- OpenCode CLI (`npm i -g @opencode/ai/cli`) - optional

## Project Structure

```
coders/
├── .claude-plugin/
│   └── plugin.json        # Plugin manifest (Claude Code discovers this)
├── commands/              # Slash commands (auto-discovered)
│   ├── spawn.md
│   ├── list.md
│   ├── attach.md
│   ├── kill.md
│   ├── snapshot.md
│   └── restore.md
├── skills/
│   ├── assets/            # Runtime assets (dashboard, heartbeat)
│   └── coders/
│       ├── scripts/
│       │   ├── main.js          # CLI entry point
│       │   └── orchestrator.js  # Orchestrator state helpers
│       ├── SKILL.md       # Skill definition (required for discovery)
│       ├── coders.ts      # Claude Code skill (TypeScript, loaded directly)
│       ├── coders.d.ts    # Type definitions
│       ├── redis.ts       # Redis heartbeat & pub/sub
│       └── tmux-resurrect.ts # Snapshot/restore logic
├── bin/
│   ├── coders             # CLI wrapper
│   └── send-to-session.sh # Helper script
├── .gitignore
├── package.json
└── README.md
```

**Note:** No build step required! Claude Code loads `.ts` files directly.

## License

MIT
