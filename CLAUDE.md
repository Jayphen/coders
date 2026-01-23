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

### Plugin Development

```bash
pnpm install          # Install plugin dependencies
pnpm plugin:test      # Test the plugin locally
```

### Go Development

All Go development happens in `packages/go/`. The Makefile provides comprehensive build and development targets:

```bash
cd packages/go

# Building
make build           # Build for current platform (output: bin/coders)
make build-all       # Build for all platforms (darwin/linux, amd64/arm64)
make clean           # Remove build artifacts

# Testing
make test            # Run all tests
make test-coverage   # Run tests with coverage report (generates coverage.html)

# Code Quality
make fmt             # Format code with gofmt
make lint            # Run golangci-lint
make tidy            # Tidy Go module dependencies

# Running
make run             # Build and run the TUI
make list            # Build and run the list command
./bin/coders --help  # Run the built binary directly

# Development Workflow
make watch           # Auto-rebuild on file changes (requires watchexec)
make install         # Install to $GOPATH/bin/coders

# Help
make help            # Show all available targets
```

**Development workflow:**
1. Make changes to Go code
2. Run `make test` to verify tests pass
3. Run `make fmt` to format code
4. Run `make run` or `./bin/coders tui` to test the TUI
5. Run `make install` to install locally for system-wide testing

## Testing Changes Locally

### Plugin Changes
```bash
cd packages/plugin
node skills/coders/scripts/main.js <command>
```

### Go/TUI Changes
```bash
cd packages/go

# Quick iteration cycle
make build && ./bin/coders spawn claude --task "test task"
make build && ./bin/coders tui  # Launch the TUI
make build && ./bin/coders list # List all sessions

# Or use convenience targets
make run   # Builds and runs TUI
make list  # Builds and runs list command
```

**Build artifacts:**
- Binary output: `packages/go/bin/coders`
- Test coverage: `packages/go/coverage.html`
- Cross-platform builds: `packages/go/bin/coders-{os}-{arch}`
