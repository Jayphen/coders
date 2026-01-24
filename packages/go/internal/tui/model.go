package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/types"
)

// Model is the Bubbletea model for the TUI.
type Model struct {
	// Data
	sessions       []types.Session
	selectedIndex  int
	preview        string
	previewSession string
	previewErr     error
	previewLoading bool
	previewLines   int
	previewFocus   bool
	previewInput   textinput.Model
	passthroughMode bool           // When true, forward raw keystrokes to tmux
	lastEscTime     time.Time      // Track last Esc press for double-Esc detection

	// Preview caching - avoid re-splitting on every render
	previewSplitLines []string
	previewSplitText  string

	// UI state
	loading       bool
	err           error
	statusMessage string
	statusExpiry  time.Time
	confirmKill   bool
	spawnMode     bool
	spawnInput    textinput.Model
	spawning      bool
	width, height int
	version       string

	// Components
	spinner spinner.Model

	// Dependencies
	redisClient *redis.Client

	// View caching - avoid re-rendering when state hasn't changed
	cachedView     string
	lastViewState  viewState
	viewStateDirty bool
}

// viewState captures the state that affects View() rendering.
// Changes to any of these fields should trigger a re-render.
type viewState struct {
	sessionCount   int
	selectedIndex  int
	preview        string
	previewSession string
	previewErr     string // error message text
	previewLoading bool
	previewFocus    bool
	passthroughMode bool   // passthrough mode indicator
	loading         bool
	err             string // error message text
	statusMessage   string
	statusExpired   bool   // whether status message should be shown
	confirmKill     bool
	spawnMode       bool
	spawnInput      string
	spawning        bool
	width, height   int
	spinnerView     string // spinner appearance (only when loading)
	previewInput    string // only when previewFocus
}

// Messages
type (
	sessionsMsg      []types.Session
	errMsg           error
	tickMsg          time.Time
	statusClearMsg   struct{}
	spawnCompleteMsg struct{ err error }
	previewMsg       struct {
		session string
		output  string
		err     error
	}
	redisDataMsg struct {
		client       *redis.Client
		promises     map[string]*types.CoderPromise
		heartbeats   map[string]*types.HeartbeatData
		healthChecks map[string]*types.HealthCheckResult
	}
)

const defaultPreviewLines = 30

