package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/tmux"
)

func newOrchestratorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orchestrator",
		Short: "Start or attach to the orchestrator session",
		Long: `Start or attach to the orchestrator session for coordinating multiple coder sessions.

The orchestrator is a persistent Claude session that can spawn and manage other coder sessions.

For a complete setup including the TUI, use 'coders init' instead.`,
		RunE: runOrchestrator,
	}
}

func runOrchestrator(cmd *cobra.Command, args []string) error {
	// Check if orchestrator already exists
	if tmux.SessionExists(tmux.OrchestratorSession) {
		fmt.Printf("\033[34m🔗 Orchestrator session exists, attaching...\033[0m\n")
		return tmux.AttachSession(tmux.OrchestratorSession)
	}

	// Start new orchestrator
	fmt.Println("🚀 Creating orchestrator session...")

	if err := createOrchestratorSession(); err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	fmt.Printf("\033[32m✅ Created orchestrator session: %s\033[0m\n", tmux.OrchestratorSession)
	fmt.Printf("   💡 Attach: coders orchestrator\n")
	fmt.Printf("   💡 Or: tmux attach -t %s\n", tmux.OrchestratorSession)

	// Wait a moment for session to initialize
	time.Sleep(500 * time.Millisecond)

	// Auto-attach if we have a TTY
	if hasTTY() {
		return tmux.AttachSession(tmux.OrchestratorSession)
	}

	return nil
}

// createOrchestratorSession creates the orchestrator session.
func createOrchestratorSession() error {
	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = os.Getenv("HOME")
	}

	// Get user's shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create orchestrator prompt
	prompt := `
╔════════════════════════════════════════════════════════════════════════════╗
║                      CODER ORCHESTRATOR SESSION                            ║
║                                                                            ║
║  This is a special persistent session for coordinating other coder        ║
║  sessions. You can use the following commands:                            ║
║                                                                            ║
║  - coders spawn <tool> [options]  : Spawn a new coder session             ║
║  - coders list                    : List all active sessions               ║
║  - coders promises                : Check completion status of sessions    ║
║  - coders attach <session>        : Attach to a session                    ║
║  - coders kill <session>          : Kill a session                         ║
║  - coders tui                     : Open the TUI for visual management     ║
║                                                                            ║
║  Use Claude Code to orchestrate your AI coding sessions!                  ║
╚════════════════════════════════════════════════════════════════════════════╝

Welcome to the Orchestrator session. You have full permissions to spawn and manage
other coder sessions. Start by spawning your first session or listing existing ones.

📌 TIP: Use 'coders promises' to see which spawned sessions have completed their tasks.
`

	// Write prompt to temp file
	promptFile := fmt.Sprintf("/tmp/coders-orchestrator-prompt-%d.txt", time.Now().UnixNano())
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Build the command
	envVars := fmt.Sprintf("CODERS_SESSION_ID=%s", tmux.OrchestratorSession)
	toolCmd := fmt.Sprintf("%s claude --dangerously-skip-permissions < %s", envVars, promptFile)
	fullCmd := fmt.Sprintf("cd %s && %s; exec %s", shellEscape(cwd), toolCmd, shell)

	// Create tmux session
	tmuxArgs := []string{"new-session", "-d", "-s", tmux.OrchestratorSession, "-c", cwd, "sh", "-c", fullCmd}

	createCmd := exec.Command("tmux", tmuxArgs...)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Wait for Claude to start
	fmt.Println("⏳ Waiting for Claude to start...")
	if ready := waitForCLIReady(tmux.OrchestratorSession, "claude", 30*time.Second); ready {
		fmt.Printf("\033[32m✅ Claude is running\033[0m\n")
	} else {
		fmt.Printf("\033[33m⚠️  Timeout waiting for Claude (session created but process may still be starting)\033[0m\n")
	}

	// Start heartbeat for orchestrator
	if err := startHeartbeat(tmux.OrchestratorSession, "orchestrator", ""); err != nil {
		fmt.Printf("\033[33m⚠️  Failed to start heartbeat: %v\033[0m\n", err)
	} else {
		fmt.Printf("\033[32m💓 Heartbeat enabled\033[0m\n")
	}

	return nil
}
