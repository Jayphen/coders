# Coder Spawner Skill

Spawn AI coding assistants (Claude, Gemini, Codex, OpenCode) in isolated tmux sessions with optional git worktrees.

## Functions

### coders.spawn(options)
Spawn a new AI coder session.

Options:
- `tool`: 'claude' | 'gemini' | 'codex' | 'opencode'
- `task`: Task description (used to auto-generate session name)
- `name`: Session name (auto-generated from task if omitted)
- `worktree`: Git branch for worktree
- `baseBranch`: Base branch for worktree (default: main)
- `prd`: PRD/spec file path

**Auto-generated Names:** Session names are automatically generated from the task description, extracting meaningful phrases to create readable identifiers (e.g., "Review the Linear project" → `coder-claude-linear-project`).

### coders.claude(task, options?)
Quick spawn for Claude.

### coders.gemini(task, options?)
Quick spawn for Gemini.

### coders.opencode(task, options?)
Quick spawn for OpenCode.

### coders.list()
List all active coder sessions.

### coders.attach(sessionName)
Attach to a session.

### coders.kill(sessionName)
Kill a session.

### coders.snapshot()
Save current sessions to disk.

### coders.restore()
Restore sessions from snapshot.

## Example

```typescript
import { coders } from '@jayphen/coders';

await coders.spawn({
  tool: 'claude',
  task: 'Refactor the authentication module',
  worktree: 'feature/auth-refactor',
  prd: 'docs/auth-prd.md'
});

coders.list();
coders.attach('session-name');
```
