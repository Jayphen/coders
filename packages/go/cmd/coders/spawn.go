package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/logging"
	"github.com/Jayphen/coders/internal/tmux"
)

var (
	spawnTool           string
	spawnTask           string
	spawnCwd            string
	spawnModel          string
	spawnHeartbeat      bool
	spawnAttach         bool
	spawnOllama         bool
	spawnRestartOnCrash bool
	spawnMaxRestarts    int
)

func newSpawnCmd() *cobra.Command {
	// Load config for defaults
	cfg, _ := config.Get()
	defaultTool := config.DefaultDefaultTool
	defaultHeartbeat := config.DefaultDefaultHeartbeat
	defaultModel := ""
	if cfg != nil {
		defaultTool = cfg.DefaultTool
		defaultHeartbeat = cfg.DefaultHeartbeat
		defaultModel = cfg.DefaultModel
	}

	cmd := &cobra.Command{
		Use:   "spawn [tool]",
		Short: "Spawn a new coder session",
		Long: `Spawn a new AI coding session in tmux.

Supported tools: claude, gemini, codex, opencode

Examples:
  coders spawn claude --task "Fix the login bug"
  coders spawn gemini --task "Add unit tests" --cwd ~/projects/myapp
  coders spawn codex --task "Refactor auth module" --model gpt-4
  coders spawn --attach  # Spawn and attach immediately
  coders spawn --restart-on-crash --task "Long running task"  # Auto-restart on crash

Crash Recovery:
  With --restart-on-crash, the session will automatically restart if the CLI
  process crashes or dies unexpectedly. Session state is stored in Redis so
  it can be restored with the same task/prompt. Use --max-restarts to limit
  the number of automatic restarts (default: 3).`,
		Args: cobra.MaximumNArgs(1),
		RunE: runSpawn,
	}

	cmd.Flags().StringVarP(&spawnTool, "tool", "t", defaultTool, "AI tool to use (claude, gemini, codex, opencode)")
	cmd.Flags().StringVar(&spawnTask, "task", "", "Task description")
	cmd.Flags().StringVar(&spawnCwd, "cwd", "", "Working directory (supports zoxide queries)")
	cmd.Flags().StringVar(&spawnModel, "model", defaultModel, "Model to use (tool-specific)")
	cmd.Flags().BoolVar(&spawnHeartbeat, "heartbeat", defaultHeartbeat, "Enable heartbeat monitoring")
	cmd.Flags().BoolVarP(&spawnAttach, "attach", "a", false, "Attach to session after spawning")
	cmd.Flags().BoolVar(&spawnOllama, "ollama", false, "Use Ollama backend (requires CODERS_OLLAMA_BASE_URL and CODERS_OLLAMA_AUTH_TOKEN)")
	cmd.Flags().BoolVar(&spawnRestartOnCrash, "restart-on-crash", false, "Automatically restart session if it crashes (requires Redis)")
	cmd.Flags().IntVar(&spawnMaxRestarts, "max-restarts", 3, "Maximum number of automatic restarts (default: 3)")

	return cmd
}

