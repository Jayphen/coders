# Go Rewrite Todolist

Migration of TUI + CLI from TypeScript/Node.js to Go for improved performance and distribution.

**Branch:** `go-rewrite`
**Target:** Single unified `coders` binary with Bubbletea TUI

---

## Phase 1: Project Setup & Foundation

- [x] Initialize Go module (`packages/go/`)
- [x] Set up project structure (cmd/, internal/, pkg/)
- [x] Add dependencies (bubbletea, bubbles, lipgloss, go-redis)
- [x] Create Makefile for building multi-platform binaries
- [ ] Set up GitHub Actions for CI/CD

## Phase 2: Core Libraries

### Tmux Integration
- [x] Port tmux session listing (`tmux list-sessions`)
- [x] Port session attach/switch functionality
- [x] Port session kill (including process tree cleanup)
- [x] Port pane PID listing for heartbeat
- [ ] Add tests for tmux operations

### Redis Integration
- [x] Set up go-redis client connection
- [x] Port Redis SCAN for keys (promises, heartbeats)
- [x] Port MGET for batch value retrieval
- [x] Port promise read/write operations
- [x] Port heartbeat read operations
- [ ] Add tests for Redis operations

### Types & Models
- [x] Define Session struct
- [x] Define HeartbeatData struct
- [x] Define CoderPromise struct
- [x] Define usage statistics types

## Phase 3: TUI Implementation

### Core Application
- [x] Create main Bubbletea model
- [x] Implement Init() with session fetch + ticker
- [x] Implement Update() for keyboard handling
- [x] Implement View() for rendering
- [x] Add 5-second auto-refresh polling

### Components
- [x] Header component (title + version)
- [x] SessionList component (active/completed sections)
- [x] SessionRow component (tool colors, status indicators)
- [x] SessionDetail component (full session info panel)
- [x] StatusBar component (help text, counts)
- [x] Spawn dialog (text input modal)
- [x] Kill confirmation dialog

### Styling (Lip Gloss)
- [x] Define color palette (tool colors, status colors)
- [x] Style borders and boxes
- [x] Style selected/dimmed states
- [x] Progress bar rendering
- [x] Status indicators (●, ◐, ○, ✓, !, ?)

### Keyboard Handling
- [x] Arrow keys + j/k navigation
- [x] Enter/a to attach
- [x] s to spawn
- [x] K to kill selected
- [x] C to kill all completed
- [x] R to resume
- [x] r to refresh
- [x] q to quit

## Phase 4: CLI Commands

### Core Commands
- [x] `coders spawn` - Spawn new session
- [x] `coders list` - List all sessions
- [x] `coders attach` - Attach to session
- [x] `coders kill` - Kill session
- [x] `coders promise` - Publish completion promise
- [x] `coders resume` - Resume completed session
- [x] `coders tui` - Launch TUI
- [x] `coders version` - Show version
- [x] `coders init` - Initialize orchestrator + TUI, attach to orchestrator
- [x] `coders orchestrator` - Start or attach to orchestrator session

### Spawn Features
- [x] Tool selection (claude, gemini, codex, opencode)
- [x] Task description via --task
- [x] Working directory via --cwd (with zoxide support)
- [x] Model selection via --model
- [ ] Worktree creation via --worktree
- [ ] Context files via --context
- [x] Heartbeat enabling (flag present, implementation pending)
- [x] Wait for CLI ready before returning

### List Features
- [x] JSON output format
- [x] Filter by status (active/completed)
- [x] Show promises/heartbeat status

## Phase 5: Background Processes

### Heartbeat
- [x] Port heartbeat script to Go
- [x] Run as background goroutine or subprocess
- [x] Write to Redis with session info
- [x] Parse usage stats from tmux pane (cost, tokens, API calls, limits)
- [x] Integrate with spawn command (--heartbeat flag)

### Loop Runner (Optional)
- [x] Port loop runner logic
- [x] Monitor promises for completion
- [x] Auto-spawn from todolist
- [x] Tool switching on usage cap

## Phase 6: Integration & Testing

