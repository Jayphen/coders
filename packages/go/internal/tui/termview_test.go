package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockPTY is a mock PTY for testing.
type mockPTY struct {
	*bytes.Buffer
	readBuf *bytes.Buffer
	closed  bool
}

func newMockPTY() *mockPTY {
	return &mockPTY{
		Buffer:  &bytes.Buffer{},
		readBuf: &bytes.Buffer{},
	}
}

func (m *mockPTY) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuf.Read(p)
}

func (m *mockPTY) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.Buffer.Write(p)
}

func (m *mockPTY) Close() error {
	m.closed = true
	return nil
}

func (m *mockPTY) AddOutput(s string) {
	m.readBuf.WriteString(s)
}

func TestTerminalBuffer_Append(t *testing.T) {
	buffer := NewTerminalBuffer(100)

	// Test simple append
	buffer.Append([]byte("line 1\n"))
	buffer.Append([]byte("line 2\n"))
	buffer.Append([]byte("line 3\n"))

	lines := buffer.GetAllLines()
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line 1" {
		t.Errorf("expected 'line 1', got '%s'", lines[0])
	}
	if lines[1] != "line 2" {
		t.Errorf("expected 'line 2', got '%s'", lines[1])
	}
	if lines[2] != "line 3" {
		t.Errorf("expected 'line 3', got '%s'", lines[2])
	}
}

func TestTerminalBuffer_AppendIncomplete(t *testing.T) {
	buffer := NewTerminalBuffer(100)

	// Test incomplete line (no newline)
	buffer.Append([]byte("partial"))
	lines := buffer.GetAllLines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for incomplete data, got %d", len(lines))
	}

	// Complete the line
	buffer.Append([]byte(" line\n"))
	lines = buffer.GetAllLines()
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "partial line" {
		t.Errorf("expected 'partial line', got '%s'", lines[0])
	}
}

func TestTerminalBuffer_MaxLines(t *testing.T) {
	buffer := NewTerminalBuffer(5)

	// Add more lines than max
	for i := 1; i <= 10; i++ {
		buffer.Append([]byte("line " + string(rune('0'+i)) + "\n"))
	}

	lines := buffer.GetAllLines()
	if len(lines) != 5 {
		t.Errorf("expected 5 lines (max), got %d", len(lines))
	}

	// Should keep the last 5 lines
	if !strings.HasPrefix(lines[0], "line 6") {
		t.Errorf("expected first line to be 'line 6...', got '%s'", lines[0])
	}
}

func TestTerminalBuffer_GetLines(t *testing.T) {
	buffer := NewTerminalBuffer(100)

	for i := 1; i <= 10; i++ {
		buffer.Append([]byte("line " + string(rune('0'+i)) + "\n"))
	}

	// Get last 3 lines
	lines := buffer.GetLines(3)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Should be lines 8, 9, 10
	if !strings.HasPrefix(lines[0], "line 8") {
		t.Errorf("expected first line to be 'line 8', got '%s'", lines[0])
	}
}

func TestTerminalBuffer_Clear(t *testing.T) {
	buffer := NewTerminalBuffer(100)

	buffer.Append([]byte("line 1\n"))
	buffer.Append([]byte("line 2\n"))

	buffer.Clear()

	lines := buffer.GetAllLines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines after clear, got %d", len(lines))
	}
}

func TestTermViewModel_Init(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	cmd := model.Init()
	if cmd == nil {
		t.Error("expected Init to return a command")
	}
}

func TestTermViewModel_Update_KeyForwarding(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)
	model.SetFocused(true)

	// Send a key message
	keyMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("a"),
	}

	model, cmd := model.Update(keyMsg)
	_ = cmd

	// Check that the key was written to PTY
	written := pty.String()
	if written != "a" {
		t.Errorf("expected 'a' to be written to PTY, got '%s'", written)
	}
}

func TestTermViewModel_Update_ExitFocus(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)
	model.SetFocused(true)

	if !model.IsFocused() {
		t.Error("expected model to be focused")
	}

	// Test that SetFocused(false) exits focus mode
	model.SetFocused(false)

	if model.IsFocused() {
		t.Error("expected model to not be focused after SetFocused(false)")
	}
}

func TestTermViewModel_Update_OutputMsg(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	// Send output message
	outputMsg := terminalOutputMsg{
		sessionID: "test-session",
		data:      []byte("test output\n"),
	}

	model, _ = model.Update(outputMsg)

	// Check that buffer was updated
	lines := model.buffer.GetAllLines()
	if len(lines) != 1 {
		t.Errorf("expected 1 line in buffer, got %d", len(lines))
	}
	if lines[0] != "test output" {
		t.Errorf("expected 'test output', got '%s'", lines[0])
	}
}

