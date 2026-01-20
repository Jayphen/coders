---
description: Open the Coders dashboard (start server if needed)
---

# Open the Coders dashboard

Execute:
```bash
node ${CLAUDE_PLUGIN_ROOT}/skills/coders/scripts/main.js dashboard
```
CLI alternative:
```bash
./bin/coders dashboard
```

Starts the dashboard server if it is not already running and opens it in your browser.

## Usage

```
/coders:dashboard
```

## Notes

- Respects `DASHBOARD_PORT` if set (default: 3030).
- Logs are written to your temp directory as `coders-dashboard.log`.
