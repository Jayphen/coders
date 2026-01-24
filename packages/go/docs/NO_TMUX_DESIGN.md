# PTY Session Management Without tmux: Design Document

**Author:** Research Coder
**Date:** 2026-01-25
**Status:** Design Proposal

## Executive Summary

This document proposes replacing the current tmux-based session management with direct PTY management in Go. The new architecture would provide native control over terminal sessions while maintaining the same user experience, with potential improvements in latency, complexity, and portability.

**Key Recommendation:** Use `github.com/creack/pty` for PTY management with a custom session manager that embeds terminal views directly into the Bubbletea TUI.

---

## Table of Contents

1. [Current tmux Approach](#current-tmux-approach)
2. [Go PTY Library Evaluation](#go-pty-library-evaluation)
3. [Terminal Multiplexer Architecture Analysis](#terminal-multiplexer-architecture-analysis)
4. [Bubbletea Terminal Embedding](#bubbletea-terminal-embedding)
5. [Recommended Architecture](#recommended-architecture)
6. [Keystroke Forwarding Strategy](#keystroke-forwarding-strategy)
7. [Session Persistence Approach](#session-persistence-approach)
8. [Risks and Tradeoffs](#risks-and-tradeoffs)
9. [Implementation Roadmap](#implementation-roadmap)
10. [References](#references)

---

## Current tmux Approach

### How It Works

The current implementation (`packages/go/internal/tmux/tmux.go`) uses tmux as an external dependency:

1. **Session Creation:** Creates detached tmux sessions via `tmux new-session -d`
2. **Session Management:** Lists sessions via `tmux list-sessions` with custom formatting
3. **Output Capture:** Reads terminal output via `tmux capture-pane`
4. **Input Injection:** Sends commands via `tmux send-keys`
5. **Attachment:** Switches/attaches to sessions for interactive use
6. **Persistence:** Relies on tmux's built-in session persistence

### Advantages

- **Zero Implementation:** tmux handles all PTY complexity
- **Battle-Tested:** Extremely stable, used by millions
- **Rich Features:** Window splitting, copy mode, status bars, etc.
- **Session Resurrection:** Built-in session persistence across SSH disconnects
- **Process Management:** Handles process trees, signal forwarding, cleanup

### Disadvantages

- **External Dependency:** Requires tmux installation (though nearly universal on Unix)
- **Command Overhead:** Each operation requires spawning a tmux subprocess
- **Latency:** ~5-15ms per tmux command invocation
- **Indirection:** Cannot directly control PTY behavior
- **Complexity:** tmux is a full terminal multiplexer with features we don't use
- **Debugging:** Harder to debug issues in the tmux layer
- **Portability:** tmux is Unix-only (no Windows support, even with WSL complications)

---

## Go PTY Library Evaluation

### 1. github.com/creack/pty (RECOMMENDED)

**Status:** Industry standard, 2.1k stars, actively maintained
**Last Updated:** 2026 releases confirmed

#### Features
- Clean, platform-independent API
- Full support for PTY sizing (`StartWithSize`, `Getsize`, `InheritSize`)
- Signal handling for window resize (`SIGWINCH`)
- Works on Linux, macOS, FreeBSD, OpenBSD
- Minimal dependencies
- Well-tested in production (used by Docker, Kubernetes ecosystem)

#### API Overview
```go
// Start a command with PTY
cmd := exec.Command("bash")
ptmx, err := pty.Start(cmd)

// Get/set terminal size
ws, err := pty.GetsizeFull(ptmx)
err = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

// Handle window resize
ch := make(chan os.Signal, 1)
signal.Notify(ch, syscall.SIGWINCH)
go func() {
    for range ch {
        pty.InheritSize(os.Stdin, ptmx)
    }
}()
```

#### Pros
- De facto standard in Go ecosystem
- Simple, focused API
- Excellent cross-platform support
- Active maintenance
- Used in terminal emulator projects

#### Cons
- No built-in session management (we need to build it)
- No session persistence (need custom solution)
- No ConPty support for Windows (though this is rarely needed for our use case)

**Verdict:** ✅ **Recommended** - Best balance of simplicity, stability, and community support.

---

### 2. github.com/google/goterm

**Status:** Experimental, less actively maintained

#### Features
- PTY creation and termios management
- Color support utilities
- SSH ↔ termios translation helpers
- Linux-specific

#### Pros
- From Google, well-engineered
- Additional terminal utilities bundled

#### Cons
- Linux-only (no macOS support without modifications)
- Less community adoption than creack/pty
- More complex API for our needs
- Appears to be internal tool that was open-sourced

**Verdict:** ❌ Not recommended due to Linux-only limitation.

---

### 3. github.com/aymanbagabas/go-pty (wideeyedreven fork)

**Status:** Fork with Windows ConPty support

#### Features
- Cross-platform (Unix + Windows via ConPty)
- Compatible API with creack/pty
- Windows pseudo-console support

#### Pros
- True Windows support via ConPty
- Maintains creack/pty API compatibility
- Active development for cross-platform needs

#### Cons
- Smaller community
- Less battle-tested than creack/pty
- Additional complexity for Windows support we may not need

**Verdict:** ⚠️ Consider if Windows support becomes a requirement, otherwise stick with creack/pty.

---

## Terminal Multiplexer Architecture Analysis

### How tmux Manages PTYs

Based on research into tmux internals and architecture:

1. **Client-Server Model:**
   - Single server process manages all sessions
   - Clients connect via Unix socket (`/tmp/tmux-UID/default`)
   - Server persists even when all clients disconnect

2. **Session Hierarchy:**
   - Session → Windows → Panes
   - Each pane = separate PTY (pseudo-terminal)
   - Sessions are independent collections of PTYs

3. **PTY Management:**
   - Server creates PTY pairs (master/slave)
   - Forks processes attached to slave PTY
   - Reads from master PTY, buffers output
   - Clients read buffered output for display

4. **Persistence Mechanism:**
   - Sessions remain alive in server process
   - Detachment doesn't kill processes
   - History buffer survives disconnection
   - Can attach from different terminals/hosts

5. **Output Handling:**
   - Server maintains scrollback buffer per pane
   - Clients receive incremental updates
   - Copy mode allows navigating buffer

### Key Insights for Our Design

1. **Buffering is Essential:** Must maintain scrollback buffer per session
2. **Process Lifecycle:** PTY process outlives UI attachment
3. **State Management:** Need to track session metadata (creation time, task, tool, etc.)
4. **Signal Forwarding:** Window resize signals must propagate to PTY

---

## Bubbletea Terminal Embedding

### Bubbletea + PTY Integration

Bubbletea is a pure TUI framework and doesn't natively "embed" a live terminal view. However, there are proven patterns:

#### Pattern 1: Output Polling + Display (RECOMMENDED)

**Approach:** Periodically read from PTY, store in buffer, render in Bubbletea view.

```go
type SessionModel struct {
    pty        *os.File
    output     []byte
    scrollback []string
}

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        // Read PTY output
        buf := make([]byte, 4096)
        n, _ := m.pty.Read(buf)
        if n > 0 {
            m.output = append(m.output, buf[:n]...)
            m.parseLines()
        }
        return m, tick()
    }
}

func (m *Model) View() string {
    // Render last N lines of scrollback
    return strings.Join(m.scrollback[max(0, len(m.scrollback)-30):], "\n")
}
```

**Pros:**
- Simple to implement
- Bubbletea handles rendering
- Full control over display

**Cons:**
- Not real-time (polling latency)
- Must parse ANSI escape codes for accurate rendering

---

#### Pattern 2: Raw PTY Pass-Through (for attach mode)

**Approach:** When attaching, exit Bubbletea and give direct PTY control to user.

```go
func attachToSession(pty *os.File) {
    // Put terminal in raw mode
    oldState, _ := term.MakeRaw(int(os.Stdin.Fd()))
    defer term.Restore(int(os.Stdin.Fd()), oldState)

    // Bidirectional copy
    go io.Copy(pty, os.Stdin)
    io.Copy(os.Stdout, pty)
}
```

**Pros:**
- Zero latency (direct terminal I/O)
- Full terminal emulation by user's terminal
- Standard attach behavior

**Cons:**
- Exits TUI (can't split view TUI + PTY)
- Same as tmux attach (not truly embedded)

---

### Charmbracelet Wish (SSH Server Example)

Wish demonstrates Bubbletea + PTY integration for SSH servers:

```go
// From wish/examples/bubbletea/main.go pattern
s := ssh.NewServer(
    ssh.WithHostKeys(keys...),
    wish.WithMiddleware(
        bubbletea.Middleware(makeApp),
        activeterm.Middleware(),  // PTY support
    ),
)
```

**Key Lesson:** The `activeterm.Middleware()` provides PTY capabilities to SSH sessions, and Bubbletea apps receive PTY info via `WithEnvironment()`.

**For Our Use Case:** This pattern confirms Bubbletea can receive PTY environment, but doesn't solve the "embed a terminal view inside Bubbletea" problem (Wish is about hosting Bubbletea over SSH, not embedding PTYs).

---

### Recommendation: Hybrid Approach

1. **TUI List View:** Use Bubbletea to display session list (current behavior)
2. **Preview Pane:** Poll PTY output, render last N lines in Bubbletea
3. **Full Attach:** Exit TUI, give raw PTY control (current tmux attach behavior)
4. **Send Commands:** Write directly to PTY master (replacing tmux send-keys)

---

## Recommended Architecture

### Component Overview

```
┌─────────────────────────────────────────────────┐
│           Bubbletea TUI (model.go)              │
│  ┌───────────────────┬───────────────────────┐  │
│  │  Session List     │   Preview Pane        │  │
│  │  - Active         │   - Last 30 lines     │  │
│  │  - Completed      │   - Polls every 500ms │  │
│  └───────────────────┴───────────────────────┘  │
└──────────────────────┬──────────────────────────┘
                       │
                       ▼
         ┌─────────────────────────┐
         │   Session Manager       │
         │   (sessionmgr/manager.go)│
         ├─────────────────────────┤
         │ - CreateSession()       │
         │ - ListSessions()        │
         │ - AttachSession()       │
         │ - SendKeys()            │
         │ - CaptureOutput()       │
         │ - KillSession()         │
         └────────┬────────────────┘
                  │
         ┌────────┴────────┐
         │                 │
         ▼                 ▼
    ┌─────────┐      ┌──────────────┐
    │ PTY Mgr │      │ Persistence  │
    │ (pty.go)│      │ (state.json) │
    └─────────┘      └──────────────┘
         │
    ┌────┴─────┐
    │          │
    ▼          ▼
  [claude]  [gemini]  (spawned processes)
```

### Session Manager Interface

```go
// packages/go/internal/sessionmgr/manager.go
package sessionmgr

type Manager struct {
    sessions map[string]*Session
    stateDir string
    mu       sync.RWMutex
}

type Session struct {
    ID          string
    Name        string
    Tool        string
    Task        string
    Cwd         string
    CreatedAt   time.Time

    // PTY components
    pty         *os.File
    cmd         *exec.Cmd
    output      *OutputBuffer

    // Metadata
    metadata    map[string]string
}

// Core operations
func (m *Manager) CreateSession(name, cwd, command string) (*Session, error)
func (m *Manager) GetSession(id string) (*Session, error)
func (m *Manager) ListSessions() ([]*Session, error)
func (m *Manager) KillSession(id string) error
func (m *Manager) SendKeys(id, keys string) error
func (m *Manager) CaptureOutput(id string, lines int) (string, error)
func (m *Manager) AttachSession(id string) error

// Persistence
func (m *Manager) SaveState() error
func (m *Manager) LoadState() error
```

### Output Buffer Design

```go
// packages/go/internal/sessionmgr/buffer.go
type OutputBuffer struct {
    lines      []string
    maxLines   int
    mu         sync.RWMutex
    parser     *ansi.Parser  // For ANSI escape code handling
}

func (b *OutputBuffer) Append(data []byte) {
    b.mu.Lock()
    defer b.mu.Unlock()

    // Parse ANSI, split into lines
    parsed := b.parser.Parse(data)
    newLines := strings.Split(string(parsed), "\n")

    b.lines = append(b.lines, newLines...)

    // Trim to maxLines
    if len(b.lines) > b.maxLines {
        b.lines = b.lines[len(b.lines)-b.maxLines:]
    }
}

func (b *OutputBuffer) GetLines(n int) []string {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if n > len(b.lines) {
        n = len(b.lines)
    }
    return b.lines[len(b.lines)-n:]
}
```

### PTY Lifecycle

```go
// packages/go/internal/sessionmgr/pty.go
func (m *Manager) createPTY(command string, cwd string) (*Session, error) {
    cmd := exec.Command("bash", "-c", command)
    cmd.Dir = cwd

    // Start with PTY
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return nil, err
    }

    // Set initial size
    pty.Setsize(ptmx, &pty.Winsize{
        Rows: 24,
        Cols: 80,
    })

    session := &Session{
        ID:        generateID(),
        pty:       ptmx,
        cmd:       cmd,
        output:    NewOutputBuffer(1000),
        CreatedAt: time.Now(),
    }

    // Start output reader
    go m.readPTYOutput(session)

    // Monitor process
    go m.monitorProcess(session)

    return session, nil
}

func (m *Manager) readPTYOutput(s *Session) {
    buf := make([]byte, 4096)
    for {
        n, err := s.pty.Read(buf)
        if err != nil {
            if err != io.EOF {
                log.Printf("PTY read error: %v", err)
            }
            return
        }
        if n > 0 {
            s.output.Append(buf[:n])
        }
    }
}

func (m *Manager) monitorProcess(s *Session) {
    err := s.cmd.Wait()

    m.mu.Lock()
    defer m.mu.Unlock()

    s.exitCode = s.cmd.ProcessState.ExitCode()
    s.exitedAt = time.Now()
    s.exitError = err

    // Mark session as completed
    m.markCompleted(s.ID)
}
```

---

## Keystroke Forwarding Strategy

### Current tmux Approach

```bash
tmux send-keys -l -t session-name "command"
tmux send-keys -t session-name Enter
```

**Latency:** ~10-15ms (subprocess spawn + IPC + tmux processing)

### Direct PTY Approach

```go
func (m *Manager) SendKeys(id, text string) error {
    s, err := m.GetSession(id)
    if err != nil {
        return err
    }

    // Write directly to PTY master
    _, err = s.pty.Write([]byte(text + "\n"))
    return err
}
```

**Latency:** <1ms (direct write to PTY master file descriptor)

### Zero-Latency Considerations

1. **No Polling:** Direct write to PTY FD, kernel handles delivery to slave
2. **No Buffering:** Write goes directly to PTY buffer
3. **No IPC:** No inter-process communication overhead
4. **Kernel Scheduling:** Only latency is kernel task scheduling (~microseconds)

### Interactive Attach Mode

For full interactivity (like `tmux attach`), use raw terminal mode:

```go
func (m *Manager) AttachSession(id string) error {
    s, err := m.GetSession(id)
    if err != nil {
        return err
    }

    // Put stdin in raw mode
    oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
    if err != nil {
        return err
    }
    defer term.Restore(int(os.Stdin.Fd()), oldState)

    // Handle window resize
    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGWINCH)
    defer signal.Stop(ch)

    go func() {
        for range ch {
            ws, _ := term.GetSize(int(os.Stdin.Fd()))
            pty.Setsize(s.pty, &pty.Winsize{
                Rows: uint16(ws.Height),
                Cols: uint16(ws.Width),
            })
        }
    }()

    // Trigger initial resize
    ch <- syscall.SIGWINCH

    // Bidirectional copy
    done := make(chan error, 2)

    go func() {
        _, err := io.Copy(s.pty, os.Stdin)
        done <- err
    }()

    go func() {
        _, err := io.Copy(os.Stdout, s.pty)
        done <- err
    }()

    // Wait for first error or completion
    return <-done
}
```

**Latency:** Zero additional latency beyond kernel I/O (same as tmux attach).

---

## Session Persistence Approach

### Challenge

PTYs are kernel-level resources tied to processes. When a process exits, the PTY is destroyed. Unlike tmux (which keeps processes alive in a server), we need a persistence strategy.

### Option 1: No True Persistence (SIMPLE)

**Approach:** Sessions die when TUI exits. Metadata persists in JSON.

```json
// ~/.coders/sessions.json
{
  "sessions": [
    {
      "id": "coder-claude-abc123",
      "name": "coder-claude-fix-bug",
      "tool": "claude",
      "task": "Fix authentication bug",
      "cwd": "/Users/me/project",
      "created_at": "2026-01-25T10:00:00Z",
      "exited_at": "2026-01-25T10:15:00Z",
      "exit_code": 0,
      "last_output": "Task completed successfully"
    }
  ]
}
```

**Pros:**
- Simple implementation
- No background daemon required
- Session metadata preserved for history

**Cons:**
- Can't resume sessions after TUI exit
- Processes terminate with TUI

**Use Case:** Suitable if sessions are always monitored via TUI.

---

### Option 2: Daemon-Based Persistence (TMUX-LIKE)

**Approach:** Run a background daemon that owns PTYs. TUI is just a client.

```
┌─────────────┐         ┌──────────────────┐
│ coders tui  │ ◄─────► │ coders daemon    │
│  (client)   │  gRPC   │  (PTY owner)     │
└─────────────┘         └────────┬─────────┘
                                 │
                        ┌────────┴────────┐
                        │  PTY Sessions   │
                        │  - claude       │
                        │  - gemini       │
                        └─────────────────┘
```

**Implementation:**
- Daemon runs as long-lived process (systemd/launchd service)
- TUI connects via Unix socket or gRPC
- Daemon manages PTY lifecycle
- Sessions persist across TUI restarts

**Pros:**
- True session persistence (like tmux)
- Sessions survive TUI crashes
- Can attach from multiple TUIs

**Cons:**
- Significant complexity (daemon management, IPC, state sync)
- Need to handle daemon crashes/restarts
- Platform-specific daemon setup (systemd vs launchd)

**Use Case:** If tmux-like persistence is required.

---

### Option 3: Detached Process + Snapshot (HYBRID)

**Approach:** Sessions run as detached processes. Capture snapshots periodically.

```go
func (m *Manager) CreateSession(...) (*Session, error) {
    // Start process in separate process group
    cmd := exec.Command("bash", "-c", command)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,  // Detach from TUI process group
    }

    ptmx, err := pty.Start(cmd)
    // ... rest of setup

    // Save PID and PTY path for recovery
    m.savePersistentMetadata(session)
}

// Later recovery
func (m *Manager) RecoverSessions() error {
    metadata := m.loadPersistentMetadata()

    for _, meta := range metadata {
        // Check if process still alive
        if processExists(meta.PID) {
            // Reattach to PTY (implementation-specific)
            // Note: This is complex and may require /dev/pts manipulation
        }
    }
}
```

**Pros:**
- Sessions survive TUI exit
- Simpler than full daemon

**Cons:**
- PTY reattachment is complex (may need /proc/PID/fd tricks)
- Not guaranteed to work across all platforms
- Output history lost (can only capture from reattachment point)

**Use Case:** Experimental, not recommended.

---

### Option 4: CRIU Checkpoint/Restore (ADVANCED)

**Approach:** Use CRIU (Checkpoint/Restore In Userspace) to snapshot PTY sessions.

**CRIU Features:**
- Freeze running process, serialize to disk
- Restore process from snapshot
- Supports PTYs, terminals, process groups

**Go Bindings:** `github.com/checkpoint-restore/go-criu`

**Example:**
```go
import "github.com/checkpoint-restore/go-criu"

// Checkpoint a session
func (m *Manager) CheckpointSession(id string) error {
    s, _ := m.GetSession(id)

    c := criu.MakeCriu()
    opts := &criu.CriuOpts{
        ImagesDirFd: proto.Int32(int32(imgDir.Fd())),
        LogLevel:    proto.Int32(4),
        LogFile:     proto.String("dump.log"),
        Pid:         proto.Int32(int32(s.cmd.Process.Pid)),
        ShellJob:    proto.Bool(true),  // For terminal jobs
    }

    return c.Dump(opts, nil)
}

// Restore a session
func (m *Manager) RestoreSession(id string) error {
    c := criu.MakeCriu()
    opts := &criu.CriuOpts{
        ImagesDirFd: proto.Int32(int32(imgDir.Fd())),
        LogLevel:    proto.Int32(4),
        LogFile:     proto.String("restore.log"),
    }

    return c.Restore(opts, nil)
}
```

**Pros:**
- True process hibernation
- Preserves full PTY state (output buffer, cursor position, etc.)
- Industry-proven (Docker checkpoint/restore)

**Cons:**
- **Complex:** CRIU setup, kernel requirements
- **Linux-only:** No macOS support
- **Heavyweight:** Requires privileged operations
- **Overkill:** For our use case

**Use Case:** Not recommended unless advanced session migration is needed.

---

### RECOMMENDATION: Option 1 (No True Persistence)

For the initial implementation:
- Use **Option 1** (metadata persistence only)
- Sessions die when TUI exits (same as current "detached tmux" behavior when tmux server restarts)
- Store session history and metadata in `~/.coders/sessions.json`
- If true persistence is needed later, upgrade to **Option 2** (daemon-based)

**Rationale:**
- Simplest to implement
- Matches common user expectations (sessions tied to TUI lifecycle)
- Avoids daemon complexity
- Can upgrade to daemon later if needed

---

## Risks and Tradeoffs

### Benefits of Removing tmux

| Benefit | Impact |
|---------|--------|
| **Lower latency** | 10-15ms → <1ms for send-keys operations |
| **Direct control** | Can customize PTY behavior, buffering, parsing |
| **Simpler debugging** | No tmux layer to troubleshoot |
| **Smaller attack surface** | One less external dependency |
| **Custom features** | Can add session recording, playback, etc. |
| **Better integration** | Direct Go API vs. subprocess commands |

### Risks of Removing tmux

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Reimplementing complexity** | HIGH | Use creack/pty for PTY basics, only build what we need |
| **Platform bugs** | MEDIUM | Comprehensive testing on Linux/macOS/FreeBSD |
| **Lost features** | LOW | We don't use most tmux features (copy mode, splits, etc.) |
| **Session persistence** | HIGH | Accept metadata-only persistence initially, or build daemon |
| **Signal handling** | MEDIUM | Properly handle SIGWINCH, SIGTERM, SIGCHLD |
| **Process cleanup** | MEDIUM | Ensure orphaned processes are reaped |
| **Terminal state** | MEDIUM | Restore terminal state on crash/exit |

### Comparison Matrix

| Feature | tmux | Direct PTY |
|---------|------|------------|
| **Latency (send-keys)** | ~10-15ms | <1ms |
| **Dependencies** | tmux binary | None |
| **Code complexity** | Low (external) | Medium (internal) |
| **Session persistence** | Excellent | Metadata-only (or daemon) |
| **Debugging** | Hard (black box) | Easy (full control) |
| **Cross-platform** | Unix only | Unix (+ Windows with ConPty fork) |
| **Output buffering** | Built-in | Custom implementation |
| **ANSI parsing** | Built-in | Needs library (charmbracelet/x/ansi) |
| **Process lifecycle** | Managed | Manual management |
| **Window resize** | Automatic | Manual SIGWINCH handling |

### Decision Factors

**Stay with tmux if:**
- ✅ True session persistence is critical (sessions must survive host reboots)
- ✅ Development velocity is more important than latency
- ✅ Team is unfamiliar with PTY programming
- ✅ Platform-specific bugs are unacceptable

**Switch to direct PTY if:**
- ✅ Latency matters (interactive feel, responsiveness)
- ✅ Want full control over session lifecycle
- ✅ Willing to invest in custom implementation
- ✅ Metadata-only persistence is acceptable
- ✅ Want to enable advanced features (session recording, playback, multiplexing)

---

## Implementation Roadmap

### Phase 1: Proof of Concept (1-2 weeks)

**Goal:** Validate core PTY operations work as expected.

**Tasks:**
1. Create `packages/go/internal/pty/` package
2. Implement basic session creation with `creack/pty`
3. Test output capture and display
4. Implement SendKeys
5. Test AttachSession with raw terminal mode
6. Handle SIGWINCH for window resize
7. Verify process cleanup on session kill

**Deliverable:** Standalone CLI that can spawn, attach, and manage a single PTY session.

---

### Phase 2: Session Manager (2-3 weeks)

**Goal:** Build full session management layer.

**Tasks:**
1. Create `packages/go/internal/sessionmgr/` package
2. Implement `Manager` struct with session registry
3. Build `OutputBuffer` with ANSI parsing (using `charmbracelet/x/ansi`)
4. Add metadata persistence (JSON file)
5. Implement ListSessions, GetSession
6. Add session filtering and sorting
7. Handle concurrent access (mutexes)
8. Process monitoring and cleanup
9. Unit tests for core operations

**Deliverable:** Session manager library with full test coverage.

---

### Phase 3: TUI Integration (2 weeks)

**Goal:** Replace tmux calls in Bubbletea TUI with session manager.

**Tasks:**
1. Replace `tmux.ListSessions()` with `sessionmgr.ListSessions()`
2. Replace `tmux.CapturePane()` with `sessionmgr.CaptureOutput()`
3. Replace `tmux.SendKeys()` with `sessionmgr.SendKeys()`
4. Replace `tmux.AttachSession()` with `sessionmgr.AttachSession()`
5. Update spawn command to use session manager
6. Remove all tmux package imports
7. Integration testing with TUI

**Deliverable:** TUI fully powered by custom session manager.

---

### Phase 4: Polish & Production (1-2 weeks)

**Goal:** Production-ready release.

**Tasks:**
1. Error handling and recovery
2. Logging and observability
3. Performance optimization (reduce allocations, cache parsed ANSI)
4. Edge case testing (rapid session creation, network issues, etc.)
5. Documentation (godoc, README, migration guide)
6. Backward compatibility layer (optional tmux fallback)
7. User acceptance testing

**Deliverable:** Production-ready release without tmux dependency.

---

### Phase 5: Advanced Features (Future)

**Optional enhancements after core implementation:**
- Session recording and playback
- Real-time session sharing (multiple TUI viewers)
- Daemon mode for true session persistence
- Windows support via ConPty (using `aymanbagabas/go-pty`)
- Session snapshot/restore
- Advanced ANSI rendering (images, hyperlinks)

---

## References

### Go PTY Libraries
- [creack/pty](https://github.com/creack/pty) - PTY interface for Go
- [Go Packages: creack/pty](https://pkg.go.dev/github.com/creack/pty)
- [aymanbagabas/go-pty](https://github.com/aymanbagabas/go-pty) - Cross-platform Go PTY with Windows support
- [google/goterm](https://github.com/google/goterm) - Go Terminal library with PTY support

### Terminal Multiplexers
- [tmux manual](https://man7.org/linux/man-pages/man1/tmux.1.html)
- [How to use tmux in 2026](https://www.hostinger.com/tutorials/how-to-use-tmux)
- [A Quick and Easy Guide to tmux](https://hamvocke.com/blog/a-quick-and-easy-guide-to-tmux/)

### Bubbletea & Charm Ecosystem
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [charmbracelet/wish](https://github.com/charmbracelet/wish) - SSH apps with Bubbletea
- [charmbracelet/x/term](https://pkg.go.dev/github.com/charmbracelet/x/term) - Terminal utilities
- [charmbracelet/x/ansi](https://pkg.go.dev/github.com/charmbracelet/x/ansi) - ANSI parsing
- [Wish Bubbletea Example](https://github.com/charmbracelet/wish/blob/main/examples/bubbletea/main.go)

### Session Persistence
- [CRIU - Checkpoint/Restore In Userspace](https://criu.org/Main_Page)
- [checkpoint-restore/go-criu](https://github.com/checkpoint-restore/go-criu) - Go bindings for CRIU
- [CRIU as terminal session manager](https://gist.github.com/salotz/6d01c4a7c126a7857bd62e17e3f5dad0)

### Terminal Architecture
- [Build A Simple Terminal Emulator In 100 Lines of Golang](https://ishuah.com/2021/03/10/build-a-terminal-emulator-in-100-lines-of-go/)
- [The Elegant Architecture of PTYs](https://medium.com/@krithikanithyanandam/the-elegant-architecture-of-ptys-behind-your-terminal-a-quick-byte-b724a50a98b4)
- [Linux terminals, tty, pty and shell - part 2](https://dev.to/napicella/linux-terminals-tty-pty-and-shell-part-2-2cb2)

### Performance & Best Practices
- [WezTerm Multiplexing](https://wezterm.org/multiplexing.html) - Local echo for latency reduction
- [Terminal Multiplexers Explained](https://www.howtogeek.com/terminal-multiplexers-explained/)

---

## Conclusion

**Recommendation:** Proceed with direct PTY management using `github.com/creack/pty`.

**Architecture:** Build a custom session manager that:
1. Creates PTY sessions for spawned processes
2. Maintains output buffers with ANSI parsing
3. Provides low-latency keystroke forwarding
4. Persists session metadata (without full process persistence initially)
5. Integrates cleanly with Bubbletea TUI

**Timeline:** 6-8 weeks for full implementation and testing.

**Risk Level:** Medium - requires careful implementation but well-supported by existing libraries.

**Next Steps:**
1. Review and approve this design
2. Create implementation plan in beads
3. Build POC to validate approach
4. Iterate based on POC findings

---

**END OF DOCUMENT**
