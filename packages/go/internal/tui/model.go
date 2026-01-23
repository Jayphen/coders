package tui

import (
	"context"
	"fmt"
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
	sessions      []types.Session
	selectedIndex int

	// UI state
	loading        bool
	err            error
	statusMessage  string
	statusExpiry   time.Time
	confirmKill    bool
	spawnMode      bool
	spawnInput     textinput.Model
	spawning       bool
	width, height  int
	version        string

	// Components
	spinner spinner.Model

	// Dependencies
	redisClient *redis.Client
}

// Messages
type (
	sessionsMsg       []types.Session
	errMsg            error
	tickMsg           time.Time
	statusClearMsg    struct{}
	spawnCompleteMsg  struct{ err error }
)

// NewModel creates a new TUI model.
func NewModel(version string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	ti := textinput.New()
	ti.Placeholder = `claude --task "Fix the bug"`
	ti.CharLimit = 500
	ti.Width = 60

	return Model{
		version:  version,
		loading:  true,
		spinner:  s,
		spawnInput: ti,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchSessions,
		m.tick(),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.fetchSessions, m.tick())

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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update text input if in spawn mode
	if m.spawnMode {
		var cmd tea.Cmd
		m.spawnInput, cmd = m.spawnInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey handles keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m, nil
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

	// Normal mode key handling
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.selectedIndex < len(m.sessions)-1 {
			m.selectedIndex++
		}
		return m, nil

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

// View renders the UI.
func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Confirmation dialog
	if m.confirmKill {
		b.WriteString(m.renderConfirmDialog())
		b.WriteString("\n")
	}

	// Spawn prompt
	if m.spawnMode {
		b.WriteString(m.renderSpawnPrompt())
		b.WriteString("\n")
	}

	// Status message
	if m.statusMessage != "" && !m.confirmKill {
		b.WriteString(StatusMsgStyle.Render(m.statusMessage))
		b.WriteString("\n\n")
	}

	// Error or content
	if m.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderSessionList())
		b.WriteString("\n")
		b.WriteString(m.renderSessionDetail())
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return lipgloss.NewStyle().Padding(1).Render(b.String())
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

	// Try to get Redis data (non-fatal if unavailable)
	var promises map[string]*types.CoderPromise
	var heartbeats map[string]*types.HeartbeatData

	if m.redisClient == nil {
		client, err := redis.NewClient()
		if err == nil {
			m.redisClient = client
		}
	}

	if m.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		promises, _ = m.redisClient.GetPromises(ctx)
		heartbeats, _ = m.redisClient.GetHeartbeats(ctx)
	}

	// Enrich sessions with Redis data
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
	}

	// Sort: orchestrator first, then active, then completed, by creation time
	sort.Slice(sessions, func(i, j int) bool {
		a, b := sessions[i], sessions[j]
		if a.IsOrchestrator {
			return true
		}
		if b.IsOrchestrator {
			return false
		}
		if a.HasPromise != b.HasPromise {
			return !a.HasPromise // Active before completed
		}
		if a.CreatedAt != nil && b.CreatedAt != nil {
			return a.CreatedAt.After(*b.CreatedAt)
		}
		return false
	})

	return sessionsMsg(sessions)
}

func (m Model) spawnSession(args string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement spawn via coders CLI
		// For now, just return success
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
