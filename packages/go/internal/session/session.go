package session

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Session represents a PTY session with a running process
type Session struct {
	ID        string
	Name      string
	Tool      string
	Task      string
	Cwd       string
	CreatedAt time.Time
	ExitedAt  *time.Time
	ExitCode  *int

	// PTY components
	pty    *os.File
	cmd    *exec.Cmd
	output *OutputBuffer

	// Metadata
	metadata map[string]string
	mu       sync.RWMutex
}

// OutputBuffer maintains a scrollback buffer of terminal output
type OutputBuffer struct {
	lines       []string
	partialLine string // Incomplete line from previous append
	maxLines    int
	mu          sync.RWMutex
}

// NewOutputBuffer creates a new output buffer with the specified max lines
func NewOutputBuffer(maxLines int) *OutputBuffer {
	return &OutputBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

// Append adds new data to the buffer, parsing ANSI codes and splitting into lines
func (b *OutputBuffer) Append(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Strip ANSI escape codes for cleaner storage
	stripped := ansi.Strip(string(data))
	text := b.partialLine + string(stripped)

	// Handle different line ending styles (\n, \r\n, \r)
	// Replace \r\n with \n first
	text = strings.ReplaceAll(text, "\r\n", "\n")
	// Replace remaining \r with \n
	text = strings.ReplaceAll(text, "\r", "\n")

	// Split on newlines
	parts := strings.Split(text, "\n")

	// If text ends with a newline, the last element will be empty
	// and we have no partial line. Otherwise, the last element is partial.
	if len(parts) > 0 && text[len(text)-1] != '\n' {
		// Last part is partial - save it for next append
		b.partialLine = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	} else {
		// No partial line
		b.partialLine = ""
		// Remove the empty string at the end from split
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}

	// Append complete lines
	b.lines = append(b.lines, parts...)

	// Trim to maxLines if exceeded
	if len(b.lines) > b.maxLines {
		b.lines = b.lines[len(b.lines)-b.maxLines:]
	}
}

// GetLines returns the last n lines from the buffer
func (b *OutputBuffer) GetLines(n int) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 {
		return []string{}
	}

	if n > len(b.lines) {
		n = len(b.lines)
	}

	result := make([]string, n)
	copy(result, b.lines[len(b.lines)-n:])
	return result
}

// GetAllLines returns all lines in the buffer
func (b *OutputBuffer) GetAllLines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// Clear clears the buffer
func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = make([]string, 0, b.maxLines)
	b.partialLine = ""
}

// IsRunning returns true if the session process is still running
func (s *Session) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ExitedAt == nil
}

// GetMetadata returns a copy of the session metadata
func (s *Session) GetMetadata() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string, len(s.metadata))
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// SetMetadata sets a metadata key-value pair
func (s *Session) SetMetadata(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.metadata == nil {
		s.metadata = make(map[string]string)
	}
	s.metadata[key] = value
}

// Write writes data to the PTY (sends input to the running process)
func (s *Session) Write(data []byte) (int, error) {
	s.mu.RLock()
	pty := s.pty
	s.mu.RUnlock()

	if pty == nil {
		return 0, io.ErrClosedPipe
	}

	return pty.Write(data)
}

// Close cleanly shuts down the session
func (s *Session) Close() error {
	s.mu.Lock()

	// Close PTY first to signal EOF to the process
	if s.pty != nil {
		s.pty.Close()
		s.pty = nil
	}

	// Check if process is still running
	if s.cmd != nil && s.cmd.Process != nil && s.ExitedAt == nil {
		// Kill the process if it's still running
		if err := s.cmd.Process.Kill(); err != nil {
			s.mu.Unlock()
			return err
		}

		// Mark as exited
		now := time.Now()
		s.ExitedAt = &now
		killExitCode := -1
		s.ExitCode = &killExitCode
	}

	s.mu.Unlock()
	return nil
}
