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
	"github.com/Jayphen/coders/internal/notify"
	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tasksource"
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
	loopWait          bool
	loopID            string
	loopSources       []string // Multi-source task specifications
	loopOnlyReady     bool     // Only process tasks with no blockers
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
		Short: "Run tasks from multiple sources in a loop",
		Long: `Automatically spawn coder sessions for each task from one or more task sources.

The loop runner can pull tasks from multiple sources:
  - Todolist files (markdown format: [ ] task description)
  - Beads issues (git-backed issue tracker)
  - Linear issues
  - GitHub issues

Multi-source support allows mixing tasks from different systems in a single loop.

Features:
  - Multi-source task aggregation (beads, Linear, GitHub, todolist files)
  - Auto-switches from Claude to Codex if usage limit warnings are detected
  - Saves state to Redis for recovery
  - Can stop on blocked tasks or continue
  - Runs in background by default (use --wait for blocking mode)
  - Supports recursive loops (coder can spawn sub-loops with --wait)

Examples:
  # Legacy todolist mode (backward compatible)
  coders loop --todolist tasks.txt --cwd ~/project

  # Multi-source mode
  coders loop --source "beads:cwd=." --cwd ~/project
  coders loop --source "todolist:path=tasks.txt" --source "beads:cwd=." --cwd ~/project
  coders loop --source "linear:team=TEAM123" --cwd ~/project
  coders loop --source "github:owner=user,repo=myrepo" --cwd ~/project

  # Only ready tasks (no blockers)
  coders loop --source "beads:cwd=." --only-ready --cwd ~/project

Recursive loops (from within a coder session):
  coders loop --wait --todolist subtasks.txt --cwd .
  # Blocks until all subtasks complete, then coder continues`,
		RunE: runLoop,
	}

	cmd.Flags().StringVar(&loopTodolist, "todolist", "", "Path to todolist file (legacy, use --source instead)")
	cmd.Flags().StringSliceVar(&loopSources, "source", []string{}, "Task source specification (format: type:param=value,...)")
	cmd.Flags().StringVar(&loopCwd, "cwd", "", "Working directory for spawned sessions (required)")
	cmd.Flags().StringVar(&loopTool, "tool", defaultTool, "AI tool to use (claude, gemini, codex, opencode)")
	cmd.Flags().StringVar(&loopModel, "model", "", "Model to use")
	cmd.Flags().IntVar(&loopMaxConcurrent, "max-concurrent", 1, "Maximum concurrent sessions (not yet implemented)")
	cmd.Flags().BoolVar(&loopStopOnBlocked, "stop-on-blocked", false, "Stop loop if a task is blocked")
	cmd.Flags().BoolVar(&loopOnlyReady, "only-ready", false, "Only process tasks with no blockers")
	cmd.Flags().BoolVar(&loopBackground, "background", true, "Run in background")
	cmd.Flags().BoolVarP(&loopWait, "wait", "w", false, "Wait for loop to complete (blocks until done, enables recursive loops)")
	cmd.Flags().StringVar(&loopID, "loop-id", "", "Custom loop ID (auto-generated if not set)")

	cmd.MarkFlagRequired("cwd")

	return cmd
}

func runLoop(cmd *cobra.Command, args []string) error {
	log := logging.WithCommand("loop")

	// Validate inputs - either --todolist or --source must be specified
	if loopTodolist == "" && len(loopSources) == 0 {
		return fmt.Errorf("either --todolist or --source must be specified")
	}

	if loopCwd == "" {
		return fmt.Errorf("--cwd is required")
	}

	// Convert legacy --todolist to source spec
	sourceSpecs := loopSources
	if loopTodolist != "" {
		todolistPath, err := filepath.Abs(loopTodolist)
		if err != nil {
			return fmt.Errorf("failed to resolve todolist path: %w", err)
		}
		sourceSpecs = append([]string{fmt.Sprintf("todolist:path=%s", todolistPath)}, sourceSpecs...)
	}

	// Resolve working directory
	cwdPath, err := resolveDirectory(loopCwd)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	// Generate loop ID if not set
	if loopID == "" {
		loopID = fmt.Sprintf("loop-%d", time.Now().Unix())
	}

	log.WithFields(map[string]interface{}{
		"sources": sourceSpecs,
		"cwd":     cwdPath,
		"tool":    loopTool,
		"loopId":  loopID,
		"wait":    loopWait,
	}).Info("starting loop")

	// --wait flag overrides --background (enables recursive loops from within coders)
	if loopWait {
		loopBackground = false
	}

	// If background mode, spawn ourselves as a background process
	if loopBackground {
		return runLoopInBackground(sourceSpecs, cwdPath)
	}

	// Run in foreground
	return executeLoopWithSources(sourceSpecs, cwdPath)
}

