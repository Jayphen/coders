# Coder Spawner Skill

Spawn AI coding assistants (Claude, Gemini, Codex) in isolated tmux sessions with optional git worktrees.

## Usage

```bash
# Spawn Claude Code with worktree
coders spawn claude --worktree feature/new-feature --prd docs/prd.md

# Spawn Gemini CLI with worktree
coders spawn gemini --worktree refactor/auth --spec src/auth/spec.md

# Spawn OpenAI Codex
coders spawn codex --prompt "Fix the login bug"

# List active sessions
coders list

# Attach to a session
coders attach <session-name>
```

## Features

- **Git Worktrees**: Creates isolated branches for each task
- **PRD/Spec Priming**: Feeds context to the AI before it starts
- **Tmux Sessions**: Runs in separate tmux windows (on Pi) or iTerm (on Mac)
- **Multi-Platform**: Detects tmux/iTerm/WezTerm automatically

## Requirements

- Claude CLI: `npm i -g @anthropic-ai/claude-code`
- Gemini CLI: `npm i -g @googlelabs/gemini-cli`
- OpenAI Codex: `pip install openai-codex`
- tmux (Linux) or iTerm2/WezTerm (Mac)
