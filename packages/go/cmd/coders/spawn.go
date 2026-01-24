package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/spf13/cobra"
	"golang.org/x/term"

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
	spawnWorktree       bool
	spawnPTY            bool
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
		Long: `Spawn a new AI coding session.

Supported tools: claude, gemini, codex, opencode

Examples:
  coders spawn claude --task "Fix the login bug"
  coders spawn gemini --task "Add unit tests" --cwd ~/projects/myapp
  coders spawn codex --task "Refactor auth module" --model gpt-4
  coders spawn --attach  # Spawn and attach immediately
  coders spawn --pty --task "Interactive coding"  # Direct PTY (no tmux)
  coders spawn --restart-on-crash --task "Long running task"  # Auto-restart on crash
  coders spawn --worktree --task "Feature branch work"  # Create git worktree

Direct PTY Mode (--pty):
  With --pty, the session uses direct PTY management instead of tmux.
  This provides zero-latency keystroke forwarding, essential for features
  like autocomplete and slash commands in Claude Code. The session runs
  in the foreground and exits when you press Ctrl+]. Note: sessions
  cannot be detached/reattached without tmux.

Git Worktree:
  With --worktree, a new git worktree is created for isolated development.
  The worktree is created in .coders/worktrees/<session-name> with a branch
  named session/<session-name>. This allows working on features in isolation
  without affecting the main working directory.

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
	cmd.Flags().BoolVar(&spawnWorktree, "worktree", false, "Create a git worktree for isolated development")
	cmd.Flags().BoolVar(&spawnPTY, "pty", false, "Use direct PTY instead of tmux (zero-latency keystrokes)")

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

	// Generate session name (needed before worktree creation)
	sessionName := generateSessionName(tool, spawnTask)
	sessionID := tmux.SessionPrefix + sessionName

	// Create git worktree if requested
	if spawnWorktree {
		worktreePath, err := createWorktree(cwd, sessionName)
		if err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		cwd = worktreePath
		fmt.Printf("\033[32m✅ Created git worktree: %s\033[0m\n", worktreePath)
	}

	// Create logger with session context
	log = log.WithSessionID(sessionID)

	// Direct PTY mode - bypasses tmux for zero-latency keystrokes
	if spawnPTY {
		return runPTYSpawn(tool, spawnTask, cwd, spawnModel, sessionID, spawnOllama)
	}

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

	// Create prompt for tools that need it (but don't use stdin for codex)
	var prompt string
	sendPromptViaTmux := false
	if spawnTask != "" && (tool == "claude" || tool == "opencode") {
		prompt = buildPrompt(tool, spawnTask)
	} else if spawnTask != "" && tool == "codex" {
		// Codex requires TTY, so we'll send the prompt via tmux send-keys instead
		prompt = buildPrompt(tool, spawnTask)
		sendPromptViaTmux = true
	}

	// Build full tmux command
	var fullCmd string
	if prompt != "" && !sendPromptViaTmux {
		// For tools that accept stdin (claude, opencode)
		promptFile := fmt.Sprintf("/tmp/coders-prompt-%d.txt", time.Now().UnixNano())
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			return fmt.Errorf("failed to write prompt file: %w", err)
		}
		fullCmd = fmt.Sprintf("cd %s && %s < %s; exec %s",
			shellEscape(cwd), toolCmd, promptFile, shell)
	} else {
		// For tools that don't need stdin or codex
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
		"tool":   tool,
		"task":   spawnTask,
		"cwd":    cwd,
		"model":  spawnModel,
		"ollama": spawnOllama,
	}).Info("session created successfully")
	fmt.Printf("\033[32m✅ Created session: %s\033[0m\n", sessionID)
	fmt.Printf("   Tool: %s\n", tool)
	if spawnTask != "" {
		fmt.Printf("   Task: %s\n", spawnTask)
	}
	fmt.Printf("   Directory: %s\n", cwd)

	// Wait for CLI to be ready
	fmt.Printf("⏳ Waiting for %s to start...\n", tool)
	if ready := waitForCLIReady(sessionID, tool, 10*time.Second); ready {
		fmt.Printf("\033[32m✅ %s is running\033[0m\n", tool)
	} else {
		fmt.Printf("\033[33m⚠️  Timeout waiting for %s (session created but process may still be starting)\033[0m\n", tool)
	}

	// Send prompt via tmux if needed (for codex)
	if sendPromptViaTmux && prompt != "" {
		// Wait a bit for CLI to be fully ready
		time.Sleep(2 * time.Second)

		// Send the prompt line by line
		lines := strings.Split(prompt, "\n")
		for _, line := range lines {
			sendCmd := exec.Command("tmux", "send-keys", "-t", sessionID, "-l", line)
			if err := sendCmd.Run(); err != nil {
				log.WithError(err).Warn("failed to send prompt line")
			}
			// Send newline
			exec.Command("tmux", "send-keys", "-t", sessionID, "Enter").Run()
		}
		fmt.Printf("\033[32m✅ Sent task prompt to session\033[0m\n")
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
		cmd = fmt.Sprintf("%s codex --dangerously-bypass-approvals-and-sandbox%s", envVars, modelArg)
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
	log := logging.WithCommand("spawn").WithSessionID(sessionID)

	processNames := map[string]string{
		"claude":   "claude",
		"gemini":   "gemini",
		"codex":    "codex",
		"opencode": "opencode",
	}
	processName := processNames[tool]

	start := time.Now()
	iteration := 0

	for time.Since(start) < timeout {
		iteration++
		iterStart := time.Now()

		// Get pane PID
		tmuxStart := time.Now()
		out, err := exec.Command("tmux", "display-message", "-t", sessionID, "-p", "#{pane_pid}").Output()
		tmuxDuration := time.Since(tmuxStart)

		if err != nil {
			log.Debugf("iter %d: tmux error after %v: %v", iteration, tmuxDuration, err)
			time.Sleep(100 * time.Millisecond) // Reduced from 500ms
			continue
		}

		panePID := strings.TrimSpace(string(out))
		if panePID == "" {
			log.Debugf("iter %d: empty pane PID after %v", iteration, tmuxDuration)
			time.Sleep(100 * time.Millisecond) // Reduced from 500ms
			continue
		}

		// Check for child processes - combine pgrep and ps into a single ps call
		// This is more efficient than running ps for each child separately
		psStart := time.Now()

		// Get all children and their command names in one shot
		// Format: PID PPID COMM
		psOut, err := exec.Command("ps", "-ax", "-o", "pid=,ppid=,comm=").Output()
		if err != nil {
			log.Debugf("iter %d: ps error after %v (tmux: %v): %v",
				iteration, time.Since(psStart), tmuxDuration, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Find children of panePID
		childCount := 0
		found := false
		for _, line := range strings.Split(string(psOut), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}

			ppid := fields[1]
			if ppid != panePID {
				continue
			}

			childCount++
			comm := strings.Join(fields[2:], " ")
			if strings.Contains(comm, processName) {
				found = true
				break
			}
		}

		if found {
			psDuration := time.Since(psStart)
			iterDuration := time.Since(iterStart)
			totalDuration := time.Since(start)

			log.Infof("CLI ready after %d iterations, %v total (iter: %v, tmux: %v, ps: %v for %d children)",
				iteration, totalDuration, iterDuration, tmuxDuration, psDuration, childCount)
			return true
		}

		psDuration := time.Since(psStart)
		iterDuration := time.Since(iterStart)
		log.Debugf("iter %d: no match in %d children after %v (tmux: %v, ps: %v)",
			iteration, childCount, iterDuration, tmuxDuration, psDuration)

		time.Sleep(100 * time.Millisecond) // Reduced from 500ms
	}

	totalDuration := time.Since(start)
	log.Warnf("CLI not ready after %d iterations, %v total (timeout)", iteration, totalDuration)
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

// createWorktree creates a git worktree for isolated development.
func createWorktree(basePath, sessionName string) (string, error) {
	// Find git root
	gitRoot, err := findGitRoot(basePath)
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}

	// Create worktrees directory in git root
	worktreesDir := filepath.Join(gitRoot, ".coders", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Create worktree path
	worktreePath := filepath.Join(worktreesDir, sessionName)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return "", fmt.Errorf("worktree already exists: %s", worktreePath)
	}

	// Create branch name
	branchName := fmt.Sprintf("session/%s", sessionName)

	// Check if branch already exists
	checkBranchCmd := exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", branchName)
	if checkBranchCmd.Run() == nil {
		return "", fmt.Errorf("branch already exists: %s", branchName)
	}

	// Create the worktree with a new branch
	createCmd := exec.Command("git", "-C", gitRoot, "worktree", "add", "-b", branchName, worktreePath)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	return worktreePath, nil
}

// findGitRoot finds the root of the git repository.
func findGitRoot(startPath string) (string, error) {
	cmd := exec.Command("git", "-C", startPath, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// runPTYSpawn runs the tool directly with PTY management (no tmux).
// This provides zero-latency keystroke forwarding for features like autocomplete.
func runPTYSpawn(tool, task, cwd, model, sessionID string, useOllama bool) error {
	log := logging.WithCommand("spawn-pty").WithSessionID(sessionID)

	fmt.Printf("\033[36m🔗 Direct PTY Mode\033[0m\n")
	fmt.Printf("   Tool: %s\n", tool)
	if task != "" {
		fmt.Printf("   Task: %s\n", task)
	}
	fmt.Printf("   Directory: %s\n", cwd)
	fmt.Printf("   Press Ctrl+] to exit\n\n")

	// Build command arguments
	args := buildToolArgs(tool, model, sessionID, useOllama)

	// Create command
	cmd := exec.Command(tool, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	// Add session ID to environment
	cmd.Env = append(cmd.Env, fmt.Sprintf("CODERS_SESSION_ID=%s", sessionID))

	// Add Ollama environment variables if needed
	if useOllama {
		cfg, _ := config.Get()
		baseURL := cfg.Ollama.BaseURL
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("ANTHROPIC_BASE_URL=%s", baseURL),
			"ANTHROPIC_API_KEY=",
		)
		if cfg.Ollama.AuthToken != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", cfg.Ollama.AuthToken))
		} else if cfg.Ollama.APIKey != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", cfg.Ollama.APIKey))
		}
	}

	// Start with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}
	defer ptmx.Close()

	log.Info("PTY session started")

	// Handle window resize
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	go func() {
		for range resizeCh {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.WithError(err).Debug("failed to resize PTY")
			}
		}
	}()
	// Initial resize
	resizeCh <- syscall.SIGWINCH

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Send initial prompt/task if provided
	if task != "" && (tool == "claude" || tool == "opencode") {
		prompt := buildPrompt(tool, task)
		// Wait a bit for the tool to start
		go func() {
			time.Sleep(500 * time.Millisecond)
			ptmx.Write([]byte(prompt + "\n"))
		}()
	}

	// Copy PTY output to stdout
	go func() {
		io.Copy(os.Stdout, ptmx)
	}()

	// Copy stdin to PTY, watch for Ctrl+]
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			// Ctrl+] (0x1D) to exit
			if buf[0] == 0x1D {
				log.Info("Ctrl+] pressed, terminating session")
				cmd.Process.Signal(syscall.SIGTERM)
				return
			}
			ptmx.Write(buf[:n])
		}
	}()

	// Wait for process to exit
	err = cmd.Wait()

	// Clean up signal handler
	signal.Stop(resizeCh)
	close(resizeCh)

	if err != nil {
		log.WithError(err).Info("PTY session ended with error")
		// Don't return error for normal exits (Ctrl+C, Ctrl+])
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == -1 || exitErr.ExitCode() == 130 {
				return nil
			}
		}
		return err
	}

	log.Info("PTY session ended normally")
	return nil
}

// buildToolArgs builds command arguments for the tool (for PTY mode).
func buildToolArgs(tool, model, sessionID string, useOllama bool) []string {
	var args []string

	switch tool {
	case "claude":
		args = append(args, "--dangerously-skip-permissions")
		if model != "" {
			args = append(args, "--model", model)
		}
	case "gemini":
		args = append(args, "--yolo")
		if model != "" {
			args = append(args, "--model", model)
		}
	case "codex":
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
		if model != "" {
			args = append(args, "--model", model)
		}
	case "opencode":
		if model != "" {
			args = append(args, "--model", model)
		}
	}

	return args
}
