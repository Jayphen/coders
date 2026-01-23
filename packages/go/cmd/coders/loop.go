package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
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
	loopTodolist      string
	loopCwd           string
	loopTool          string
	loopModel         string
	loopMaxConcurrent int
	loopStopOnBlocked bool
	loopBackground    bool
	loopID            string
)

const (
	loopStateKeyPrefix   = "coders:loop:state:"
	usageCapThreshold    = 90
	promiseCheckInterval = 5 * time.Second
)

// LoopState represents the current state of a loop execution
type LoopState struct {
	LoopID           string `json:"loopId"`
	TodolistPath     string `json:"todolistPath"`
	Cwd              string `json:"cwd"`
	CurrentTaskIndex int    `json:"currentTaskIndex"`
	TotalTasks       int    `json:"totalTasks"`
	CurrentTool      string `json:"currentTool"`
	Status           string `json:"status"` // running, completed, paused
}

func newLoopCmd() *cobra.Command {
	cfg, _ := config.Get()
	defaultTool := config.DefaultDefaultTool
	if cfg != nil {
		defaultTool = cfg.DefaultTool
	}

	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Run tasks from a todolist in a loop",
		Long: `Automatically spawn coder sessions for each task in a todolist file.

The loop runner reads tasks from a todolist file (format: [ ] task description),
spawns a coder session for each uncompleted task, waits for the session to
publish a completion promise, marks the task complete, and moves to the next.

Features:
  - Auto-switches from Claude to Codex if usage limit warnings are detected
  - Saves state to Redis for recovery
  - Can stop on blocked tasks or continue
  - Runs in background by default

Examples:
  coders loop --todolist tasks.txt --cwd ~/project
  coders loop --todolist todo.md --cwd ~/app --tool gemini
  coders loop --todolist tasks.txt --cwd . --stop-on-blocked --foreground`,
		RunE: runLoop,
	}

	cmd.Flags().StringVar(&loopTodolist, "todolist", "", "Path to todolist file (required)")
	cmd.Flags().StringVar(&loopCwd, "cwd", "", "Working directory for spawned sessions (required)")
	cmd.Flags().StringVar(&loopTool, "tool", defaultTool, "AI tool to use (claude, gemini, codex, opencode)")
	cmd.Flags().StringVar(&loopModel, "model", "", "Model to use")
	cmd.Flags().IntVar(&loopMaxConcurrent, "max-concurrent", 1, "Maximum concurrent sessions (not yet implemented)")
	cmd.Flags().BoolVar(&loopStopOnBlocked, "stop-on-blocked", false, "Stop loop if a task is blocked")
	cmd.Flags().BoolVar(&loopBackground, "background", true, "Run in background")
	cmd.Flags().StringVar(&loopID, "loop-id", "", "Custom loop ID (auto-generated if not set)")

	cmd.MarkFlagRequired("todolist")
	cmd.MarkFlagRequired("cwd")

	return cmd
}

func runLoop(cmd *cobra.Command, args []string) error {
	log := logging.WithCommand("loop")

	// Validate inputs
	if loopTodolist == "" || loopCwd == "" {
		return fmt.Errorf("--todolist and --cwd are required")
	}

	// Resolve paths
	todolistPath, err := filepath.Abs(loopTodolist)
	if err != nil {
		return fmt.Errorf("failed to resolve todolist path: %w", err)
	}

	cwdPath, err := resolveDirectory(loopCwd)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	// Generate loop ID if not set
	if loopID == "" {
		loopID = fmt.Sprintf("loop-%d", time.Now().Unix())
	}

	log.WithFields(map[string]interface{}{
		"todolist": todolistPath,
		"cwd":      cwdPath,
		"tool":     loopTool,
		"loopId":   loopID,
	}).Info("starting loop")

	// If background mode, spawn ourselves as a background process
	if loopBackground {
		return runLoopInBackground(todolistPath, cwdPath)
	}

	// Run in foreground
	return executeLoop(todolistPath, cwdPath)
}

