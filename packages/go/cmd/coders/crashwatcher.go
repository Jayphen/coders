package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/logging"
	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/types"
)

var (
	crashWatcherSessionID string
)

func newCrashWatcherCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "crash-watcher",
		Short:  "Monitor a session for crashes and restart if needed",
		Hidden: true, // Internal command, started by spawn
		Long: `Run a background crash watcher that monitors a tmux session.
If the session crashes or the CLI process dies unexpectedly, it will
automatically restart the session with the same task/prompt.

This is typically started automatically by 'coders spawn' when --restart-on-crash is enabled.`,
		RunE: runCrashWatcher,
	}

	cmd.Flags().StringVar(&crashWatcherSessionID, "session", "", "Session ID to watch")

	return cmd
}

func runCrashWatcher(cmd *cobra.Command, args []string) error {
	log := logging.WithCommand("crash-watcher")

	sessionID := crashWatcherSessionID
	if sessionID == "" {
		sessionID = os.Getenv("CODERS_SESSION_ID")
	}
	if sessionID == "" {
		log.Error("session ID required")
		return fmt.Errorf("session ID required (use --session or CODERS_SESSION_ID env)")
	}

	log = log.WithSessionID(sessionID)

	// Connect to Redis
	redisClient, err := redis.NewClient()
	if err != nil {
		log.WithError(err).Error("failed to connect to Redis")
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	// Get session state from Redis
	ctx := context.Background()
	state, err := redisClient.GetSessionState(ctx, sessionID)
	if err != nil {
		log.WithError(err).Error("failed to get session state")
		return fmt.Errorf("failed to get session state: %w", err)
	}
	if state == nil {
		log.Error("no session state found")
		return fmt.Errorf("no session state found for %s", sessionID)
	}

	log.WithFields(map[string]interface{}{
		"max_restarts":     state.MaxRestarts,
		"current_restarts": state.RestartCount,
	}).Info("crash watcher started")
	fmt.Printf("[CrashWatcher] Started for session: %s\n", sessionID)
	fmt.Printf("[CrashWatcher] Max restarts: %d, Current restarts: %d\n", state.MaxRestarts, state.RestartCount)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Check interval
	checkInterval := 5 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Consecutive failures counter for debouncing
	consecutiveFailures := 0
	failureThreshold := 2 // Require 2 consecutive failures before restart

	for {
		select {
		case <-ticker.C:
			crashed, reason := checkSessionCrashed(sessionID)
			if crashed {
				consecutiveFailures++
				log.WithFields(map[string]interface{}{
					"consecutive_failures": consecutiveFailures,
					"threshold":            failureThreshold,
					"reason":               reason,
				}).Warn("session appears crashed")
				fmt.Printf("[CrashWatcher] Session appears crashed (check %d/%d): %s\n",
					consecutiveFailures, failureThreshold, reason)

				if consecutiveFailures >= failureThreshold {
					log.WithField("reason", reason).Error("session confirmed crashed")
					fmt.Printf("[CrashWatcher] Session confirmed crashed: %s\n", reason)

					// Record crash event
					crashEvent := &types.CrashEvent{
						SessionID:   sessionID,
						Timestamp:   time.Now().UnixMilli(),
						Reason:      reason,
						WillRestart: state.RestartCount < state.MaxRestarts,
					}
					if err := redisClient.RecordCrashEvent(ctx, crashEvent); err != nil {
						fmt.Printf("[CrashWatcher] Failed to record crash event: %v\n", err)
					}

					// Check if we can restart
					if state.RestartCount >= state.MaxRestarts {
						log.WithField("max_restarts", state.MaxRestarts).Warn("max restarts reached, not restarting")
						fmt.Printf("[CrashWatcher] Max restarts (%d) reached, not restarting\n", state.MaxRestarts)
						// Clean up session state
						if err := redisClient.DeleteSessionState(ctx, sessionID); err != nil {
							log.WithError(err).Warn("failed to delete session state")
							fmt.Printf("[CrashWatcher] Failed to delete session state: %v\n", err)
						}
						return nil
					}

					// Attempt restart
					log.WithFields(map[string]interface{}{
						"restart_attempt": state.RestartCount + 1,
						"max_restarts":    state.MaxRestarts,
					}).Info("attempting restart")
					fmt.Printf("[CrashWatcher] Attempting restart %d/%d...\n",
						state.RestartCount+1, state.MaxRestarts)

					if err := restartSession(redisClient, state); err != nil {
						log.WithError(err).Error("failed to restart session")
						fmt.Printf("[CrashWatcher] Failed to restart session: %v\n", err)
						return err
					}

					log.Info("session restarted successfully")
					fmt.Printf("[CrashWatcher] Session restarted successfully\n")
					consecutiveFailures = 0

					// Refresh state from Redis (restart count updated)
					state, err = redisClient.GetSessionState(ctx, sessionID)
					if err != nil || state == nil {
						fmt.Printf("[CrashWatcher] Failed to refresh session state, exiting\n")
						return nil
					}
				}
			} else {
				// Reset failure counter on successful check
				if consecutiveFailures > 0 {
					fmt.Printf("[CrashWatcher] Session recovered, resetting failure counter\n")
				}
				consecutiveFailures = 0
			}

		case sig := <-sigChan:
			fmt.Printf("\n[CrashWatcher] Received %v, shutting down...\n", sig)
			return nil
		}
	}
}

// checkSessionCrashed checks if a session has crashed.
// Returns (crashed, reason).
func checkSessionCrashed(sessionID string) (bool, string) {
	// Check if tmux session still exists
	if !tmux.SessionExists(sessionID) {
		return true, "tmux session no longer exists"
	}

	// Get pane PIDs
	pids, err := tmux.GetPanePIDs(sessionID)
	if err != nil || len(pids) == 0 {
		return true, "no pane PIDs found"
	}

	// Check if any CLI process is running in the pane
	for _, pid := range pids {
		// Check if the process is alive
		if processExists(pid) {
			// Check child processes for CLI tools
			childOut, err := exec.Command("pgrep", "-P", fmt.Sprintf("%d", pid)).Output()
			if err == nil && len(childOut) > 0 {
				// There are child processes, check if any is a CLI tool
				children := strings.Split(strings.TrimSpace(string(childOut)), "\n")
				for _, childPID := range children {
					if childPID == "" {
						continue
					}
					procOut, err := exec.Command("ps", "-p", childPID, "-o", "comm=").Output()
					if err != nil {
						continue
					}
					procName := strings.TrimSpace(string(procOut))
					// Check for known CLI tools
					if isKnownCLIProcess(procName) {
						return false, "" // Session is healthy
					}
				}
			}

			// Check if the pane process itself is a CLI tool
			procOut, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=").Output()
			if err == nil {
				procName := strings.TrimSpace(string(procOut))
				if isKnownCLIProcess(procName) {
					return false, "" // Session is healthy
				}
			}

			// Pane exists but no CLI running - might be in shell fallback
			// Check pane content for signs of crash
			out, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionID, "-S", "-20").Output()
			if err == nil {
				content := string(out)
				// Look for common crash indicators
				crashIndicators := []string{
					"error:",
					"Error:",
					"panic:",
					"fatal:",
					"FATAL:",
					"Segmentation fault",
					"Killed",
					"OOM",
					"command not found",
				}
				for _, indicator := range crashIndicators {
					if strings.Contains(content, indicator) {
						return true, fmt.Sprintf("crash indicator found: %s", indicator)
					}
				}
			}

			// Check if shell prompt is visible (indicates CLI exited)
			if out, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionID, "-S", "-5").Output(); err == nil {
				content := strings.TrimSpace(string(out))
				lines := strings.Split(content, "\n")
				if len(lines) > 0 {
					lastLine := strings.TrimSpace(lines[len(lines)-1])
					// Common shell prompts
					if strings.HasSuffix(lastLine, "$") ||
						strings.HasSuffix(lastLine, "#") ||
						strings.HasSuffix(lastLine, "%") ||
						strings.HasSuffix(lastLine, ">") {
						return true, "shell prompt detected (CLI exited)"
					}
				}
			}
		}
	}

	return false, ""
}

