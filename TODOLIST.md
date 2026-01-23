# Go Rewrite Todolist

Migration of TUI + CLI from TypeScript/Node.js to Go for improved performance and distribution.

**Branch:** `go-rewrite`
**Target:** Single unified `coders` binary with Bubbletea TUI

---

## Phase 1: Project Setup & Foundation

- [ ] Initialize Go module (`packages/go/`)
- [ ] Set up project structure (cmd/, internal/, pkg/)
- [ ] Add dependencies (bubbletea, bubbles, lipgloss, go-redis)
- [ ] Create Makefile for building multi-platform binaries
- [ ] Set up GitHub Actions for CI/CD

## Phase 2: Core Libraries

### Tmux Integration
- [ ] Port tmux session listing (`tmux list-sessions`)
- [ ] Port session attach/switch functionality
- [ ] Port session kill (including process tree cleanup)
- [ ] Port pane PID listing for heartbeat
- [ ] Add tests for tmux operations

### Redis Integration
- [ ] Set up go-redis client connection
- [ ] Port Redis SCAN for keys (promises, heartbeats)
- [ ] Port MGET for batch value retrieval
- [ ] Port promise read/write operations
- [ ] Port heartbeat read operations
- [ ] Add tests for Redis operations

### Types & Models
- [ ] Define Session struct
- [ ] Define HeartbeatData struct
- [ ] Define CoderPromise struct
- [ ] Define usage statistics types

## Phase 3: TUI Implementation

### Core Application
- [ ] Create main Bubbletea model
- [ ] Implement Init() with session fetch + ticker
- [ ] Implement Update() for keyboard handling
- [ ] Implement View() for rendering
- [ ] Add 5-second auto-refresh polling

### Components
- [ ] Header component (title + version)
- [ ] SessionList component (active/completed sections)
- [ ] SessionRow component (tool colors, status indicators)
- [ ] SessionDetail component (full session info panel)
- [ ] StatusBar component (help text, counts)
- [ ] Spawn dialog (text input modal)
- [ ] Kill confirmation dialog

### Styling (Lip Gloss)
- [ ] Define color palette (tool colors, status colors)
- [ ] Style borders and boxes
- [ ] Style selected/dimmed states
- [ ] Progress bar rendering
- [ ] Status indicators (●, ◐, ○, ✓, !, ?)

### Keyboard Handling
- [ ] Arrow keys + j/k navigation
- [ ] Enter/a to attach
- [ ] s to spawn
- [ ] K to kill selected
- [ ] C to kill all completed
- [ ] R to resume
- [ ] r to refresh
- [ ] q to quit

## Phase 4: CLI Commands

### Core Commands
- [ ] `coders spawn` - Spawn new session
- [ ] `coders list` - List all sessions
- [ ] `coders attach` - Attach to session
- [ ] `coders kill` - Kill session
- [ ] `coders promise` - Publish completion promise
- [ ] `coders resume` - Resume completed session
- [ ] `coders tui` - Launch TUI
- [ ] `coders version` - Show version

### Spawn Features
- [ ] Tool selection (claude, gemini, codex, opencode)
- [ ] Task description via --task
- [ ] Working directory via --cwd (with zoxide support)
- [ ] Model selection via --model
- [ ] Worktree creation via --worktree
- [ ] Context files via --context
- [ ] Heartbeat enabling
- [ ] Wait for CLI ready before returning

### List Features
- [ ] JSON output format
- [ ] Filter by status (active/completed)
- [ ] Show promises/heartbeat status

## Phase 5: Background Processes

### Heartbeat
- [ ] Port heartbeat script to Go
- [ ] Run as background goroutine or subprocess
- [ ] Write to Redis with session info

### Loop Runner (Optional)
- [ ] Port loop runner logic
- [ ] Monitor promises for completion
- [ ] Auto-spawn from todolist
- [ ] Tool switching on usage cap

## Phase 6: Integration & Testing

### Plugin Integration
- [ ] Update TypeScript skills to call Go binary
- [ ] Test /coders:spawn calls Go binary
- [ ] Test /coders:promise calls Go binary
- [ ] Ensure backward compatibility

### Testing
- [ ] Unit tests for tmux operations
- [ ] Unit tests for Redis operations
- [ ] Integration tests for TUI
- [ ] Manual testing on macOS
- [ ] Manual testing on Linux

### Documentation
- [ ] Update README with Go installation
- [ ] Update CLAUDE.md with new build process
- [ ] Add architecture documentation
- [ ] Release notes

## Phase 7: Distribution

- [ ] Build darwin/arm64 binary
- [ ] Build darwin/amd64 binary
- [ ] Build linux/amd64 binary
- [ ] Build linux/arm64 binary
- [ ] Create GitHub release workflow
- [ ] Add install script (curl | sh)
- [ ] Optional: Homebrew formula

---

## Deferred / Out of Scope

- [ ] AI name generator (keep in Node.js, call via exec if needed)
- [ ] tmux-resurrect integration (evaluate later)
- [ ] Windows support (low priority)

---

## Progress Tracking

| Phase | Status | Notes |
|-------|--------|-------|
| 1. Setup | Not Started | |
| 2. Core Libraries | Not Started | |
| 3. TUI | Not Started | |
| 4. CLI | Not Started | |
| 5. Background | Not Started | |
| 6. Integration | Not Started | |
| 7. Distribution | Not Started | |

---

## Reference

- Investigation doc: `docs/go-tui-investigation.md`
- Current TUI: `packages/tui/`
- Current CLI: `packages/plugin/skills/coders/scripts/main.js`
