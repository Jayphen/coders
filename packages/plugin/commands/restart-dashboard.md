---
description: Restart the Coders dashboard server
---

# Restart the Coders dashboard

Execute:
```bash
${CLAUDE_PLUGIN_ROOT}/bin/coders restart-dashboard
```

Stops the running dashboard server (if any) and starts a fresh instance, then opens it in your browser.

## Usage

```
/coders:restart-dashboard
```

## When to use

- After updating dashboard code (dashboard.html or dashboard-server.js)
- If the dashboard becomes unresponsive
- To clear any stale state in the server

## Notes

- Respects `DASHBOARD_PORT` if set (default: 3030).
- Logs are written to your temp directory as `coders-dashboard.log`.
- Safe to run even if no dashboard is currently running.