func runLoopInBackground(sourceSpecs []string, cwdPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build args for background process
	bgArgs := []string{
		"loop",
		"--cwd", cwdPath,
		"--tool", loopTool,
		"--background=false", // Don't recurse
		"--loop-id", loopID,
	}

	// Add source specs
	for _, spec := range sourceSpecs {
		bgArgs = append(bgArgs, "--source", spec)
	}

	if loopModel != "" {
		bgArgs = append(bgArgs, "--model", loopModel)
	}
	if loopStopOnBlocked {
		bgArgs = append(bgArgs, "--stop-on-blocked")
	}
	if loopOnlyReady {
		bgArgs = append(bgArgs, "--only-ready")
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

	fmt.Printf("\033[34m🔄 Starting multi-source loop\033[0m\n")
	fmt.Printf("   📂 Sources: %v\n", sourceSpecs)
	fmt.Printf("   📁 Working directory: %s\n", cwdPath)
	fmt.Printf("   🤖 Tool: %s\n", loopTool)
	fmt.Printf("   🆔 Loop ID: %s\n", loopID)
	fmt.Printf("\n\033[32m✅ Loop started in background\033[0m\n")
	fmt.Printf("   📋 Log: %s\n", logFile)
	fmt.Printf("   💡 Check status: coders loop-status --loop-id %s\n", loopID)

	return nil
}

// executeLoopWithSources executes a loop using the new multi-source TaskSource interface
func executeLoopWithSources(sourceSpecs []string, cwdPath string) error {
	log := logging.WithCommand("loop")

	fmt.Printf("\033[34m🔄 Starting Multi-Source Loop\033[0m\n")
	fmt.Printf("   📂 Sources: %v\n", sourceSpecs)
	fmt.Printf("   📁 Working directory: %s\n", cwdPath)
	fmt.Printf("   🤖 Tool: %s\n", loopTool)
	fmt.Printf("   🆔 Loop ID: %s\n", loopID)
	fmt.Println()

	// Create task sources
	multiSource, err := tasksource.CreateMultiSourceFromStrings(sourceSpecs)
	if err != nil {
		return fmt.Errorf("failed to create task sources: %w", err)
	}
	defer multiSource.Close()

	// Set up context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build task filter
	filter := &tasksource.TaskFilter{
		Status:    []tasksource.TaskStatus{tasksource.TaskStatusOpen, tasksource.TaskStatusInProgress},
		OnlyReady: loopOnlyReady,
	}

	// List tasks from all sources
	tasks, err := multiSource.ListTasks(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	fmt.Printf("📋 Found %d tasks from %d source(s)\n\n", len(tasks), len(multiSource.Sources()))

	currentTool := loopTool
	completedCount := 0

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\033[33m⏹️  Loop interrupted by user\033[0m")
		saveLoopState(LoopState{
			LoopID: loopID,
			Status: "paused",
		})
		// Send notification about paused loop with actual completed count
		if err := notifyLoopComplete(loopID, completedCount, "paused"); err != nil {
			log.WithError(err).Warn("failed to send loop notification")
		}
		cancel()
	}()

	if len(tasks) == 0 {
		fmt.Println("\033[32m✅ All tasks already completed!\033[0m")
		return nil
	}

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
			Cwd:              cwdPath,
			CurrentTaskIndex: i,
			TotalTasks:       len(tasks),
			CurrentTool:      currentTool,
			Status:           "running",
		}); err != nil {
			log.WithError(err).Warn("failed to save loop state")
		}

		// Spawn task
		sessionName, err := spawnLoopTaskFromSource(task, i, len(tasks), currentTool, cwdPath)
		if err != nil {
			fmt.Printf("\033[31m❌ Failed to spawn task: %v\033[0m\n", err)
			if err := notifyLoopComplete(loopID, completedCount, "failed"); err != nil {
				log.WithError(err).Warn("failed to send loop notification")
			}
			break
		}

		// Wait for promise
		promise, err := waitForLoopPromise(ctx, sessionName)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Cancelled
			}
			fmt.Printf("\033[31m❌ Failed waiting for promise: %v\033[0m\n", err)
			if err := notifyLoopComplete(loopID, completedCount, "failed"); err != nil {
				log.WithError(err).Warn("failed to send loop notification")
			}
			break
		}

		// Check if blocked
		if promise.Status == "blocked" {
			fmt.Printf("\n\033[33m🚫 Task blocked: %s\033[0m\n", promise.Summary)

			// Mark task as blocked in source
			if err := multiSource.MarkBlocked(ctx, task.ID, promise.Summary); err != nil {
				log.WithError(err).Warn("failed to mark task as blocked")
			}

			if loopStopOnBlocked {
				fmt.Println("\033[33m⏸️  Stopping loop (--stop-on-blocked enabled)\033[0m")
				if err := notifyLoopComplete(loopID, completedCount, "blocked"); err != nil {
					log.WithError(err).Warn("failed to send loop notification")
				}
				break
			}
			fmt.Println("\033[33m⚠️  Continuing despite blocked status...\033[0m")
			continue // Skip marking as complete
		}

		// Mark task as complete in its source
		result, err := multiSource.MarkComplete(ctx, task.ID)
		if err != nil {
			log.WithError(err).Warn("failed to mark task complete")
		} else {
			fmt.Printf("\033[32m✅ %s\033[0m\n", result.Message)
		}

		completedCount++
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
		Cwd:              cwdPath,
		CurrentTaskIndex: completedCount,
		TotalTasks:       len(tasks),
		Status:           "completed",
	})

	// Send notification about completed loop with actual completed count
	if err := notifyLoopComplete(loopID, completedCount, "completed"); err != nil {
		log.WithError(err).Warn("failed to send loop notification")
	}

	return nil
}

