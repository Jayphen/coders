// Package tmux provides functions for interacting with tmux sessions.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Jayphen/coders/internal/types"
)

const (
	// SessionPrefix is the prefix for all coder tmux sessions.
	SessionPrefix = "coder-"
	// TUISession is the name of the TUI's own tmux session.
	TUISession = "coders-tui"
)

// IsInsideTmux returns true if we're running inside a tmux session.
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// GetCurrentSession returns the name of the current tmux session, if any.
func GetCurrentSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SessionExists checks if a tmux session with the given name exists.
func SessionExists(name string) bool {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	return err == nil
}

// ListSessions returns all coder sessions (sessions starting with SessionPrefix).
func ListSessions() ([]types.Session, error) {
	// Get session info from tmux
	out, err := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}|#{session_created}|#{pane_current_path}|#{pane_title}").Output()
	if err != nil {
		// No sessions is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []types.Session{}, nil
		}
		return nil, fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	var sessions []types.Session
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Only include coder sessions
		if !strings.HasPrefix(line, SessionPrefix) {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		createdStr := parts[1]
		cwd := parts[2]
		paneTitle := ""
		if len(parts) > 3 {
			paneTitle = parts[3]
		}

		// Parse tool and task from session name (coder-{tool}-{task})
		nameParts := strings.SplitN(strings.TrimPrefix(name, SessionPrefix), "-", 2)
		tool := "unknown"
		taskFromName := ""
		if len(nameParts) > 0 {
			tool = nameParts[0]
			if !types.IsValidTool(tool) {
				tool = "unknown"
			}
		}
		if len(nameParts) > 1 {
			taskFromName = nameParts[1]
		}

		isOrchestrator := name == "coder-orchestrator"

		// Parse creation time
		var createdAt *time.Time
		if ts, err := strconv.ParseInt(createdStr, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			createdAt = &t
		}

		// Determine task description
		task := taskFromName
		if task == "" && paneTitle != "" && !strings.Contains(paneTitle, "bash") &&
			!strings.Contains(paneTitle, "zsh") && paneTitle != name {
			task = paneTitle
		}

		session := types.Session{
			Name:           name,
			Tool:           tool,
			Task:           task,
			Cwd:            cwd,
			CreatedAt:      createdAt,
			IsOrchestrator: isOrchestrator,
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// AttachSession attaches to or switches to a tmux session.
func AttachSession(name string) error {
	if !IsInsideTmux() {
		// Outside tmux - attach directly
		cmd := exec.Command("tmux", "attach", "-t", name)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Inside tmux - switch client
	return exec.Command("tmux", "switch-client", "-t", name).Run()
}

// KillSession kills a tmux session and its process tree.
func KillSession(name string) error {
	// First, kill the process tree
	if err := killSessionProcessTree(name); err != nil {
		// Non-fatal, continue to kill session
	}

	// Kill the tmux session
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// GetPanePIDs returns the PIDs of all panes in a session.
func GetPanePIDs(sessionName string) ([]int, error) {
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_pid}").Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// killSessionProcessTree kills all processes in a session's process tree.
func killSessionProcessTree(sessionName string) error {
	pids, err := GetPanePIDs(sessionName)
	if err != nil || len(pids) == 0 {
		return err
	}

	// Get all descendant PIDs
	descendants := collectDescendants(pids)
	if len(descendants) == 0 {
		return nil
	}

	// Send SIGTERM first
	killPIDs(descendants, "TERM")

	// Wait a bit
	time.Sleep(300 * time.Millisecond)

	// Send SIGKILL to any remaining
	remaining := filterLivePIDs(descendants)
	if len(remaining) > 0 {
		killPIDs(remaining, "KILL")
	}

	return nil
}

// collectDescendants collects all descendant PIDs of the given root PIDs.
func collectDescendants(rootPIDs []int) []int {
	// Get process table
	out, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return rootPIDs
	}

	// Build parent -> children map
	children := make(map[int][]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}

	// BFS to collect all descendants
	visited := make(map[int]bool)
	queue := append([]int{}, rootPIDs...)

	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]

		if visited[pid] {
			continue
		}
		visited[pid] = true

		for _, child := range children[pid] {
			queue = append(queue, child)
		}
	}

	result := make([]int, 0, len(visited))
	for pid := range visited {
		result = append(result, pid)
	}
	return result
}

// killPIDs sends a signal to a list of PIDs.
func killPIDs(pids []int, signal string) {
	if len(pids) == 0 {
		return
	}

	args := []string{"-" + signal}
	for _, pid := range pids {
		args = append(args, strconv.Itoa(pid))
	}

	// Ignore errors - some processes may have already exited
	exec.Command("kill", args...).Run()
}

// filterLivePIDs returns only the PIDs that are still running.
func filterLivePIDs(pids []int) []int {
	out, err := exec.Command("ps", "-axo", "pid=").Output()
	if err != nil {
		return nil
	}

	live := make(map[int]bool)
	for _, line := range strings.Split(string(out), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			live[pid] = true
		}
	}

	var result []int
	for _, pid := range pids {
		if live[pid] {
			result = append(result, pid)
		}
	}
	return result
}

// CreateSession creates a new tmux session with the given name and command.
func CreateSession(name, cwd, command string) error {
	args := []string{"new-session", "-d", "-s", name}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	if command != "" {
		args = append(args, command)
	}

	return exec.Command("tmux", args...).Run()
}

// SendKeys sends keys to a tmux session.
func SendKeys(sessionName, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", sessionName, keys, "Enter").Run()
}
