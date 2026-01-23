package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/tmux"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize coders: start orchestrator and TUI",
		Long: `Initialize the coders environment by:
1. Starting the orchestrator session if not already running
2. Starting the TUI in the background for monitoring
3. Attaching to the orchestrator session

This is the recommended way to start a coders workflow.`,
		RunE: runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	// Step 1: Ensure orchestrator is running
	orchestratorRunning := tmux.SessionExists(tmux.OrchestratorSession)

	if !orchestratorRunning {
		fmt.Println("🚀 Starting orchestrator session...")
		if err := createOrchestratorSession(); err != nil {
			return fmt.Errorf("failed to start orchestrator: %w", err)
		}
		fmt.Printf("\033[32m✅ Orchestrator started: %s\033[0m\n", tmux.OrchestratorSession)
	} else {
		fmt.Printf("\033[32m✅ Orchestrator already running: %s\033[0m\n", tmux.OrchestratorSession)
	}

	// Step 2: Ensure TUI is running in background
	tuiRunning := tmux.SessionExists(tmux.TUISession)

	if !tuiRunning {
		fmt.Println("📊 Starting TUI in background...")
		if err := startTUIBackground(); err != nil {
			// Non-fatal - we can still attach to orchestrator
			fmt.Printf("\033[33m⚠️  Failed to start TUI: %v\033[0m\n", err)
		} else {
			fmt.Printf("\033[32m✅ TUI started: %s\033[0m\n", tmux.TUISession)
		}
	} else {
		fmt.Printf("\033[32m✅ TUI already running: %s\033[0m\n", tmux.TUISession)
	}

	// Step 3: Attach to orchestrator
	fmt.Println("\n🔗 Attaching to orchestrator...")
	fmt.Printf("   (TUI running in background: tmux attach -t %s)\n\n", tmux.TUISession)

	// Wait a moment for everything to settle
	time.Sleep(500 * time.Millisecond)

	return tmux.AttachSession(tmux.OrchestratorSession)
}

// startTUIBackground starts the TUI in a detached tmux session.
func startTUIBackground() error {
	// Get the path to this executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create a command that waits for clients before starting TUI
	// This prevents the TUI from rendering before anyone is attached
	tuiCmd := fmt.Sprintf("while [ $(tmux list-clients -t %s 2>/dev/null | wc -l) -eq 0 ]; do sleep 0.1; done; %s tui",
		tmux.TUISession, exe)

	cmd := exec.Command("tmux", "new-session", "-d", "-s", tmux.TUISession, "-n", "tui", "sh", "-c", tuiCmd)
	return cmd.Run()
}