func runLoopInBackground(todolistPath, cwdPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build args for background process
	bgArgs := []string{
		"loop",
		"--todolist", todolistPath,
		"--cwd", cwdPath,
		"--tool", loopTool,
		"--background=false", // Don't recurse
		"--loop-id", loopID,
	}
	if loopModel != "" {
		bgArgs = append(bgArgs, "--model", loopModel)
	}
	if loopStopOnBlocked {
		bgArgs = append(bgArgs, "--stop-on-blocked")
	}

	logFile := fmt.Sprintf("/tmp/coders-loop-%s.log", loopID)

	// Create log file
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	bgCmd := exec.Command(exe, bgArgs...)
	bgCmd.Stdout = f
	bgCmd.Stderr = f
	bgCmd.Stdin = nil
	bgCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := bgCmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("failed to start background loop: %w", err)
	}

	// Don't wait - let it run in background
	go func() {
		bgCmd.Wait()
		f.Close()
	}()

	fmt.Printf("\033[34m🔄 Starting recursive loop\033[0m\n")
	fmt.Printf("   📂 Todolist: %s\n", todolistPath)
	fmt.Printf("   📁 Working directory: %s\n", cwdPath)
	fmt.Printf("   🤖 Tool: %s\n", loopTool)
	fmt.Printf("   🆔 Loop ID: %s\n", loopID)
	fmt.Printf("\n\033[32m✅ Loop started in background\033[0m\n")
	fmt.Printf("   📋 Log: %s\n", logFile)
	fmt.Printf("   💡 Check status: coders loop-status --loop-id %s\n", loopID)

	return nil
}