func runSpawn(cmd *cobra.Command, args []string) error {
	log := logging.WithCommand("spawn")

	// Get tool from arg or flag
	tool := spawnTool
	if len(args) > 0 {
		tool = args[0]
	}

	log.Debugf("starting spawn with tool=%s, task=%s", tool, spawnTask)

	// Validate tool
	validTools := map[string]bool{
		"claude": true, "gemini": true, "codex": true, "opencode": true,
	}
	if !validTools[tool] {
		log.Errorf("invalid tool: %s", tool)
		return fmt.Errorf("invalid tool '%s': must be claude, gemini, codex, or opencode", tool)
	}

	// Validate Ollama settings if --ollama is set
	if spawnOllama {
		if tool != "claude" {
			return fmt.Errorf("--ollama flag is only supported with claude tool")
		}
		cfg, err := config.Get()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.Ollama.BaseURL == "" {
			return fmt.Errorf("--ollama requires CODERS_OLLAMA_BASE_URL environment variable or ollama.base_url in config")
		}
		if cfg.Ollama.AuthToken == "" && cfg.Ollama.APIKey == "" {
			return fmt.Errorf("--ollama requires CODERS_OLLAMA_AUTH_TOKEN/API_KEY or ollama.auth_token/api_key in config")
		}
	}

	// Resolve working directory
	cwd := spawnCwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	} else {
		resolved, err := resolveDirectory(cwd)
		if err != nil {
			return fmt.Errorf("failed to resolve directory '%s': %w", cwd, err)
		}
		cwd = resolved
	}

	// Generate session name
	sessionName := generateSessionName(tool, spawnTask)
	sessionID := tmux.SessionPrefix + sessionName

	// Create logger with session context
	log = log.WithSessionID(sessionID)

	// Check if session already exists
	if tmux.SessionExists(sessionID) {
		log.Warn("session already exists")
		return fmt.Errorf("session '%s' already exists", sessionID)
	}

	// Build the command to run
	toolCmd := buildToolCommand(tool, spawnTask, spawnModel, sessionID, spawnOllama)

	// Get user's shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create prompt file for tools that need it
	promptFile := ""
	if spawnTask != "" && (tool == "claude" || tool == "codex" || tool == "opencode") {
		promptFile = fmt.Sprintf("/tmp/coders-prompt-%d.txt", time.Now().UnixNano())
		prompt := buildPrompt(tool, spawnTask)
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			return fmt.Errorf("failed to write prompt file: %w", err)
		}
	}

	// Build full tmux command
	var fullCmd string
	if promptFile != "" {
		fullCmd = fmt.Sprintf("cd %s && %s < %s; exec %s",
			shellEscape(cwd), toolCmd, promptFile, shell)
	} else {
		fullCmd = fmt.Sprintf("cd %s && %s; exec %s",
			shellEscape(cwd), toolCmd, shell)
	}

	// Create tmux session
	log.Info("creating tmux session")
	fmt.Printf("Creating session: %s\n", sessionID)
	tmuxArgs := []string{"new-session", "-d", "-s", sessionID, "-c", cwd, "sh", "-c", fullCmd}

	createCmd := exec.Command("tmux", tmuxArgs...)
	if err := createCmd.Run(); err != nil {
		log.WithError(err).Error("failed to create tmux session")
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"tool":    tool,
		"task":    spawnTask,
		"cwd":     cwd,
		"model":   spawnModel,
		"ollama":  spawnOllama,
	}).Info("session created successfully")
	fmt.Printf("\033[32m✅ Created session: %s\033[0m\n", sessionID)
	fmt.Printf("   Tool: %s\n", tool)
	if spawnTask != "" {
		fmt.Printf("   Task: %s\n", spawnTask)
	}
	fmt.Printf("   Directory: %s\n", cwd)

	// Wait for CLI to be ready
	fmt.Printf("⏳ Waiting for %s to start...\n", tool)
	if ready := waitForCLIReady(sessionID, tool, 30*time.Second); ready {
		fmt.Printf("\033[32m✅ %s is running\033[0m\n", tool)
	} else {
		fmt.Printf("\033[33m⚠️  Timeout waiting for %s (session created but process may still be starting)\033[0m\n", tool)
	}

	// Start heartbeat if enabled
	if spawnHeartbeat {
		if err := startHeartbeat(sessionID, spawnTask, ""); err != nil {
			fmt.Printf("\033[33m⚠️  Failed to start heartbeat: %v\033[0m\n", err)
		} else {
			fmt.Printf("\033[32m💓 Heartbeat enabled\033[0m\n")
		}
	}

	// Store session state and start crash watcher if enabled
	if spawnRestartOnCrash {
		if err := storeSessionState(sessionID, sessionName, tool, spawnTask, cwd, spawnModel, spawnOllama, spawnHeartbeat, true, spawnMaxRestarts); err != nil {
			fmt.Printf("\033[33m⚠️  Failed to store session state for crash recovery: %v\033[0m\n", err)
			fmt.Printf("\033[33m   Crash recovery will not be available for this session.\033[0m\n")
		} else {
			if err := startCrashWatcher(sessionID); err != nil {
				fmt.Printf("\033[33m⚠️  Failed to start crash watcher: %v\033[0m\n", err)
			} else {
				fmt.Printf("\033[32m🔄 Crash recovery enabled (max %d restarts)\033[0m\n", spawnMaxRestarts)
			}
		}
	}

	// Print attach instructions
	fmt.Printf("\n\033[33m💡 Attach: coders attach %s\033[0m\n", sessionName)
	fmt.Printf("\033[33m💡 Or: tmux attach -t %s\033[0m\n", sessionID)

	// Optionally attach
	if spawnAttach {
		fmt.Println("\nAttaching...")
		return tmux.AttachSession(sessionID)
	}

	return nil
}

// generateSessionName creates a session name from tool and task.
func generateSessionName(tool, task string) string {
	if task == "" {
		return fmt.Sprintf("%s-%d", tool, time.Now().Unix()%10000)
	}

	// Slugify task
	slug := strings.ToLower(task)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, slug)

	// Remove consecutive dashes and trim
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")

	// Truncate if too long
	if len(slug) > 30 {
		slug = slug[:30]
		slug = strings.TrimRight(slug, "-")
	}

	if slug == "" {
		slug = fmt.Sprintf("%d", time.Now().Unix()%10000)
	}

	return fmt.Sprintf("%s-%s", tool, slug)
}

