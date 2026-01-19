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

Then import and use:

```typescript
import { coders } from '@jayphen/coders';

// Spawn Claude in new tmux window
await coders.spawn({
  tool: 'claude',
  task: 'Review this PR and suggest improvements',
  worktree: 'feature-review',
  prd: 'docs/prd.md'
});

// Quick helpers
await coders.claude('Fix the bug', { worktree: 'fix-auth' });
await coders.gemini('Research this approach');

coders.list();
coders.attach('session-name');
coders.kill('session-name');
```

## Features

- **Git Worktrees**: Creates isolated branches for each task
- **PRD Priming**: Feeds context to the AI before it starts
- **Tmux Sessions**: Runs in separate tmux windows
- **Recursive Spawning**: Claude instances can spawn more Claudes (`--dangerously-spawn-permission`)

## Requirements

- tmux installed
- Claude Code CLI (`npm i -g @anthropic-ai/claude-code`) - optional
- Gemini CLI (`npm i -g @googlelabs/gemini-cli`) - optional
- OpenAI Codex CLI (`pip install openai-codex`) - optional

## License

MIT