func TestTermViewModel_SetFocused(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	// Initially not focused
	if model.IsFocused() {
		t.Error("expected model to not be focused initially")
	}

	// Set focused
	model.SetFocused(true)
	if !model.IsFocused() {
		t.Error("expected model to be focused")
	}

	// Border color should change
	if model.borderColor != ColorCyan {
		t.Error("expected border color to be cyan when focused")
	}

	// Unfocus
	model.SetFocused(false)
	if model.IsFocused() {
		t.Error("expected model to not be focused")
	}

	if model.borderColor != ColorBlue {
		t.Error("expected border color to be blue when not focused")
	}
}

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name     string
		keyMsg   tea.KeyMsg
		expected []byte
	}{
		{
			name:     "simple rune",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
			expected: []byte("a"),
		},
		{
			name:     "space",
			keyMsg:   tea.KeyMsg{Type: tea.KeySpace},
			expected: []byte(" "),
		},
		{
			name:     "enter",
			keyMsg:   tea.KeyMsg{Type: tea.KeyEnter},
			expected: []byte("\n"),
		},
		{
			name:     "tab",
			keyMsg:   tea.KeyMsg{Type: tea.KeyTab},
			expected: []byte("\t"),
		},
		{
			name:     "backspace",
			keyMsg:   tea.KeyMsg{Type: tea.KeyBackspace},
			expected: []byte{0x7f},
		},
		{
			name:     "up arrow",
			keyMsg:   tea.KeyMsg{Type: tea.KeyUp},
			expected: []byte{0x1b, 0x5b, 0x41},
		},
		{
			name:     "ctrl+c",
			keyMsg:   tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: []byte{0x03},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyToBytes(tt.keyMsg)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTermViewModel_View(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test-name", pty)

	// Set dimensions
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	model, _ = model.Update(sizeMsg)

	// Add some output
	outputMsg := terminalOutputMsg{
		sessionID: "test-session",
		data:      []byte("test line\n"),
	}
	model, _ = model.Update(outputMsg)

	// Render view
	view := model.View()

	// Check that view contains session name
	if !strings.Contains(view, "test-name") {
		t.Error("expected view to contain session name")
	}

	// Check that view contains content
	if !strings.Contains(view, "test line") {
		t.Error("expected view to contain output content")
	}
}

func TestTermViewModel_Concurrency(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	// Simulate concurrent output appends
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			msg := terminalOutputMsg{
				sessionID: "test-session",
				data:      []byte("line " + string(rune('0'+n)) + "\n"),
			}
			model.Update(msg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that all lines were added
	lines := model.buffer.GetAllLines()
	if len(lines) < 10 {
		t.Errorf("expected at least 10 lines, got %d", len(lines))
	}
}

func TestTermViewModel_PollingBehavior(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	// Add data to mock PTY
	pty.AddOutput("test output\n")

	// Simulate polling
	cmd := model.pollPTY()
	if cmd == nil {
		t.Error("expected pollPTY to return a command")
	}

	// Execute the command to read from PTY
	msg := cmd()

	// Should get output message
	if outputMsg, ok := msg.(terminalOutputMsg); ok {
		if outputMsg.sessionID != "test-session" {
			t.Errorf("expected session ID 'test-session', got '%s'", outputMsg.sessionID)
		}
		if string(outputMsg.data) != "test output\n" {
			t.Errorf("expected 'test output\\n', got '%s'", string(outputMsg.data))
		}
	} else {
		t.Errorf("expected terminalOutputMsg, got %T", msg)
	}
}

func TestTermViewModel_Tick(t *testing.T) {
	pty := newMockPTY()
	model := NewTermViewModel("test-session", "test", pty)

	cmd := model.tick()
	if cmd == nil {
		t.Error("expected tick to return a command")
	}

	// Wait a bit for tick
	time.Sleep(600 * time.Millisecond)

	// Execute tick command
	msg := cmd()

	// Should get tick message
	if tickMsg, ok := msg.(terminalTickMsg); ok {
		if tickMsg.sessionID != "test-session" {
			t.Errorf("expected session ID 'test-session', got '%s'", tickMsg.sessionID)
		}
	} else {
		t.Errorf("expected terminalTickMsg, got %T", msg)
	}
}
