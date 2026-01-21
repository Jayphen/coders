# Coders - Project Guide

This is a monorepo containing:
- `packages/plugin` - Claude Code plugin (distributed via git/npm)
- `packages/tui` - Terminal UI (distributed via npm)

## Deployment

### TUI (`@jayphen/coders-tui`)

**When to deploy:** Only when files in `packages/tui/` are changed.

```bash
cd packages/tui
pnpm build
npm version patch --no-git-tag-version
npm publish --access public --otp=<OTP>
git add -A && git commit -m "chore: release @jayphen/coders-tui vX.X.X" && git push
```

The TUI is lazy-installed by the plugin on first use from npm. Users get updates by clearing `~/.cache/coders-tui/`.

### Plugin (`@jayphen/coders`)

**When to deploy:** When files in `packages/plugin/` are changed.

The plugin is installed via Claude's plugin system from the git repo. For most changes, just push to git:

```bash
git add -A && git commit -m "feat: description" && git push
```

Users update via `/plugin:update coders`.

**If npm distribution is needed** (rare - only for users installing via npm):

```bash
cd packages/plugin
npm version patch --no-git-tag-version
# Also update .claude-plugin/plugin.json version to match!
cd ../..
git add -A && git commit -m "chore: release @jayphen/coders vX.X.X" && git push
cd packages/plugin
npm publish --access public --otp=<OTP>
```

### Version Sync

The plugin has two version files that must stay in sync:
- `packages/plugin/package.json`
- `packages/plugin/.claude-plugin/plugin.json`

Always update both when bumping versions.

## Development

```bash
pnpm install          # Install dependencies
pnpm dev:tui          # Run TUI in dev mode
pnpm plugin:test      # Test the plugin locally
```

## Testing Changes Locally

For plugin changes, you can test without publishing:
```bash
cd packages/plugin
node skills/coders/scripts/main.js <command>
```

For TUI changes:
```bash
cd packages/tui
pnpm dev
```
