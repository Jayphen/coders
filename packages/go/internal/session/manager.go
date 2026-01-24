package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/rs/zerolog/log"
)

// Manager manages multiple PTY sessions
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession spawns a new CLI tool with a PTY
func (m *Manager) CreateSession(tool, task, cwd string) (*Session, error) {
	// Generate unique session ID
	id := fmt.Sprintf("coder-%s-%d", tool, time.Now().UnixNano())

	// Create the command
	cmd := exec.Command(tool)
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Inherit current environment and add task variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("CODERS_TASK=%s", task))

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set initial PTY size
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		log.Warn().Err(err).Msg("Failed to set initial PTY size")
	}

	// Create session
	session := &Session{
		ID:        id,
		Name:      fmt.Sprintf("%s-%s", tool, task),
		Tool:      tool,
		Task:      task,
		Cwd:       cwd,
		CreatedAt: time.Now(),
		pty:       ptmx,
		cmd:       cmd,
		output:    NewOutputBuffer(1000),
		metadata:  make(map[string]string),
	}

	// Add to sessions map
	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	// Start background goroutines for this session
	go m.readPTYOutput(session)
	go m.monitorProcess(session)

	log.Info().
		Str("id", id).
		Str("tool", tool).
		Str("task", task).
		Msg("Created new session")

	return session, nil
}

// ListSessions returns all active sessions
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}

	return sessions
}

// GetSession retrieves a specific session by ID
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// KillSession terminates a session
func (m *Manager) KillSession(id string) error {
	session, err := m.GetSession(id)
	if err != nil {
		return err
	}

	log.Info().Str("id", id).Msg("Killing session")

	// Close the session (this handles cleanup)
	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	// Remove from sessions map
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	return nil
}

// SendKeys writes data to the PTY (zero latency)
func (m *Manager) SendKeys(id string, data string) error {
	session, err := m.GetSession(id)
	if err != nil {
		return err
	}

	if !session.IsRunning() {
		return fmt.Errorf("session is not running: %s", id)
	}

	// Write directly to PTY
	_, err = session.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("failed to write to PTY: %w", err)
	}

	return nil
}

// CaptureOutput reads recent PTY output
func (m *Manager) CaptureOutput(id string, lines int) ([]string, error) {
	session, err := m.GetSession(id)
	if err != nil {
		return nil, err
	}

	return session.output.GetLines(lines), nil
}

// CaptureAllOutput returns all buffered output for a session
func (m *Manager) CaptureAllOutput(id string) ([]string, error) {
	session, err := m.GetSession(id)
	if err != nil {
		return nil, err
	}

	return session.output.GetAllLines(), nil
}

// readPTYOutput continuously reads from the PTY and appends to the output buffer
func (m *Manager) readPTYOutput(s *Session) {
	buf := make([]byte, 4096)

	for {
		n, err := s.pty.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Debug().Str("id", s.ID).Msg("PTY output stream closed")
			} else {
				log.Error().Err(err).Str("id", s.ID).Msg("PTY read error")
			}
			return
		}

		if n > 0 {
			s.output.Append(buf[:n])
		}
	}
}

// monitorProcess waits for the process to exit and updates session metadata
func (m *Manager) monitorProcess(s *Session) {
	err := s.cmd.Wait()

	s.mu.Lock()
	now := time.Now()
	s.ExitedAt = &now

	exitCode := 0
	if s.cmd.ProcessState != nil {
		exitCode = s.cmd.ProcessState.ExitCode()
	} else if err != nil {
		// Process was killed or errored without a valid state
		exitCode = -1
	}
	s.ExitCode = &exitCode
	s.mu.Unlock()

	if err != nil {
		log.Info().
			Str("id", s.ID).
			Err(err).
			Int("exit_code", exitCode).
			Msg("Session process exited with error")
	} else {
		log.Info().
			Str("id", s.ID).
			Int("exit_code", exitCode).
			Msg("Session process exited")
	}
}

// Close shuts down all sessions managed by this manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for id, session := range m.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session %s: %w", id, err))
		}
	}

	// Clear the sessions map
	m.sessions = make(map[string]*Session)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing sessions: %v", errs)
	}

	return nil
}
