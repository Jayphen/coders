package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TermViewModel represents an embedded terminal view component.
// It displays PTY output with ANSI support, scrolling, and focus mode.
type TermViewModel struct {
	// PTY connection (can be nil for mock/testing)
	pty io.ReadWriteCloser

	// Output buffer
	buffer       *TerminalBuffer
	viewport     viewport.Model
	viewportInit bool

	// Focus state
	focused bool

	// Session info
	sessionID   string
	sessionName string

	// Dimensions
	width  int
	height int

	// Polling
	lastUpdate time.Time

	// Styles
	borderColor lipgloss.Color
}

// TerminalBuffer manages PTY output with ANSI parsing and scrollback.
type TerminalBuffer struct {
	lines    []string     // Parsed lines of output
	maxLines int          // Maximum scrollback lines
	mu       sync.RWMutex // Protects concurrent access
	parser   *ansi.Parser // ANSI escape code parser
	rawBuf   []byte       // Raw data buffer for incomplete reads
}

// NewTerminalBuffer creates a new terminal buffer with the specified scrollback limit.
func NewTerminalBuffer(maxLines int) *TerminalBuffer {
	if maxLines <= 0 {
		maxLines = 1000 // Default scrollback
	}

	return &TerminalBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
		parser:   &ansi.Parser{},
		rawBuf:   make([]byte, 0, 4096),
	}
}

// Append adds new data to the buffer, parsing ANSI codes and splitting into lines.
func (b *TerminalBuffer) Append(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Append to raw buffer
	b.rawBuf = append(b.rawBuf, data...)

	// Split on newlines, keeping incomplete lines in buffer
	text := string(b.rawBuf)
	lines := strings.Split(text, "\n")

	// If the last element doesn't end with newline, keep it in buffer
	if len(lines) > 0 && !strings.HasSuffix(text, "\n") {
		b.rawBuf = []byte(lines[len(lines)-1])
		lines = lines[:len(lines)-1]
	} else {
		b.rawBuf = b.rawBuf[:0]
		// If text ends with \n, the split will have an empty string at the end
		// Remove it
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	// Parse ANSI codes from each line and append
	for _, line := range lines {
		// Keep ANSI codes to preserve colors in output
		b.lines = append(b.lines, line)
	}

	// Trim to max lines if exceeded
	if len(b.lines) > b.maxLines {
		overflow := len(b.lines) - b.maxLines
		b.lines = b.lines[overflow:]
	}
}

// GetLines returns the last n lines from the buffer.
// If n <= 0 or n > total lines, returns all lines.
func (b *TerminalBuffer) GetLines(n int) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 || n >= len(b.lines) {
		result := make([]string, len(b.lines))
		copy(result, b.lines)
		return result
	}

	start := len(b.lines) - n
	result := make([]string, n)
	copy(result, b.lines[start:])
	return result
}

// GetAllLines returns all lines in the buffer.
func (b *TerminalBuffer) GetAllLines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// Clear clears the buffer.
func (b *TerminalBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = b.lines[:0]
	b.rawBuf = b.rawBuf[:0]
}

// LineCount returns the total number of lines in the buffer.
func (b *TerminalBuffer) LineCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.lines)
}

// Messages

// terminalOutputMsg is sent when new PTY output is available.
type terminalOutputMsg struct {
	sessionID string
	data      []byte
}

// terminalTickMsg triggers periodic PTY polling.
type terminalTickMsg struct {
	sessionID string
	time      time.Time
}

// NewTermViewModel creates a new terminal view model.
func NewTermViewModel(sessionID, sessionName string, pty io.ReadWriteCloser) TermViewModel {
	buffer := NewTerminalBuffer(1000)
	vp := viewport.New(80, 24)
	vp.MouseWheelEnabled = true
	vp.KeyMap = viewport.KeyMap{
		PageDown:     viewport.DefaultKeyMap().PageDown,
		PageUp:       viewport.DefaultKeyMap().PageUp,
		HalfPageUp:   viewport.DefaultKeyMap().HalfPageUp,
		HalfPageDown: viewport.DefaultKeyMap().HalfPageDown,
		Down:         viewport.DefaultKeyMap().Down,
		Up:           viewport.DefaultKeyMap().Up,
	}

	return TermViewModel{
		pty:         pty,
		buffer:      buffer,
		viewport:    vp,
		sessionID:   sessionID,
		sessionName: sessionName,
		borderColor: ColorBlue,
		lastUpdate:  time.Now(),
	}
}

// Init initializes the terminal view model.
func (m TermViewModel) Init() tea.Cmd {
	if m.pty != nil {
		return tea.Batch(
			m.pollPTY(),
			m.tick(),
		)
	}
	return nil
}