### Plugin Integration
- [x] Update bin/coders script to prefer Go binary
- [x] Fall back to Node.js if Go binary not found
- [ ] Test /coders:spawn calls Go binary
- [ ] Test /coders:promise calls Go binary
- [x] Ensure backward compatibility (Node.js fallback)

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

- [x] Build darwin/arm64 binary (~7MB)
- [x] Build darwin/amd64 binary (~7.4MB)
- [x] Build linux/amd64 binary (~7.3MB)
- [x] Build linux/arm64 binary (~6.9MB)
- [x] Create GitHub release workflow (.github/workflows/release.yml)
- [x] Add install script (curl | sh)
- [ ] Optional: Homebrew formula

---

## Phase 8: TUI Performance Optimizations

### High Priority

- [x] **Cache preview line splitting** (`views.go:321-349`)
  - `tailLines()` and `truncateLines()` split entire preview string on every render
  - Store last preview content hash and split result, only re-split when content changes
  - Critical: preview can be thousands of lines

- [x] **Pre-cache lipgloss styles** (`views.go:204-310`)
  - `renderSessionRow()` creates new style objects for each row on every render
  - Move to package-level cached styles in `styles.go` (e.g., `OrchestratorNameStyle`, `CompletedNameStyle`, `ActiveNameStyle`)

- [x] **Non-blocking Redis client init** (`model.go:478-492`)
  - Redis client created lazily during `fetchSessions()` which can block UI with 2-second timeout
  - Move initialization to `NewModel()` with non-blocking error handling

- [x] **Optimize session sorting** (`model.go:526-542`)
  - Full O(n log n) sort every 5 seconds even when order hasn't changed
  - Cache session order signature, only re-sort on session add/remove
  - Use stable sort key instead of complex conditional comparisons

- [x] **Fix status bar string concatenation** (`views.go:599-601`)
  - Uses `+=` string concatenation instead of `strings.Builder`

### Medium Priority

- [x] **Pre-allocate command slices** (`model.go:187-197`)
  - `cmds` slice grows dynamically with repeated appends
  - Pre-allocate with capacity (max ~3 commands)

- [x] **Cache ANSI-aware widths** (`views.go:314-318`)
  - `lipgloss.Width()` called repeatedly per render, parses ANSI codes each time
  - Cache widths during initial render pass, reuse in `padRight()`

- [x] **Fix progress bar string concat** (`styles.go:142-148`)
  - Loop concatenation for filled/empty sections
  - Use `strings.Repeat()` instead

### Lower Priority

- [x] **Implement dirty tracking for View()** (`model.go:356-414`)
  - Currently rebuilds entire view on every message
  - Only re-render changed sections (selection, session list, preview)

- [x] **Pre-allocate spawn args slice** (`model.go:602-642`)
  - Unbounded slice growth during argument parsing
  - Estimate count from spaces, pre-allocate

- [x] **Optimize process tree scanning** (`tmux/tmux.go:201-247`)
  - Full `ps` output scan in `killSessionProcessTree()`
  - Consider platform-specific APIs or caching

---

## Deferred / Out of Scope

- [ ] AI name generator (keep in Node.js, call via exec if needed)
- [ ] tmux-resurrect integration (evaluate later)
- [ ] Windows support (low priority)

---

## Progress Tracking

| Phase | Status | Notes |
|-------|--------|-------|
| 1. Setup | ✅ Complete | Module, deps, Makefile done |
| 2. Core Libraries | ✅ Complete | Tmux, Redis, Types implemented |
| 3. TUI | ✅ Complete | All components, keyboard, styling |
| 4. CLI | ✅ Complete | All 8 commands implemented |
| 5. Background | ✅ Complete | Heartbeat implemented |
| 6. Integration | ✅ Complete | bin/coders prefers Go binary |
| 7. Distribution | ✅ Complete | GitHub Actions, install script |
| 8. Performance | 🔲 Not Started | 11 optimization items identified |

---

## Reference

- Investigation doc: `docs/go-tui-investigation.md`
- Current TUI: `packages/tui/`
- Current CLI: `packages/plugin/skills/coders/scripts/main.js`
