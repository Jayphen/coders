package session

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestOutputBuffer(t *testing.T) {
	t.Run("basic append and retrieve", func(t *testing.T) {
		buf := NewOutputBuffer(10)

		buf.Append([]byte("line 1\n"))
		buf.Append([]byte("line 2\n"))
		buf.Append([]byte("line 3\n"))

		lines := buf.GetAllLines()
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}

		if lines[0] != "line 1" {
			t.Errorf("expected 'line 1', got '%s'", lines[0])
		}
	})

	t.Run("max lines trimming", func(t *testing.T) {
		buf := NewOutputBuffer(3)

		for i := 1; i <= 5; i++ {
			buf.Append([]byte("line " + string(rune('0'+i)) + "\n"))
		}

		lines := buf.GetAllLines()
		if len(lines) != 3 {
			t.Errorf("expected 3 lines (max), got %d", len(lines))
		}

		// Should contain lines 3, 4, 5
		if lines[0] != "line 3" {
			t.Errorf("expected 'line 3', got '%s'", lines[0])
		}
	})

	t.Run("get last N lines", func(t *testing.T) {
		buf := NewOutputBuffer(10)

		buf.Append([]byte("line 1\nline 2\nline 3\nline 4\nline 5\n"))

		lines := buf.GetLines(2)
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}

		if lines[0] != "line 4" || lines[1] != "line 5" {
			t.Errorf("expected lines 4 and 5, got %v", lines)
		}
	})

	t.Run("partial lines", func(t *testing.T) {
		buf := NewOutputBuffer(10)

		buf.Append([]byte("partial"))
		buf.Append([]byte(" line\n"))

		lines := buf.GetAllLines()
		if len(lines) != 1 {
			t.Errorf("expected 1 line, got %d", len(lines))
		}

		if lines[0] != "partial line" {
			t.Errorf("expected 'partial line', got '%s'", lines[0])
		}
	})

	t.Run("clear buffer", func(t *testing.T) {
		buf := NewOutputBuffer(10)

		buf.Append([]byte("line 1\nline 2\n"))
		buf.Clear()

		lines := buf.GetAllLines()
		if len(lines) != 0 {
			t.Errorf("expected 0 lines after clear, got %d", len(lines))
		}
	})
}

func TestSession(t *testing.T) {
	t.Run("session metadata", func(t *testing.T) {
		session := &Session{
			ID:        "test-id",
			metadata:  make(map[string]string),
			CreatedAt: time.Now(),
		}

		session.SetMetadata("key1", "value1")
		session.SetMetadata("key2", "value2")

		meta := session.GetMetadata()
		if meta["key1"] != "value1" {
			t.Errorf("expected 'value1', got '%s'", meta["key1"])
		}
		if meta["key2"] != "value2" {
			t.Errorf("expected 'value2', got '%s'", meta["key2"])
		}
	})

	t.Run("session running state", func(t *testing.T) {
		session := &Session{
			ID:        "test-id",
			CreatedAt: time.Now(),
		}

		if !session.IsRunning() {
			t.Error("new session should be running")
		}

		now := time.Now()
		session.ExitedAt = &now

		if session.IsRunning() {
			t.Error("session should not be running after exit")
		}
	})
}

func TestManager(t *testing.T) {
	t.Run("create and list sessions", func(t *testing.T) {
		mgr := NewManager()
		defer mgr.Close()

		// Create a session with a simple command
		session, err := mgr.CreateSession("echo", "test task", "")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		if session.ID == "" {
			t.Error("session ID should not be empty")
		}

		if session.Tool != "echo" {
			t.Errorf("expected tool 'echo', got '%s'", session.Tool)
		}

		// List sessions
		sessions := mgr.ListSessions()
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}

		// Wait for process to complete
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("get session by ID", func(t *testing.T) {
		mgr := NewManager()
		defer mgr.Close()

		session, err := mgr.CreateSession("echo", "test", "")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		retrieved, err := mgr.GetSession(session.ID)
		if err != nil {
			t.Fatalf("failed to get session: %v", err)
		}

		if retrieved.ID != session.ID {
			t.Errorf("expected ID '%s', got '%s'", session.ID, retrieved.ID)
		}

		// Try to get non-existent session
		_, err = mgr.GetSession("non-existent")
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})

	t.Run("send keys to session", func(t *testing.T) {
		if os.Getenv("CI") == "true" {
			t.Skip("Skipping interactive test in CI")
		}

		mgr := NewManager()
		defer mgr.Close()

		// Create a session with cat (reads stdin and echoes to stdout)
		session, err := mgr.CreateSession("cat", "echo test", "")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Give the process time to start
		time.Sleep(100 * time.Millisecond)

		// Send some input
		err = mgr.SendKeys(session.ID, "hello world\n")
		if err != nil {
			t.Fatalf("failed to send keys: %v", err)
		}

		// Wait for output to be captured
		time.Sleep(200 * time.Millisecond)

		// Capture output
		output, err := mgr.CaptureOutput(session.ID, 10)
		if err != nil {
			t.Fatalf("failed to capture output: %v", err)
		}

		// Check if our input was echoed back
		found := false
		for _, line := range output {
			if strings.Contains(line, "hello world") {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected to find 'hello world' in output, got: %v", output)
		}
	})

	t.Run("kill session", func(t *testing.T) {
		mgr := NewManager()
		defer mgr.Close()

		// Create a long-running session
		session, err := mgr.CreateSession("sleep", "10", "")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Verify it's in the list
		sessions := mgr.ListSessions()
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}

		// Kill the session
		err = mgr.KillSession(session.ID)
		if err != nil {
			t.Fatalf("failed to kill session: %v", err)
		}

		// Verify it's removed from the list
		sessions = mgr.ListSessions()
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions after kill, got %d", len(sessions))
		}
	})

	t.Run("capture all output", func(t *testing.T) {
		mgr := NewManager()
		defer mgr.Close()

		// Create a session that produces some output
		session, err := mgr.CreateSession("echo", "test output", "")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Wait for the command to complete and output to be captured
		time.Sleep(200 * time.Millisecond)

		// Capture all output
		output, err := mgr.CaptureAllOutput(session.ID)
		if err != nil {
			t.Fatalf("failed to capture all output: %v", err)
		}

		// Echo should have produced some output
		if len(output) == 0 {
			t.Error("expected some output from echo command")
		}
	})

	t.Run("close manager", func(t *testing.T) {
		mgr := NewManager()

		// Create a few sessions
		_, err := mgr.CreateSession("sleep", "10", "")
		if err != nil {
			t.Fatalf("failed to create session 1: %v", err)
		}

		_, err = mgr.CreateSession("sleep", "10", "")
		if err != nil {
			t.Fatalf("failed to create session 2: %v", err)
		}

		// Close the manager
		err = mgr.Close()
		if err != nil {
			t.Fatalf("failed to close manager: %v", err)
		}

		// Verify all sessions are closed
		sessions := mgr.ListSessions()
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions after close, got %d", len(sessions))
		}
	})
}