// processExists checks if a process with the given PID exists.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isKnownCLIProcess checks if a process name is a known AI CLI tool.
func isKnownCLIProcess(name string) bool {
	knownCLIs := []string{"claude", "gemini", "codex", "opencode", "node", "python", "ruby"}
	for _, cli := range knownCLIs {
		if strings.Contains(strings.ToLower(name), cli) {
			return true
		}
	}
	return false
}

// restartSession restarts a crashed session using its stored state.
func restartSession(redisClient *redis.Client, state *types.SessionState) error {
	ctx := context.Background()

	// Kill any remaining processes in the old session
	if tmux.SessionExists(state.SessionID) {
		_ = tmux.KillSession(state.SessionID)
		time.Sleep(500 * time.Millisecond) // Give tmux time to clean up
	}

	// Get user's shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Build the tool command
	toolCmd := buildToolCommand(state.Tool, state.Task, state.Model, state.SessionID, state.UseOllama)

	// Create prompt file if needed
	promptFile := ""
	if state.Task != "" && (state.Tool == "claude" || state.Tool == "codex" || state.Tool == "opencode") {
		promptFile = fmt.Sprintf("/tmp/coders-prompt-%d.txt", time.Now().UnixNano())
		prompt := buildRestartPrompt(state.Tool, state.Task, state.RestartCount+1)
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			return fmt.Errorf("failed to write prompt file: %w", err)
		}
	}

	// Build full tmux command
	var fullCmd string
	if promptFile != "" {
		fullCmd = fmt.Sprintf("cd %s && %s < %s; exec %s",
			shellEscape(state.Cwd), toolCmd, promptFile, shell)
	} else {
		fullCmd = fmt.Sprintf("cd %s && %s; exec %s",
			shellEscape(state.Cwd), toolCmd, shell)
	}

	// Create tmux session
	tmuxArgs := []string{"new-session", "-d", "-s", state.SessionID, "-c", state.Cwd, "sh", "-c", fullCmd}
	createCmd := exec.Command("tmux", tmuxArgs...)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Update session state in Redis
	state.RestartCount++
	state.LastRestartAt = time.Now().UnixMilli()
	if err := redisClient.SetSessionState(ctx, state); err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	// Wait for CLI to be ready
	if ready := waitForCLIReady(state.SessionID, state.Tool, 30*time.Second); !ready {
		fmt.Printf("[CrashWatcher] Warning: timeout waiting for CLI to start\n")
	}

	// Restart heartbeat if it was enabled
	if state.HeartbeatEnabled {
		if err := startHeartbeat(state.SessionID, state.Task, ""); err != nil {
			fmt.Printf("[CrashWatcher] Warning: failed to start heartbeat: %v\n", err)
		}
	}

	return nil
}