func executeLoop(todolistPath, cwdPath string) error {
	log := logging.WithCommand("loop")

	fmt.Printf("\033[34m🔄 Starting Recursive Loop\033[0m\n")
	fmt.Printf("   📂 Todolist: %s\n", todolistPath)
	fmt.Printf("   📁 Working directory: %s\n", cwdPath)
	fmt.Printf("   🤖 Tool: %s\n", loopTool)
	fmt.Printf("   🆔 Loop ID: %s\n", loopID)
	fmt.Println()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\033[33m⏹️  Loop interrupted by user\033[0m")
		saveLoopState(LoopState{
			LoopID: loopID,
			Status: "paused",
		})
		cancel()
	}()

	// Parse todolist
	tasks, err := parseTodolist(todolistPath)
	if err != nil {
		return fmt.Errorf("failed to parse todolist: %w", err)
	}

	fmt.Printf("📋 Found %d uncompleted tasks\n\n", len(tasks))

	if len(tasks) == 0 {
		fmt.Println("\033[32m✅ All tasks already completed!\033[0m")
		return nil
	}

	currentTool := loopTool

	// Execute tasks sequentially
	for i, task := range tasks {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Save current state
		if err := saveLoopState(LoopState{
			LoopID:           loopID,
			TodolistPath:     todolistPath,
			Cwd:              cwdPath,
			CurrentTaskIndex: i,
			TotalTasks:       len(tasks),
			CurrentTool:      currentTool,
			Status:           "running",
		}); err != nil {
			log.WithError(err).Warn("failed to save loop state")
		}

		// Spawn task
		sessionName, err := spawnLoopTask(task, i, len(tasks), currentTool, cwdPath)
		if err != nil {
			fmt.Printf("\033[31m❌ Failed to spawn task: %v\033[0m\n", err)
			break
		}

		// Wait for promise
		promise, err := waitForLoopPromise(ctx, sessionName)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Cancelled
			}
			fmt.Printf("\033[31m❌ Failed waiting for promise: %v\033[0m\n", err)
			break
		}

		// Check if blocked
		if promise.Status == "blocked" {
			fmt.Printf("\n\033[33m🚫 Task blocked: %s\033[0m\n", promise.Summary)
			if loopStopOnBlocked {
				fmt.Println("\033[33m⏸️  Stopping loop (--stop-on-blocked enabled)\033[0m")
				break
			}
			fmt.Println("\033[33m⚠️  Continuing despite blocked status...\033[0m")
		}

		// Mark task as complete
		if err := markTaskComplete(todolistPath, task); err != nil {
			log.WithError(err).Warn("failed to mark task complete")
		}
		fmt.Printf("\033[32m✅ Task %d/%d completed\033[0m\n", i+1, len(tasks))

		// Check for usage warning and switch tools if needed
		if currentTool == "claude" && checkForUsageWarning(sessionName) {
			fmt.Println("\n\033[33m⚠️  Detected Claude usage warning - switching to codex for remaining tasks\033[0m")
			currentTool = "codex"
		}

		// Small delay before next task
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n\033[32m🎉 Loop completed!\033[0m")

	saveLoopState(LoopState{
		LoopID:           loopID,
		TodolistPath:     todolistPath,
		Cwd:              cwdPath,
		CurrentTaskIndex: len(tasks),
		TotalTasks:       len(tasks),
		Status:           "completed",
	})

	return nil
}

// parseTodolist reads a todolist file and returns uncompleted tasks
func parseTodolist(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tasks []string
	scanner := bufio.NewScanner(file)

	// Match uncompleted tasks: [ ] Task description
	taskRegex := regexp.MustCompile(`^\[\ \]\s*(.+)$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := taskRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			tasks = append(tasks, strings.TrimSpace(matches[1]))
		}
	}

	return tasks, scanner.Err()
}

// markTaskComplete replaces [ ] with [x] for a task in the todolist file
func markTaskComplete(filePath, taskDescription string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Escape special regex characters in task description
	escaped := regexp.QuoteMeta(taskDescription)
	pattern := regexp.MustCompile(`\[\ \]\s*` + escaped)

	newContent := pattern.ReplaceAllString(string(content), "[x] "+taskDescription)

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// spawnLoopTask spawns a coder session for a task
func spawnLoopTask(task string, index, total int, tool, cwd string) (string, error) {
	sessionName := fmt.Sprintf("%s-loop-task-%d", tool, index+1)

	fmt.Printf("\n\033[34m🚀 Spawning task %d/%d\033[0m\n", index+1, total)
	fmt.Printf("   📝 Task: %s\n", task)
	fmt.Printf("   🤖 Tool: %s\n", tool)

	// Build the task description with completion instructions
	fullTask := fmt.Sprintf("%s. When complete, commit changes and push to GitHub, then publish a completion promise.", task)

	// Build spawn command args
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	spawnArgs := []string{
		"spawn", tool,
		"--cwd", cwd,
		"--task", fullTask,
	}
	if loopModel != "" {
		spawnArgs = append(spawnArgs, "--model", loopModel)
	}

	// Run spawn command
	spawnCmd := exec.Command(exe, spawnArgs...)
	spawnCmd.Stdout = os.Stdout
	spawnCmd.Stderr = os.Stderr

	if err := spawnCmd.Run(); err != nil {
		return "", fmt.Errorf("spawn failed: %w", err)
	}

	return sessionName, nil
}

// waitForLoopPromise waits for a promise from a session
func waitForLoopPromise(ctx context.Context, sessionName string) (*types.CoderPromise, error) {
	rdb, err := redis.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	sessionID := tmux.SessionPrefix + sessionName

	fmt.Printf("\n\033[33m⏳ Waiting for promise from %s...\033[0m\n", sessionName)

	ticker := time.NewTicker(promiseCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			promise, err := rdb.GetPromise(sessionID)
			if err != nil {
				// Key doesn't exist yet, keep waiting
				continue
			}
			if promise != nil {
				fmt.Printf("\n\033[32m✅ Promise received from %s\033[0m\n", sessionName)
				fmt.Printf("   📋 Status: %s\n", promise.Status)
				fmt.Printf("   💬 Summary: %s\n", promise.Summary)
				return promise, nil
			}
		}
	}
}

// checkForUsageWarning checks if Claude has shown a usage warning in the session output
func checkForUsageWarning(sessionName string) bool {
	sessionID := tmux.SessionPrefix + sessionName

	// Capture recent output from the session
	out, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionID, "-S", "-100").Output()
	if err != nil {
		return false
	}

	output := string(out)

	// Look for Claude's usage warning patterns
	warningPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)approaching.*usage\s*limit`),
		regexp.MustCompile(`9[0-9]%.*limit`),
		regexp.MustCompile(`(?i)usage.*limit.*reached`),
		regexp.MustCompile(`(?i)exceeded.*limit`),
	}

	for _, pattern := range warningPatterns {
		if pattern.MatchString(output) {
			return true
		}
	}

	return false
}

// saveLoopState saves the current loop state to Redis
func saveLoopState(state LoopState) error {
	rdb, err := redis.GetClient()
	if err != nil {
		return err
	}

	key := loopStateKeyPrefix + state.LoopID
	return rdb.SetJSON(key, state, 7*24*time.Hour) // 7 day TTL
}