// NewModel creates a new TUI model.
func NewModel(version string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	ti := textinput.New()
	ti.Placeholder = `claude --task "Fix the bug"`
	ti.CharLimit = 500
	ti.Width = 60

	pi := textinput.New()
	pi.Placeholder = "Type a message..."
	pi.CharLimit = 2000
	pi.Width = 60
	pi.Prompt = ""
	pi.Blur()

	return Model{
		version:      version,
		loading:      true,
		spinner:      s,
		spawnInput:   ti,
		previewLines: defaultPreviewLines,
		previewInput: pi,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchSessions,
		m.fetchRedisData(), // Non-blocking Redis initialization
		m.tick(),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Mark view as potentially dirty - View() will compare state to decide if re-render is needed
	m.viewStateDirty = true

	cmds := make([]tea.Cmd, 0, 3)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsMsg:
		m.sessions = msg
		m.loading = false
		// Ensure selected index is in bounds
		if m.selectedIndex >= len(m.sessions) && len(m.sessions) > 0 {
			m.selectedIndex = len(m.sessions) - 1
		}
		if len(m.sessions) == 0 {
			m.preview = ""
			m.previewSession = ""
			m.previewErr = nil
			m.previewLoading = false
			return m, nil
		}
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.sessions) {
			if m.previewSession != m.sessions[m.selectedIndex].Name {
				previewCmd := m.startPreviewFetch()
				return m, previewCmd
			}
		}
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case tickMsg:
		previewCmd := m.startPreviewFetch()
		return m, tea.Batch(m.fetchSessions, previewCmd, m.tick())

	case statusClearMsg:
		if time.Now().After(m.statusExpiry) {
			m.statusMessage = ""
		}
		return m, nil

	case spawnCompleteMsg:
		m.spawning = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Spawn failed: %v", msg.err))
		} else {
			m.setStatus("Spawn command sent")
		}
		return m, m.fetchSessions

	case previewMsg:
		if msg.session != m.previewSession {
			return m, nil
		}
		m.previewLoading = false
		if msg.err != nil {
			m.previewErr = msg.err
			m.preview = ""
			m.previewSplitLines = nil
			m.previewSplitText = ""
		} else {
			m.previewErr = nil
			m.preview = msg.output
			// Cache split lines to avoid re-splitting on every render
			m.previewSplitLines = strings.Split(msg.output, "\n")
			m.previewSplitText = msg.output
		}
		return m, nil

	case redisDataMsg:
		// Store the Redis client if it was just initialized
		if msg.client != nil && m.redisClient == nil {
			m.redisClient = msg.client
		}
		// Enrich current sessions with Redis data
		if len(m.sessions) > 0 && (msg.promises != nil || msg.heartbeats != nil || msg.healthChecks != nil) {
			enrichSessionsWithRedisData(m.sessions, msg.promises, msg.heartbeats, msg.healthChecks)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update text inputs for non-key messages (e.g. cursor blink).
	if m.spawnMode {
		var cmd tea.Cmd
		m.spawnInput, cmd = m.spawnInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.previewFocus {
		var cmd tea.Cmd
		m.previewInput, cmd = m.previewInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey handles keyboard input.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle spawn mode input
	if m.spawnMode {
		switch msg.String() {
		case "esc":
			m.spawnMode = false
			m.spawnInput.SetValue("")
			m.setStatus("Spawn cancelled")
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.spawnInput.Value())
			m.spawnMode = false
			m.spawnInput.SetValue("")
			if value == "" {
				m.setStatus("Spawn cancelled")
				return m, nil
			}
			m.spawning = true
			m.setStatus("Spawning session...")
			return m, m.spawnSession(value)
		}
		var cmd tea.Cmd
		m.spawnInput, cmd = m.spawnInput.Update(msg)
		return m, cmd
	}

	// Handle confirmation dialog
	if m.confirmKill {
		switch msg.String() {
		case "y", "Y":
			m.confirmKill = false
			return m, m.killCompletedSessions()
		case "n", "N", "enter", "esc":
			m.confirmKill = false
			m.setStatus("Cancelled")
			return m, nil
		}
		return m, nil
	}

	// Handle preview input focus
	if m.previewFocus {
		// Check for passthrough mode toggle (Shift+Tab to enter/exit)
		if msg.String() == "shift+tab" {
			m.passthroughMode = !m.passthroughMode
			if m.passthroughMode {
				m.setStatus("Passthrough mode enabled - keystrokes forwarded to session (Shift+Tab to exit)")
			} else {
				m.setStatus("Passthrough mode disabled")
			}
			return m, nil
		}

		// Handle passthrough mode - forward raw keystrokes
		if m.passthroughMode {
			s := m.selectedSession()
			if s == nil {
				m.setStatus("No session selected")
				m.passthroughMode = false
				return m, nil
			}

			// Allow Ctrl+C to quit even in passthrough mode
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

			// Check for double-Esc to exit passthrough mode
			if msg.String() == "esc" {
				now := time.Now()
				if !m.lastEscTime.IsZero() && now.Sub(m.lastEscTime) < 500*time.Millisecond {
					// Double-Esc detected - exit passthrough mode
					m.passthroughMode = false
					m.setStatus("Passthrough mode disabled")
					m.lastEscTime = time.Time{}
					return m, nil
				}
				m.lastEscTime = now
				// Still forward the first Esc to the session
			}

			// Forward the keystroke to tmux
			if err := tmux.SendRawKey(s.Name, msg.String()); err != nil {
				m.setStatus(fmt.Sprintf("Send key failed: %v", err))
			}
			return m, nil
		}

		// Normal buffered input mode (legacy behavior)
		switch msg.String() {
		case "tab":
			m.previewFocus = false
			m.previewInput.Blur()
			return m, nil
		case "enter":
			text := strings.TrimSpace(m.previewInput.Value())
			if text == "" {
				return m, nil
			}
			s := m.selectedSession()
			if s == nil {
				m.setStatus("No session selected")
				return m, nil
			}
			if err := tmux.SendKeys(s.Name, text); err != nil {
				m.setStatus(fmt.Sprintf("Send failed: %v", err))
			} else {
				m.setStatus("Sent to session")
			}
			m.previewInput.SetValue("")
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.previewInput, cmd = m.previewInput.Update(msg)
		return m, cmd
	}

	// Normal mode key handling
	switch msg.String() {
	case "tab":
		m.previewFocus = true
		m.previewInput.Focus()
		return m, textinput.Blink

	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m, m.startPreviewFetch()

	case "down", "j":
		if m.selectedIndex < len(m.sessions)-1 {
			m.selectedIndex++
		}
		return m, m.startPreviewFetch()

	case "enter", "a":
		if len(m.sessions) > 0 && m.selectedIndex < len(m.sessions) {
			session := m.sessions[m.selectedIndex]
			tmux.AttachSession(session.Name)
		}
		return m, nil

	case "s":
		if m.spawning {
			m.setStatus("Spawn already in progress")
			return m, nil
		}
		m.spawnMode = true
		m.previewFocus = false
		m.previewInput.Blur()
		m.spawnInput.Focus()
		return m, textinput.Blink

	case "K":
		if len(m.sessions) > 0 && m.selectedIndex < len(m.sessions) {
			session := m.sessions[m.selectedIndex]
			tmux.KillSession(session.Name)
			if m.redisClient != nil {
				m.redisClient.DeletePromise(context.Background(), session.Name)
			}
			m.setStatus(fmt.Sprintf("Killed: %s", session.Name))
			return m, m.fetchSessions
		}
		return m, nil

	case "C":
		completedCount := m.countCompleted()
		if completedCount > 0 {
			m.confirmKill = true
		} else {
			m.setStatus("No completed sessions to kill")
		}
		return m, nil

	case "R":
		if len(m.sessions) > 0 && m.selectedIndex < len(m.sessions) {
			session := m.sessions[m.selectedIndex]
			if session.HasPromise && m.redisClient != nil {
				m.redisClient.DeletePromise(context.Background(), session.Name)
				m.setStatus(fmt.Sprintf("Resumed: %s", strings.TrimPrefix(session.Name, tmux.SessionPrefix)))
				return m, m.fetchSessions
			} else {
				m.setStatus("Selected session is not completed")
			}
		}
		return m, nil

	case "r":
		return m, m.fetchSessions
	}

	return m, nil
}

// currentViewState captures the current state that affects rendering.
func (m Model) currentViewState() viewState {
	vs := viewState{
		sessionCount:    len(m.sessions),
		selectedIndex:   m.selectedIndex,
		preview:         m.preview,
		previewSession:  m.previewSession,
		previewLoading:  m.previewLoading,
		previewFocus:    m.previewFocus,
		passthroughMode: m.passthroughMode,
		loading:         m.loading,
		statusMessage:   m.statusMessage,
		statusExpired:   time.Now().After(m.statusExpiry),
		confirmKill:     m.confirmKill,
		spawnMode:       m.spawnMode,
		spawning:        m.spawning,
		width:           m.width,
		height:          m.height,
	}

	if m.previewErr != nil {
		vs.previewErr = m.previewErr.Error()
	}
	if m.err != nil {
		vs.err = m.err.Error()
	}
	if m.loading {
		vs.spinnerView = m.spinner.View()
	}
	if m.previewFocus {
		vs.previewInput = m.previewInput.Value()
	}
	if m.spawnMode {
		vs.spawnInput = m.spawnInput.Value()
	}

	return vs
}

// View renders the UI.
func (m *Model) View() string {
	// Check if we can use cached view
	if !m.viewStateDirty && m.cachedView != "" {
		currentState := m.currentViewState()
		if currentState == m.lastViewState {
			return m.cachedView
		}
	}

	// State has changed, render the view
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Confirmation dialog
	if m.confirmKill {
		confirm := m.renderConfirmDialog()
		b.WriteString(confirm)
		b.WriteString("\n")
	}

	// Spawn prompt
	if m.spawnMode {
		spawn := m.renderSpawnPrompt()
		b.WriteString(spawn)
		b.WriteString("\n")
	}

	contentHeight := 0
	if m.height > 0 {
		innerHeight := m.height - 2 // outer padding
		usedHeight := lipgloss.Height(header) + 1
		if m.confirmKill {
			usedHeight += lipgloss.Height(m.renderConfirmDialog()) + 1
		}
		if m.spawnMode {
			usedHeight += lipgloss.Height(m.renderSpawnPrompt()) + 1
		}
		statusBar := m.renderStatusBar()
		usedHeight += 1 + lipgloss.Height(statusBar)
		contentHeight = innerHeight - usedHeight
		if contentHeight < 1 {
			contentHeight = 1
		}
	}

	// Error or content
	if m.err != nil {
		errLine := ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		if contentHeight > 0 {
			errLine = truncateLines(errLine, contentHeight, "")
		}
		b.WriteString(errLine)
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderMainContent(contentHeight))
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	rendered := lipgloss.NewStyle().Padding(1).Render(b.String())

	// Cache the rendered view and state
	m.cachedView = rendered
	m.lastViewState = m.currentViewState()
	m.viewStateDirty = false

	return rendered
}

// Helper methods

func (m *Model) setStatus(msg string) {
	m.statusMessage = msg
	m.statusExpiry = time.Now().Add(3 * time.Second)
}

func (m Model) countCompleted() int {
	count := 0
	for _, s := range m.sessions {
		if s.HasPromise && !s.IsOrchestrator {
			count++
		}
	}
	return count
}

func (m Model) selectedSession() *types.Session {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.sessions) {
		return &m.sessions[m.selectedIndex]
	}
	return nil
}

func (m *Model) startPreviewFetch() tea.Cmd {
	s := m.selectedSession()
	if s == nil {
		m.previewSession = ""
		m.preview = ""
		m.previewErr = nil
		m.previewLoading = false
		return nil
	}
	lines := m.previewLines
	if lines <= 0 {
		lines = defaultPreviewLines
	}
	m.previewLoading = true
	m.previewSession = s.Name
	return m.fetchPreview(s.Name, lines)
}

// Commands

func (m Model) tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchSessions() tea.Msg {
	// Get tmux sessions
	sessions, err := tmux.ListSessions()
	if err != nil {
		return errMsg(err)
	}

	// Enrich sessions with Redis data if client is already initialized
	if m.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		promises, _ := m.redisClient.GetPromises(ctx)
		heartbeats, _ := m.redisClient.GetHeartbeats(ctx)
		healthChecks, _ := m.redisClient.GetHealthChecks(ctx)

		enrichSessionsWithRedisData(sessions, promises, heartbeats, healthChecks)
	}

	// Sort: orchestrator first, then active, then completed, by creation time
	// Optimize by pre-calculating sort keys to avoid repeated comparisons
	type sortKey struct {
		priority  int   // 0=orchestrator, 1=active, 2=completed
		timestamp int64 // negative for reverse chronological (newer first)
		index     int   // preserve stable sort for equal elements
	}

	keys := make([]sortKey, len(sessions))
	for i := range sessions {
		s := &sessions[i]
		key := sortKey{index: i}

		if s.IsOrchestrator {
			key.priority = 0
		} else if s.HasPromise {
			key.priority = 2 // Completed (has promise)
		} else {
			key.priority = 1 // Active (no promise)
		}

		if s.CreatedAt != nil {
			key.timestamp = -s.CreatedAt.Unix() // Negative for reverse order
		}

		keys[i] = key
	}

	sort.Slice(sessions, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		if ki.priority != kj.priority {
			return ki.priority < kj.priority
		}
		if ki.timestamp != kj.timestamp {
			return ki.timestamp < kj.timestamp
		}
		return ki.index < kj.index
	})

	return sessionsMsg(sessions)
}