// buildRestartPrompt creates a prompt that indicates this is a restart.
func buildRestartPrompt(tool, task string, restartCount int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("TASK: %s\n\n", task))
	b.WriteString(fmt.Sprintf("NOTE: This is restart #%d. The previous session crashed unexpectedly.\n", restartCount))
	b.WriteString("Please continue working on the task. Check git status to see what was done previously.\n\n")
	b.WriteString("You have full permissions. Complete the task.\n\n")
	b.WriteString("IMPORTANT: When you finish this task, you MUST publish a completion promise.\n")

	if tool == "codex" {
		b.WriteString("Run this shell command: coders promise \"Brief summary of what you accomplished\"\n")
		b.WriteString("\nThis notifies the orchestrator and dashboard that your work is complete.\n")
		b.WriteString("If you get blocked, use: coders promise \"Reason for being blocked\" --status blocked\n")
	} else {
		b.WriteString("/coders:promise \"Brief summary of what you accomplished\"\n")
		b.WriteString("\nThis notifies the orchestrator and dashboard that your work is complete.\n")
		b.WriteString("If you get blocked, use: /coders:promise \"Reason for being blocked\" --status blocked\n")
	}

	return b.String()
}

// startCrashWatcher starts a background crash watcher process for a session.
func startCrashWatcher(sessionID string) error {
	// Get the path to this executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build crash watcher command args
	args := []string{"crash-watcher", "--session", sessionID}

	// Start crash watcher as a background process
	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start crash watcher: %w", err)
	}

	// Don't wait for it - let it run in background
	go func() {
		cmd.Wait()
	}()

	return nil
}

// storeSessionState saves the session state to Redis for crash recovery.
func storeSessionState(sessionID, sessionName, tool, task, cwd, model string, useOllama, heartbeatEnabled, restartOnCrash bool, maxRestarts int) error {
	// Load config to check if Redis is available
	cfg, err := config.Get()
	if err != nil || cfg.RedisURL == "" {
		return fmt.Errorf("Redis configuration not available")
	}

	// Connect to Redis
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	state := &types.SessionState{
		SessionID:        sessionID,
		SessionName:      sessionName,
		Tool:             tool,
		Task:             task,
		Cwd:              cwd,
		Model:            model,
		UseOllama:        useOllama,
		HeartbeatEnabled: heartbeatEnabled,
		RestartOnCrash:   restartOnCrash,
		RestartCount:     0,
		MaxRestarts:      maxRestarts,
		CreatedAt:        time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return redisClient.SetSessionState(ctx, state)
}
