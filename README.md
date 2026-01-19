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

## Requirements

- tmux installed
- Claude Code CLI (`npm i -g @anthropic-ai/claude-code`) - optional
- Gemini CLI (`npm i -g @googlelabs/gemini-cli`) - optional
- OpenAI Codex CLI (`pip install openai-codex`) - optional

## License

MIT