// spawnLoopTaskFromSource spawns a coder session for a task from a TaskSource
func spawnLoopTaskFromSource(task tasksource.Task, index, total int, tool, cwd string) (string, error) {
	// Build the task description with completion instructions and source context
	sourceInfo := fmt.Sprintf("[Source: %s, ID: %s]", task.Source, task.SourceID)
	fullTask := fmt.Sprintf("%s %s. When complete, commit changes and push to GitHub, then publish a completion promise.", task.Title, sourceInfo)

	// Generate the session name using the same logic as spawn command
	sessionName := generateSessionName(tool, fullTask)

	fmt.Printf("\n\033[34m🚀 Spawning task %d/%d\033[0m\n", index+1, total)
	fmt.Printf("   📝 Task: %s\n", task.Title)
	fmt.Printf("   🔖 Source: %s (%s)\n", task.Source, task.SourceID)
	if task.Priority >= 0 && task.Priority <= 4 {
		fmt.Printf("   🔥 Priority: P%d\n", task.Priority)
	}
	fmt.Printf("   🤖 Tool: %s\n", tool)

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

	// Parse todolist
	tasks, err := parseTodolist(todolistPath)
	if err != nil {
		return fmt.Errorf("failed to parse todolist: %w", err)
	}

	fmt.Printf("📋 Found %d uncompleted tasks\n\n", len(tasks))

	currentTool := loopTool
	completedCount := 0

	// Set up signal handling for graceful shutdown (after tasks are parsed)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\033[33m⏹️  Loop interrupted by user\033[0m")
		saveLoopState(LoopState{
			LoopID: loopID,
			Status: "paused",
		})
		// Send notification about paused loop with actual completed count
		if err := notifyLoopComplete(loopID, completedCount, "paused"); err != nil {
			log.WithError(err).Warn("failed to send loop notification")
		}
		cancel()
	}()

	if len(tasks) == 0 {
		fmt.Println("\033[32m✅ All tasks already completed!\033[0m")
		return nil
	}

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
			if err := notifyLoopComplete(loopID, completedCount, "failed"); err != nil {
				log.WithError(err).Warn("failed to send loop notification")
			}
			break
		}

		// Wait for promise
		promise, err := waitForLoopPromise(ctx, sessionName)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Cancelled
			}
			fmt.Printf("\033[31m❌ Failed waiting for promise: %v\033[0m\n", err)
			if err := notifyLoopComplete(loopID, completedCount, "failed"); err != nil {
				log.WithError(err).Warn("failed to send loop notification")
			}
			break
		}

		// Check if blocked
		if promise.Status == "blocked" {
			fmt.Printf("\n\033[33m🚫 Task blocked: %s\033[0m\n", promise.Summary)
			if loopStopOnBlocked {
				fmt.Println("\033[33m⏸️  Stopping loop (--stop-on-blocked enabled)\033[0m")
				if err := notifyLoopComplete(loopID, completedCount, "blocked"); err != nil {
					log.WithError(err).Warn("failed to send loop notification")
				}
				break
			}
			fmt.Println("\033[33m⚠️  Continuing despite blocked status...\033[0m")
		}

		// Mark task as complete
		if err := markTaskComplete(todolistPath, task); err != nil {
			log.WithError(err).Warn("failed to mark task complete")
		}
		completedCount++
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
		CurrentTaskIndex: completedCount,
		TotalTasks:       len(tasks),
		Status:           "completed",
	})

	// Send notification about completed loop with actual completed count
	if err := notifyLoopComplete(loopID, completedCount, "completed"); err != nil {
		log.WithError(err).Warn("failed to send loop notification")
	}

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
	// Build the task description with completion instructions
	fullTask := fmt.Sprintf("%s. When complete, commit changes and push to GitHub, then publish a completion promise.", task)

	// Generate the session name using the same logic as spawn command
	// This ensures we wait for the correct promise
	sessionName := generateSessionName(tool, fullTask)

	fmt.Printf("\n\033[34m🚀 Spawning task %d/%d\033[0m\n", index+1, total)
	fmt.Printf("   📝 Task: %s\n", task)
	fmt.Printf("   🤖 Tool: %s\n", tool)

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