func (m Model) fetchPreview(sessionName string, lines int) tea.Cmd {
	return func() tea.Msg {
		output, err := tmux.CapturePane(sessionName, lines)
		return previewMsg{session: sessionName, output: output, err: err}
	}
}

func (m Model) spawnSession(args string) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			return spawnCompleteMsg{err: err}
		}

		parsedArgs, err := parseSpawnArgs(args)
		if err != nil {
			return spawnCompleteMsg{err: err}
		}
		if len(parsedArgs) == 0 {
			return spawnCompleteMsg{err: fmt.Errorf("no spawn arguments provided")}
		}

		cmd := exec.Command(exe, append([]string{"spawn"}, parsedArgs...)...)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(output.String())
			if msg != "" {
				return spawnCompleteMsg{err: fmt.Errorf("%w: %s", err, msg)}
			}
			return spawnCompleteMsg{err: err}
		}
		return spawnCompleteMsg{err: nil}
	}
}

func (m Model) killCompletedSessions() tea.Cmd {
	return func() tea.Msg {
		killed := 0
		for _, s := range m.sessions {
			if s.HasPromise && !s.IsOrchestrator {
				if err := tmux.KillSession(s.Name); err == nil {
					killed++
					if m.redisClient != nil {
						m.redisClient.DeletePromise(context.Background(), s.Name)
					}
				}
			}
		}
		m.setStatus(fmt.Sprintf("Killed %d completed session(s)", killed))
		return nil
	}
}

