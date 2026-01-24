# Investigation: Converting TUI from TypeScript/React to Go

**Date:** January 2025
**Status:** Investigation Only - No Code Changes

## Executive Summary

This document analyzes the feasibility and implications of converting the Coders TUI from TypeScript/React (Ink) to Go. The current TUI is approximately 1,365 lines of code across 9 files. Converting to Go would provide significant performance benefits, particularly in startup time (~20x faster), while requiring a moderate development effort (estimated 2-3 weeks).

**Recommendation:** Bubbletea is the recommended Go framework if proceeding, due to its modern architecture, active ecosystem, and good component library.

---

## Current Architecture Analysis

### Technology Stack

| Component | Current Implementation |
|-----------|----------------------|
| Runtime | Node.js (v18+) |
| Framework | [Ink](https://github.com/vadimdemedes/ink) (React for CLI) |
| Language | TypeScript |
| UI Paradigm | React components with hooks |
| Dependencies | ink, ink-select-input, ink-spinner, ink-text-input, react, redis |

### Code Structure

```
packages/tui/
├── src/
│   ├── cli.tsx          (91 lines)  - Entry point, tmux session management
│   ├── app.tsx          (307 lines) - Main application component with state
│   ├── types.ts         (49 lines)  - TypeScript interfaces
│   ├── tmux.ts          (459 lines) - Tmux & Redis operations
│   └── components/
│       ├── Header.tsx       (26 lines)
│       ├── SessionList.tsx  (94 lines)
│       ├── SessionRow.tsx   (96 lines)
│       ├── SessionDetail.tsx (189 lines)
│       └── StatusBar.tsx    (63 lines)
└── Total: ~1,365 lines
```

### Key Features to Preserve

1. **Session List View** - Displays active/completed coder sessions with status indicators
2. **Session Detail Panel** - Shows detailed info for selected session
3. **Real-time Updates** - Auto-refresh every 5 seconds
4. **Keyboard Navigation** - vim-style (j/k) + arrow keys
5. **Session Actions** - Attach, kill, spawn, resume
6. **Redis Integration** - Fetches heartbeats and promises
7. **Tmux Integration** - List sessions, attach, kill, spawn
8. **Visual Styling** - Color-coded tools, status indicators, progress bars

### External Integrations

| Integration | Method | Complexity |
|-------------|--------|------------|
| Tmux | Shell commands via `execSync` | Low |
| Redis | Shell commands via `redis-cli` | Low |
| Process Management | `ps`, `kill` via shell | Low |

---

## Go TUI Framework Options

### Option 1: Bubbletea (Recommended)

[Bubbletea](https://github.com/charmbracelet/bubbletea) by Charm Bracelet is a modern TUI framework based on The Elm Architecture (TEA).

**Architecture:**
- Model-Update-View pattern (similar to React/Redux)
- Immutable state updates
- Command-based side effects

**Ecosystem (Charm Bracelet):**
- **[Bubbles](https://github.com/charmbracelet/bubbles)** - Pre-built components (spinners, text inputs, lists, tables)
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** - Styling and layout (similar to CSS-in-JS)
- **Harmonica** - Spring animations
- **BubbleZone** - Mouse event tracking

**Pros:**
- Active development with 10,000+ apps built
- Familiar architecture for React developers
- Excellent component ecosystem
- Good documentation and examples
- Strong community support

**Cons:**
- Opinionated architecture (must follow TEA pattern)
- Learning curve for Elm-style programming
- Some report setup complexity for simple use cases

**Relevant Components for TUI:**
- `list` - For session list
- `spinner` - For loading states
- `textinput` - For spawn prompt
- `viewport` - For scrollable content
- `table` - Alternative for session list

### Option 2: tview

[tview](https://github.com/rivo/tview) is a more traditional widget-based TUI library built on tcell.

**Architecture:**
- Widget-based (similar to desktop GUI toolkits)
- Direct mutation of widget state
- Event callbacks

**Available Widgets:**
- TextView, TextArea, Table
- Button, Checkbox, DropDown, InputField
- Modal, Frame
- Grid, Flex layouts

**Pros:**
- Lower learning curve for imperative programmers
- Rich built-in widget library
- Good backwards compatibility
- Used by K9s (Kubernetes CLI)

**Cons:**
- More imperative/mutable style
- Less modern architecture
- Smaller ecosystem for styling
- Some find debugging harder with callbacks

### Comparison Matrix

| Feature | Bubbletea | tview |
|---------|-----------|-------|
| Architecture | Elm/Functional | Imperative/Widget |
| Learning Curve | Medium (familiar to React devs) | Low |
| Styling | Lip Gloss (powerful) | Basic |
| Components | Bubbles library | Built-in widgets |
| Maintenance | Very active | Active |
| GitHub Stars | 28k+ | 11k+ |
| Documentation | Excellent | Good |

---

## Implementation Mapping

### Component Mapping

| Current (TypeScript/Ink) | Go Equivalent (Bubbletea) |
|--------------------------|---------------------------|
| React component | Model struct + View function |
| `useState` | Fields in Model |
| `useEffect` (polling) | `tea.Tick` command |
| `useCallback` | Method on Model |
| `useInput` | Update function with `tea.KeyMsg` |
| `<Box>` | `lipgloss.Style` |
| `<Text>` | String with lipgloss styling |
| `<Spinner>` | `bubbles/spinner` |
| `TextInput` | `bubbles/textinput` |

### State Management Translation

**Current (React):**
```typescript
const [sessions, setSessions] = useState<Session[]>([]);
const [selectedIndex, setSelectedIndex] = useState(0);
```

**Go (Bubbletea):**
```go
type model struct {
    sessions      []Session
    selectedIndex int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle key press
    }
    return m, nil
}
```

### Redis/Tmux Integration

Go equivalents for shell integrations:

| Current | Go Approach |
|---------|-------------|
| `execSync('tmux ...')` | `exec.Command("tmux", args...).Output()` |
| `execSync('redis-cli ...')` | `exec.Command("redis-cli", args...)` or [go-redis](https://github.com/redis/go-redis) library |

**Note:** The current implementation uses `redis-cli` shell commands. In Go, we could:
1. Continue using `redis-cli` via `exec.Command` (simplest migration)
2. Use go-redis library for native integration (cleaner, better performance)

---

## Effort Assessment

### Development Tasks

| Task | Estimated Effort | Notes |
|------|-----------------|-------|
| Project setup & dependencies | 2 hours | go mod, bubbletea, bubbles, lipgloss |
| Types/models translation | 2 hours | Session, HeartbeatData, CoderPromise |
| Tmux integration | 4 hours | Port tmux.ts functions |
| Redis integration | 3 hours | Port or use go-redis |
| Main model & update loop | 8 hours | Core app logic |
| SessionList component | 4 hours | List with sections |
| SessionRow component | 2 hours | Row rendering |
| SessionDetail component | 4 hours | Detail panel with styling |
| Header component | 1 hour | Simple text |
| StatusBar component | 2 hours | Help text and counts |
| Keyboard handling | 3 hours | Input mapping |
| Spawn dialog | 3 hours | Text input modal |
| Confirmation dialog | 2 hours | Kill confirmation |
| Styling with Lip Gloss | 4 hours | Colors, borders, layout |
| Testing & debugging | 8 hours | Manual testing, edge cases |
| Documentation | 2 hours | Update README |

**Total Estimated Effort: 52-65 hours (2-3 weeks)**

### Risk Factors

| Risk | Impact | Mitigation |
|------|--------|------------|
| Elm architecture learning curve | Medium | Use Bubbletea tutorials, start simple |
| Styling parity with Ink | Low | Lip Gloss is very capable |
| Tmux edge cases | Low | Same shell commands, well-tested |
| Redis connection handling | Low | Can use same redis-cli approach |

---

## Pros and Cons of Migration

### Advantages of Go

1. **Startup Performance (~20x faster)**
   - Go: ~0.002s startup
   - Node.js: ~0.04s startup
   - Critical for frequently-invoked CLI tools

2. **Single Binary Distribution**
   - No npm install required
   - No Node.js runtime dependency
   - Simpler installation: just download binary

3. **Lower Memory Footprint**
   - Go uses less memory than Node.js
   - Important for long-running TUI sessions

4. **Better Concurrency Model**
   - Goroutines are lighter than Node.js event loop
   - Easier parallel Redis/Tmux operations

5. **Type Safety at Compile Time**
   - Catches errors before runtime
   - Better refactoring support

6. **Cross-Compilation**
   - Build for Linux/macOS/Windows from any platform
   - No platform-specific npm dependencies

### Disadvantages of Migration

1. **Development Time**
   - 2-3 weeks of development effort
   - Testing and stabilization period

2. **Loss of React Ecosystem**
   - Ink has many community components
   - React patterns are widely known

3. **Team Knowledge**
   - Go may require learning curve
   - TypeScript/React is more common skill

4. **Build Complexity**
   - Need to build/distribute binaries
   - Multi-platform CI setup required

5. **Maintenance Burden**
   - Two codebases if gradual migration
   - Feature parity maintenance

---

## Distribution Considerations

### Current Distribution (npm)

```bash
npm install -g @jayphen/coders-tui
coders-tui
```
- Requires Node.js runtime
- ~10 dependencies to install
- Platform-agnostic (runs anywhere Node runs)

### Go Distribution Options

**Option A: Pre-built Binaries (Recommended)**
```bash
# Download from GitHub releases
curl -L https://github.com/Jayphen/coders/releases/download/v1.0.0/coders-tui-darwin-arm64 -o coders-tui
chmod +x coders-tui
```
- Zero dependencies
- Fastest startup
- Larger download (~10-15MB per platform)

**Option B: Go Install**
```bash
go install github.com/Jayphen/coders/packages/tui@latest
```
- Requires Go toolchain
- Builds from source

**Option C: Homebrew Formula**
```bash
brew install jayphen/tap/coders-tui
```
- Good UX for macOS users
- Requires maintaining formula

### Binary Size Estimates

| Target | Estimated Size |
|--------|---------------|
| darwin/arm64 | 8-12 MB |
| darwin/amd64 | 9-13 MB |
| linux/amd64 | 8-12 MB |
| windows/amd64 | 9-14 MB |

---

## Alternative Approaches

### Hybrid Approach: Keep Node.js, Optimize Startup

Instead of full rewrite, optimize current approach:

1. **Lazy load dependencies** - Reduce initial bundle
2. **Use esbuild** - Faster than tsc
3. **Pre-compile with pkg** - Bundle Node.js into binary
4. **Optimize Redis calls** - Batch more aggressively

**Estimated improvement:** 30-50% faster startup (still slower than Go)

### Keep Current Stack, Accept Tradeoffs

- Startup time difference may be acceptable for interactive TUI
- Node.js ecosystem advantages remain
- Zero migration effort

---

## Recommendations

### If Performance is Critical: Proceed with Go + Bubbletea

1. Start with tmux/redis integration layer (most testable)
2. Build core model and update loop
3. Add components incrementally
4. Use Lip Gloss for styling parity
5. Set up GitHub Actions for multi-platform builds

### If Time is Constrained: Optimize Current Stack

1. Switch to esbuild for faster builds
2. Lazy-load ink components
3. Profile and optimize startup path
4. Consider pkg for single-binary distribution

### Migration Timeline (if proceeding)

| Week | Milestone |
|------|-----------|
| 1 | Core model, tmux/redis integration, basic rendering |
| 2 | All components, keyboard handling, styling |
| 3 | Testing, bug fixes, documentation, CI/CD |

---

## Appendix: Example Code Sketches

### Bubbletea Model Structure

```go
package main

import (
    "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/lipgloss"
)

type Session struct {
    Name            string
    Tool            string
    Task            string
    IsOrchestrator  bool
    HeartbeatStatus string
    HasPromise      bool
}

type model struct {
    sessions      []Session
    list          list.Model
    selectedIndex int
    loading       bool
    err           error
    confirmKill   bool
    spawnMode     bool
    width, height int
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        fetchSessions,
        tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
            return tickMsg(t)
        }),
    )
}
```

### Lip Gloss Styling Example

```go
var (
    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("cyan")).
        Bold(true)

    dimStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("gray"))

    borderStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("gray")).
        Padding(1, 2)
)
```

---

## Extended Analysis: Go for Other Plugin Components

### Current Plugin Architecture Overview

```
packages/plugin/
├── skills/coders/
│   ├── scripts/
│   │   ├── main.js          (1,564 lines) - Core CLI: spawn, list, attach, kill, promise
│   │   ├── loop-runner.js   (294 lines)  - Automated task loop runner
│   │   ├── orchestrator.js  (140 lines)  - Orchestrator state management
│   │   ├── session-name.js  (32 lines)   - Session name generation
│   │   └── ai-name-generator.js (153 lines) - AI-powered naming
│   ├── coders.ts            (685 lines)  - Skill definition for Claude
│   ├── redis.ts             (738 lines)  - Redis integration helpers
│   └── tmux-resurrect.ts    (451 lines)  - Tmux session restore
└── Total: ~5,355 lines (scripts + skills)
```

### Component-by-Component Analysis

| Component | Current | Go Benefit | Recommended |
|-----------|---------|------------|-------------|
| **TUI** | TypeScript/Ink | High (startup, distribution) | ✅ Yes |
| **main.js CLI** | Node.js | High (startup critical for CLI) | ✅ Yes |
| **loop-runner.js** | Node.js | Medium (long-running, startup less critical) | ⚠️ Maybe |
| **Skill definitions** | TypeScript | None (Claude plugin requirement) | ❌ No |
| **Redis helpers** | TypeScript | None (used by skills) | ❌ No |

### Components Where Go Makes Sense

#### 1. CLI Commands (`main.js`) - **Strongly Recommended**

The CLI (`coders spawn`, `coders list`, `coders attach`, etc.) is invoked frequently from the terminal. Go would provide:

- **~20x faster startup** - Critical for responsive CLI experience
- **Single binary** - No Node.js dependency, simpler installation
- **Better process management** - Native goroutines for tmux/Redis operations

**Current pain points Go would solve:**
- Cold start delay when running `coders spawn`
- Requires Node.js to be installed
- npm dependency management complexity

#### 2. Loop Runner - **Consider**

The loop runner is long-running (monitors promises, spawns tasks), so startup time is less critical. However:

**Pros:**
- Lower memory footprint for background process
- Better concurrent Redis polling with goroutines
- Could be part of unified binary

**Cons:**
- Already runs as background process (startup less impactful)
- May need Node.js anyway for AI name generation

#### 3. Heartbeat Script - **Consider**

Currently runs as background Node.js process. Go would reduce memory footprint for what's essentially a timer + Redis write loop.

### Components That Should Stay Node.js/TypeScript

#### Skill Definitions (coders.ts, redis.ts, etc.)

These are **loaded by Claude Code** as plugin skills. Claude's plugin system expects:
- TypeScript/JavaScript files
- Specific SKILL.md format
- Integration with Claude's runtime

**Cannot be rewritten in Go** - Claude plugin architecture requirement.

### Unified Go Binary Strategy

If proceeding with Go, consider a **single unified binary** that handles:

```
coders                    # Main CLI entry point
├── coders spawn          # Spawn new session
├── coders list           # List sessions
├── coders attach         # Attach to session
├── coders kill           # Kill session
├── coders promise        # Publish completion promise
├── coders tui            # Launch TUI (currently separate)
├── coders loop           # Run task loop (could be subcommand)
└── coders heartbeat      # Run heartbeat (could be subcommand)
```

**Benefits of unified binary:**
- Single ~12MB download instead of TUI + CLI + Node.js
- Shared code for tmux/Redis operations
- Consistent versioning
- Simpler installation: `curl ... | sh`

### Hybrid Architecture Recommendation

```
┌─────────────────────────────────────────────────────────┐
│                    User's Terminal                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────┐    ┌──────────────────────────┐  │
│  │   Go Binary       │    │   Claude Code Plugin     │  │
│  │   (coders)        │    │   (TypeScript)           │  │
│  │                   │    │                          │  │
│  │ • spawn           │◄───┤ • /coders:spawn          │  │
│  │ • list            │    │ • /coders:list           │  │
│  │ • attach          │    │ • /coders:promise        │  │
│  │ • kill            │    │                          │  │
│  │ • promise         │    │ Calls Go binary via      │  │
│  │ • tui             │    │ exec for all commands    │  │
│  │ • loop            │    │                          │  │
│  │ • heartbeat       │    └──────────────────────────┘  │
│  │                   │                                   │
│  └──────────────────┘                                   │
│           │                                              │
│           ▼                                              │
│  ┌──────────────────┐    ┌──────────────────────────┐  │
│  │      tmux         │    │        Redis             │  │
│  │   (sessions)      │    │   (state/promises)       │  │
│  └──────────────────┘    └──────────────────────────┘  │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**How it works:**
1. **Go binary** handles all CLI operations (fast startup)
2. **Claude plugin** (TypeScript) calls the Go binary via `exec`
3. **Skills** remain TypeScript (Claude requirement)
4. **Shared state** via Redis (works across languages)

### Migration Path

| Phase | Scope | Effort | Value |
|-------|-------|--------|-------|
| 1 | TUI only | 2-3 weeks | High (immediate UX improvement) |
| 2 | Core CLI (spawn, list, attach, kill) | 2 weeks | Very High (most-used commands) |
| 3 | Promise/heartbeat | 1 week | Medium (completes feature set) |
| 4 | Loop runner | 1 week | Low (background process) |

### Cost-Benefit Summary

| Approach | Total Effort | Startup Improvement | Distribution Improvement |
|----------|-------------|---------------------|-------------------------|
| TUI only | 2-3 weeks | TUI: 20x faster | TUI becomes single binary |
| TUI + CLI | 4-5 weeks | Everything: 20x faster | Full single binary distribution |
| Keep Node.js | 0 weeks | None | None |

### Final Recommendation

**If investing in Go migration, do TUI + CLI together:**

1. The value of Go is maximized when the entire user-facing CLI is fast
2. Shared code (tmux, Redis, types) reduces total effort vs. separate rewrites
3. Single binary distribution is the main user benefit
4. Plugin skills must stay TypeScript regardless

**Estimated total effort for TUI + CLI: 4-5 weeks**

This creates a clean architecture where:
- All user-invoked commands are instant (Go)
- Claude integration works seamlessly (TypeScript skills call Go binary)
- Distribution is trivial (single binary download)

---

## References

- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [tview GitHub](https://github.com/rivo/tview)
- [go-redis Client](https://github.com/redis/go-redis)
- [Exploring TUI Libraries in Go](https://leapcell.io/blog/exploring-tui-libraries-in-go)
- [Go vs Node.js Startup Comparison](https://www.go-on-aws.com/optimize/poly-start/)
- [Building TUIs with Bubbletea](https://www.inngest.com/blog/interactive-clis-with-bubbletea)
