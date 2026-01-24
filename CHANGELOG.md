# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-01-24

### 🚀 Complete Go Rewrite

This is a major release that rewrites the core CLI from TypeScript/Node.js to Go, delivering substantial performance improvements and better user experience.

### Added

#### Core Architecture
- **Native Go binary** - Complete rewrite of the CLI in Go, eliminating Node.js runtime dependency
- **Single binary distribution** - Self-contained executable with no external dependencies
- **Cross-platform support** - Pre-built binaries for Darwin/Linux on AMD64/ARM64 architectures
- **GitHub Actions release workflow** - Automated builds and releases
- **Install script** - One-line installation via curl

#### CLI Commands (All Ported to Go)
- `coders tui` - Interactive terminal UI built with Bubbletea
- `coders spawn` - Spawn new coding sessions
- `coders list` - List all sessions with JSON output support
- `coders attach` - Attach to running sessions
- `coders kill` - Terminate sessions
- `coders promise` - Publish completion promises
- `coders resume` - Resume completed sessions
- `coders orchestrator` - Manage orchestrator session
- `coders init` - Initialize coders in a project
- `coders heartbeat` - Session monitoring and health checks
- `coders version` - Display version information

#### New Features
- **Loop runner** (`/coders:loop`) - Automatically spawn tasks from todolist with recursive execution
- **Ollama backend support** - Run sessions using Ollama with `--ollama` flag
- **Configuration file support** - YAML configuration for customization
- **Structured logging** - Zerolog integration for better debugging
- **Automatic crash recovery** - Sessions auto-restart on error
- **Live preview panel** - Two-way communication in TUI with real-time session output
- **Health check system** - Monitor session liveness and status

#### Enhanced TUI
- **Interactive navigation** - Vim-style keybindings (j/k) and arrow keys
- **Session management** - Spawn, attach, kill, resume directly from TUI
- **Status visualization** - Real-time status updates with color-coded indicators
- **Bulk operations** - Kill all completed sessions with `C` key
- **Smart quit behavior** - Auto-switch to orchestrator when quitting TUI
- **Session filtering** - Filter by status (active/completed/error)

### Performance Improvements

#### Startup Performance
- **~20x faster startup** - Go binary starts in ~2ms vs Node.js ~40ms
- **Instant command response** - No runtime initialization overhead

#### TUI Optimizations
- **Dirty tracking** - Only re-render changed components
- **Style pre-caching** - Lipgloss styles cached to avoid recomputation
- **String builder usage** - Efficient string concatenation for progress bars and status
- **ANSI-aware width caching** - Cached calculations for terminal width handling
- **Pre-calculated sort keys** - Optimized session list sorting
- **Non-blocking Redis** - Async Redis client initialization
- **Spawn args pre-allocation** - Reduced memory allocations

#### Process Management
- **Optimized liveness checking** - Faster process detection with combined checks
- **Faster polling** - Improved spawn CLI detection

### Changed
- **Removed TypeScript TUI** - Replaced with faster Go implementation
- **Binary distribution model** - Plugin now bundles pre-built Go binaries for all platforms
- **Build process** - Makefile-based build system with comprehensive targets

### Technical Details

#### Project Structure
```
packages/
├── go/                    # Go CLI implementation
│   ├── cmd/coders/       # CLI entry points
│   ├── internal/         # Internal packages
│   │   ├── tui/         # Bubbletea TUI
│   │   ├── tmux/        # Tmux integration
│   │   ├── redis/       # Redis client
│   │   └── types/       # Shared types
│   └── Makefile         # Build system
└── plugin/               # Claude Code plugin (TypeScript)
    └── bin/             # Pre-built Go binaries
```

#### Development Workflow
- `make build` - Build for current platform
- `make build-all` - Cross-compile for all platforms
- `make test` - Run test suite with coverage
- `make watch` - Auto-rebuild on changes
- `make lint` - Code quality checks

### Migration Notes

#### For Users
- **No breaking changes to commands** - All existing commands work identically
- **Automatic binary selection** - Plugin automatically uses correct binary for your platform
- **Update via plugin system** - Run `/plugin:update coders` to get the latest version

#### For Developers
- **Go 1.21+ required** - For building from source
- **Makefile build targets** - See `make help` for all available commands
- **Cross-compilation** - Build for all platforms with single command

### Dependencies
- Go standard library
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Zerolog](https://github.com/rs/zerolog) - Structured logging
- Redis client (go-redis)

---

## [1.3.0] - 2025-01-15

### Added
- Promise system for tracking coder session completion
- `/coders:promises` command to check completion status
- Dashboard improvements with session sorting and textarea input
- Claude Code usage statistics in TUI

### Changed
- Sessions now automatically report completion promises
- Dashboard sorts sessions by creation time

---

## [1.2.0] - 2024-12-20

### Added
- Hybrid `--system-prompt` support for better customization
- Comprehensive test coverage for prompt handling

---

## [1.1.0] - 2024-12-15

### Added
- Initial release of TypeScript-based CLI
- Basic TUI implementation
- Session spawning and management
- Redis integration for state management
- Multi-agent support (Claude, Gemini, Codex, OpenCode)

[2.0.0]: https://github.com/Jayphen/coders/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/Jayphen/coders/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/Jayphen/coders/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Jayphen/coders/releases/tag/v1.1.0
