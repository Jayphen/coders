# Coders - Project Guide

This is a monorepo containing:
- `packages/plugin` - Claude Code plugin (distributed via git/npm)
- `packages/go` - Go implementation (TUI, CLI tools, orchestrator)

## Deployment

### Go Binary (coders-tui)

The Go binary is built and distributed from `packages/go/`:

```bash
cd packages/go
make build        # Build the binary
make install      # Install to /usr/local/bin
```

Users can also install via the install script:
```bash
curl -fsSL https://raw.githubusercontent.com/Jayphen/coders/go-rewrite/packages/go/install.sh | bash
```

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
pnpm install          # Install plugin dependencies
pnpm plugin:test      # Test the plugin locally
```

### Go Development

```bash
cd packages/go
make build           # Build the binary
make test            # Run tests
./coders-tui --help  # Test the CLI
```

## Testing Changes Locally

### Plugin Changes
```bash
cd packages/plugin
node skills/coders/scripts/main.js <command>
```

### Go/TUI Changes
```bash
cd packages/go
go build -o coders-tui ./cmd/coders
./coders-tui spawn claude --task "test task"
./coders-tui tui  # Launch the TUI
```
