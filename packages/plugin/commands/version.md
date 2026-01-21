---
description: Check coders plugin version and available updates
---

# Check coders plugin version

Display the current plugin version and check GitHub for updates.

## Steps

1. Read the current version from the local plugin.json:
```bash
cat ${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json | grep '"version"'
```

2. Fetch the latest version from GitHub using WebFetch:
   - URL: `https://raw.githubusercontent.com/Jayphen/coders/master/packages/plugin/.claude-plugin/plugin.json`
   - Extract the version field from the response

3. Compare versions and display:

| Component | Version |
|-----------|---------|
| Installed | (from local plugin.json) |
| Latest    | (from GitHub) |

4. Show status:
   - If versions match: `✓ Up to date`
   - If update available: `⚠️ Update available`

5. If an update is available, show:
```
To update, run: /plugin update coders
```

## Usage

```
/coders:version
```