// notifyLoopComplete sends a notification when a loop finishes
func notifyLoopComplete(loopID string, taskCount int, status string) error {
	log := logging.WithCommand("loop")

	rdb, err := redis.GetClient()
	if err != nil {
		log.WithError(err).Warn("failed to connect to Redis for notification")
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	notification := &types.LoopNotification{
		LoopID:    loopID,
		Timestamp: time.Now().UnixMilli(),
		TaskCount: taskCount,
		Status:    status,
	}

	// Add a descriptive message based on status
	switch status {
	case "completed":
		notification.Message = fmt.Sprintf("Loop completed successfully with %d tasks", taskCount)
	case "paused":
		notification.Message = fmt.Sprintf("Loop paused after processing %d tasks", taskCount)
	case "failed":
		notification.Message = fmt.Sprintf("Loop failed after processing %d tasks", taskCount)
	default:
		notification.Message = fmt.Sprintf("Loop finished with status '%s' after %d tasks", status, taskCount)
	}

	ctx := context.Background()
	if err := rdb.SetLoopNotification(ctx, notification); err != nil {
		log.WithError(err).Warn("failed to store loop notification")
		return fmt.Errorf("failed to store notification: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"loopId":    loopID,
		"taskCount": taskCount,
		"status":    status,
	}).Info("loop notification sent")

	// Send tmux display-message notification to parent session
	tmuxMessage := fmt.Sprintf("Loop %s %s: %d tasks", loopID, status, taskCount)
	if err := tmux.SendDisplayMessage("", tmuxMessage); err != nil {
		log.WithError(err).Debug("failed to send tmux display message (non-fatal)")
		// Non-fatal - continue even if tmux notification fails
	}

	// Send OS-native notification
	notificationTitle := fmt.Sprintf("Loop %s", status)
	notify.Send(notificationTitle, notification.Message)

	return nil
}