// buildToolCommand builds the command to run the AI tool.
func buildToolCommand(tool, task, model, sessionID string, useOllama bool) string {
	var cmd string
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf(" --model %s", shellEscape(model))
	}

	// Set environment variables
	envVars := fmt.Sprintf("CODERS_SESSION_ID=%s", sessionID)

	// Add Ollama env var mappings if --ollama flag is set
	if useOllama {
		cfg, _ := config.Get()
		baseURL := cfg.Ollama.BaseURL
		authToken := cfg.Ollama.AuthToken
		apiKey := cfg.Ollama.APIKey

		// Ensure URL has protocol
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}

		envVars += fmt.Sprintf(" ANTHROPIC_BASE_URL=%s", shellEscape(baseURL))
		// Set API_KEY to empty string to prevent Claude Code from falling back to Anthropic
		// and use AUTH_TOKEN for Bearer auth (required by most Ollama proxies)
		envVars += " ANTHROPIC_API_KEY=''"
		if authToken != "" {
			envVars += fmt.Sprintf(" ANTHROPIC_AUTH_TOKEN=%s", shellEscape(authToken))
		} else if apiKey != "" {
			// Fallback to API_KEY if AUTH_TOKEN not set
			envVars += fmt.Sprintf(" ANTHROPIC_AUTH_TOKEN=%s", shellEscape(apiKey))
		}
	}

	switch tool {
	case "claude":
		cmd = fmt.Sprintf("%s claude --dangerously-skip-permissions%s", envVars, modelArg)
	case "gemini":
		if task != "" {
			escapedTask := shellEscape(task)
			cmd = fmt.Sprintf("%s gemini --yolo%s --prompt-interactive %s", envVars, modelArg, escapedTask)
		} else {
			cmd = fmt.Sprintf("%s gemini --yolo%s", envVars, modelArg)
		}
	case "codex":
		cmd = fmt.Sprintf("%s codex exec --dangerously-bypass-approvals-and-sandbox%s", envVars, modelArg)
	case "opencode":
		cmd = fmt.Sprintf("%s opencode%s", envVars, modelArg)
	}

	return cmd
}

// buildPrompt creates the initial prompt for tools that accept stdin.
func buildPrompt(tool, task string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("TASK: %s\n\n", task))
	b.WriteString("You have full permissions. Complete the task.\n\n")
	b.WriteString("⚠️  IMPORTANT: When you finish this task, you MUST publish a completion promise.\n")

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

// resolveDirectory resolves a directory path, with zoxide support.
func resolveDirectory(path string) (string, error) {
	// First check if it's a direct path
	if filepath.IsAbs(path) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}

	// Try relative to cwd
	cwd, _ := os.Getwd()
	absPath := filepath.Join(cwd, path)
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		return absPath, nil
	}

	// Try zoxide
	if isZoxideAvailable() {
		out, err := exec.Command("zoxide", "query", path).Output()
		if err == nil {
			resolved := strings.TrimSpace(string(out))
			if info, err := os.Stat(resolved); err == nil && info.IsDir() {
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("directory not found: %s", path)
}

// isZoxideAvailable checks if zoxide is installed.
func isZoxideAvailable() bool {
	_, err := exec.LookPath("zoxide")
	return err == nil
}

// waitForCLIReady waits for the CLI process to start in the tmux session.
func waitForCLIReady(sessionID, tool string, timeout time.Duration) bool {
	processNames := map[string]string{
		"claude":   "claude",
		"gemini":   "gemini",
		"codex":    "codex",
		"opencode": "opencode",
	}
	processName := processNames[tool]

	start := time.Now()
	for time.Since(start) < timeout {
		// Get pane PID
		out, err := exec.Command("tmux", "display-message", "-t", sessionID, "-p", "#{pane_pid}").Output()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		panePID := strings.TrimSpace(string(out))
		if panePID == "" {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check for child processes
		childOut, err := exec.Command("pgrep", "-P", panePID).Output()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

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
			if strings.Contains(procName, processName) {
				return true
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return false
}

// shellEscape escapes a string for safe use in shell commands.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// startHeartbeat starts a background heartbeat process for the session.
func startHeartbeat(sessionID, task, parentSessionID string) error {
	// Get the path to this executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build heartbeat command args
	args := []string{"heartbeat", "--session", sessionID}
	if task != "" {
		args = append(args, "--task", task)
	}
	if parentSessionID != "" {
		args = append(args, "--parent", parentSessionID)
	}

	// Start heartbeat as a background process
	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	// Don't wait for it - let it run in background
	go func() {
		cmd.Wait()
	}()

	return nil
}
