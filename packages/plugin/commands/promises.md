---
description: Check completion promises from all coder sessions
---

# Check Completion Promises

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js promises
```

Check the completion status of all spawned coder sessions. Shows which sessions have finished their tasks, are blocked, or need review.

## Usage

```
/coders:promises
```

Alternative command name:
```
/coders:check-promises
```

## What It Shows

For each session that has published a promise:
- ✅ **Completed** sessions with summary of work done
- 🚫 **Blocked** sessions waiting on something
- 👀 **Needs Review** sessions ready for human review
- Time since completion
- Optional: Files changed, blocker reasons

## Output Example

```
📋 Completion Promises (3):

✅ solar-prd
   Implemented all PRD features: PDF export, webhooks, analytics, SEO pages
   Status: completed • 5m ago
   Files: src/export.ts, src/webhooks.ts, src/analytics.ts...

🚫 backyard-prd
   Blocked waiting on API credentials from ops team
   Status: blocked • 2m ago
   Blockers: Need Stripe API keys

👀 auth-feature
   Authentication system ready for code review
   Status: needs-review • 1h ago
```

## Use Cases

**From orchestrator:**
```bash
# Spawn sessions
coders spawn claude --task "Build feature A" --name feature-a
coders spawn claude --task "Build feature B" --name feature-b

# Check progress
coders promises

# Kill completed sessions
coders kill feature-a
```

**Integration with workflows:**
- Check before spawning follow-up tasks
- Identify blocked sessions needing attention
- See what's ready for review/merge
- Clean up completed sessions

## Notes

- Promises are stored in Redis with 24h TTL
- Sessions must explicitly publish promises using `/coders:promise`
- If no promises are found, sessions may still be working or didn't publish
- Use `/coders:list` to see all active sessions (running or completed)