// Update handles messages for the terminal view.
func (m TermViewModel) Update(msg tea.Msg) (TermViewModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.focused {
			// In focus mode, handle special keys
			switch msg.String() {
			case "ctrl+]":
				// Exit focus mode
				m.focused = false
				m.borderColor = ColorBlue
				return m, nil

			case "ctrl+c":
				// Allow Ctrl+C to exit even in focus mode
				return m, tea.Quit

			default:
				// Forward all other keystrokes directly to PTY
				if m.pty != nil {
					// Convert key to bytes and send to PTY
					keyBytes := keyToBytes(msg)
					_, _ = m.pty.Write(keyBytes)
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.viewportInit {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.viewport.MouseWheelEnabled = true
			m.viewportInit = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height
		}
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport content when size changes
		m.updateViewportContent()
		return m, nil

	case terminalOutputMsg:
		if msg.sessionID == m.sessionID {
			// Append new data to buffer
			m.buffer.Append(msg.data)

			// Update viewport content
			m.updateViewportContent()

			// Auto-scroll to bottom if we're already at/near the bottom
			if m.viewport.AtBottom() || m.viewport.ScrollPercent() > 0.95 {
				m.viewport.GotoBottom()
			}

			m.lastUpdate = time.Now()
		}
		return m, nil

	case terminalTickMsg:
		if msg.sessionID == m.sessionID {
			// Continue polling
			return m, tea.Batch(m.pollPTY(), m.tick())
		}
		return m, nil
	}

	// Update viewport (for scrolling)
	if !m.focused {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the terminal view.
func (m TermViewModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Build header
	title := "Terminal"
	if m.sessionName != "" {
		title = "Terminal: " + m.sessionName
	}
	if m.focused {
		title += " [FOCUS MODE - Ctrl+] to exit]"
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.borderColor).
		Padding(0, 1)
	header := headerStyle.Render(title)

	// Render viewport
	viewportContent := m.viewport.View()

	// Build footer with status
	footer := m.renderFooter()

	// Wrap in border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.borderColor).
		Padding(0, 1)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		viewportContent,
		footer,
	)

	return borderStyle.Render(content)
}

// updateViewportContent updates the viewport with current buffer content.
func (m *TermViewModel) updateViewportContent() {
	lines := m.buffer.GetAllLines()
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
}

// renderFooter renders the footer status line.
func (m TermViewModel) renderFooter() string {
	lineCount := m.buffer.LineCount()
	scrollPct := int(m.viewport.ScrollPercent() * 100)

	status := lipgloss.NewStyle().
		Foreground(ColorGray).
		Render(
			fmt.Sprintf(
				"%d lines | %d%% | %s",
				lineCount,
				scrollPct,
				m.lastUpdate.Format("15:04:05"),
			),
		)

	helpText := ""
	if m.focused {
		helpText = lipgloss.NewStyle().
			Foreground(ColorYellow).
			Render("Ctrl+] to exit focus")
	} else {
		helpText = lipgloss.NewStyle().
			Foreground(ColorGray).
			Render("↑↓/pgup/pgdn to scroll")
	}

	// Pad to width
	padding := ""
	statusWidth := lipgloss.Width(status)
	helpWidth := lipgloss.Width(helpText)
	if m.width > statusWidth+helpWidth+4 {
		padding = strings.Repeat(" ", m.width-statusWidth-helpWidth-4)
	}

	return status + padding + helpText
}

// SetFocused sets the focus state of the terminal view.
func (m *TermViewModel) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.borderColor = ColorCyan
	} else {
		m.borderColor = ColorBlue
	}
}

// IsFocused returns whether the terminal view is focused.
func (m TermViewModel) IsFocused() bool {
	return m.focused
}

// SetPTY sets the PTY connection for the terminal view.
func (m *TermViewModel) SetPTY(pty io.ReadWriteCloser) {
	m.pty = pty
}

// Commands

// pollPTY polls the PTY for new output.
func (m TermViewModel) pollPTY() tea.Cmd {
	if m.pty == nil {
		return nil
	}

	sessionID := m.sessionID

	return func() tea.Msg {
		// Non-blocking read from PTY
		buf := make([]byte, 4096)

		// Set read deadline for non-blocking behavior
		if f, ok := m.pty.(*os.File); ok {
			_ = f.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		}

		n, err := m.pty.Read(buf)
		if err != nil && err != io.EOF {
			// Ignore timeout/temporary errors
			if !isTemporaryError(err) {
				return nil
			}
		}

		if n > 0 {
			return terminalOutputMsg{
				sessionID: sessionID,
				data:      buf[:n],
			}
		}

		return nil
	}
}

// tick creates a periodic tick for polling.
func (m TermViewModel) tick() tea.Cmd {
	sessionID := m.sessionID
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return terminalTickMsg{
			sessionID: sessionID,
			time:      t,
		}
	})
}

// Helper functions

// keyToBytes converts a tea.KeyMsg to bytes for PTY input.
func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(msg.String())
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyEnter:
		return []byte("\n")
	case tea.KeyTab:
		return []byte("\t")
	case tea.KeyBackspace:
		return []byte{0x7f} // DEL
	case tea.KeyDelete:
		return []byte{0x1b, 0x5b, 0x33, 0x7e} // ESC [ 3 ~
	case tea.KeyUp:
		return []byte{0x1b, 0x5b, 0x41} // ESC [ A
	case tea.KeyDown:
		return []byte{0x1b, 0x5b, 0x42} // ESC [ B
	case tea.KeyRight:
		return []byte{0x1b, 0x5b, 0x43} // ESC [ C
	case tea.KeyLeft:
		return []byte{0x1b, 0x5b, 0x44} // ESC [ D
	case tea.KeyHome:
		return []byte{0x1b, 0x5b, 0x48} // ESC [ H
	case tea.KeyEnd:
		return []byte{0x1b, 0x5b, 0x46} // ESC [ F
	case tea.KeyPgUp:
		return []byte{0x1b, 0x5b, 0x35, 0x7e} // ESC [ 5 ~
	case tea.KeyPgDown:
		return []byte{0x1b, 0x5b, 0x36, 0x7e} // ESC [ 6 ~
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyCtrlZ:
		return []byte{0x1a}
	default:
		// Try to extract the rune
		if len(msg.Runes) > 0 {
			return []byte(string(msg.Runes))
		}
		return []byte(msg.String())
	}
}

// isTemporaryError checks if an error is temporary (timeout, etc.).
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	// Check for timeout or temporary error
	type temporary interface {
		Temporary() bool
	}
	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}
	return false
}