func parseSpawnArgs(input string) ([]string, error) {
	// Estimate capacity: rough heuristic of input_length/10 + 2, min 4
	estCap := len(input)/10 + 2
	if estCap < 4 {
		estCap = 4
	}
	args := make([]string, 0, estCap)
	var b strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if b.Len() > 0 {
			args = append(args, b.String())
			b.Reset()
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			flush()
		default:
			b.WriteRune(r)
		}
	}

	if escaped {
		b.WriteRune('\\')
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in spawn arguments")
	}
	flush()
	return args, nil
}

// enrichSessionsWithRedisData enriches sessions with Redis data
func enrichSessionsWithRedisData(
	sessions []types.Session,
	promises map[string]*types.CoderPromise,
	heartbeats map[string]*types.HeartbeatData,
	healthChecks map[string]*types.HealthCheckResult,
) {
	for i := range sessions {
		s := &sessions[i]

		// Add promise data
		if promise, ok := promises[s.Name]; ok {
			s.Promise = promise
			s.HasPromise = true
		}

		// Add heartbeat data
		if hb, ok := heartbeats[s.Name]; ok {
			s.HeartbeatStatus = redis.DetermineHeartbeatStatus(hb)
			if hb.Task != "" && s.Task == "" {
				s.Task = hb.Task
			}
			if hb.ParentSessionID != "" {
				s.ParentSessionID = hb.ParentSessionID
			}
			s.Usage = hb.Usage
		} else if s.IsOrchestrator {
			s.HeartbeatStatus = types.HeartbeatHealthy
		} else {
			s.HeartbeatStatus = types.HeartbeatDead
		}

		// Add health check data (for stuck/unresponsive detection)
		if hc, ok := healthChecks[s.Name]; ok {
			s.HealthCheck = hc
		}
	}
}

// fetchRedisData asynchronously initializes Redis client and fetches data
func (m Model) fetchRedisData() tea.Cmd {
	return func() tea.Msg {
		// Initialize client if needed
		client := m.redisClient
		if client == nil {
			var err error
			client, err = redis.NewClient()
			if err != nil {
				// Return empty data on error (non-fatal)
				return redisDataMsg{}
			}
		}

		// Fetch data with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		promises, _ := client.GetPromises(ctx)
		heartbeats, _ := client.GetHeartbeats(ctx)
		healthChecks, _ := client.GetHealthChecks(ctx)

		return redisDataMsg{
			client:       client,
			promises:     promises,
			heartbeats:   heartbeats,
			healthChecks: healthChecks,
		}
	}
}
