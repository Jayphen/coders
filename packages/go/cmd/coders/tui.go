package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the terminal user interface",
		Long:  `Launch the interactive TUI for managing coder sessions.`,
		RunE:  runTUI,
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	// If not inside tmux or no TTY, launch TUI in its own tmux session
	if !tmux.IsInsideTmux() || !hasTTY() {
		return launchInTmuxSession()
	}

	// We're inside tmux with a TTY - run the TUI directly
	model := tui.NewModel(Version)
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

func hasTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func launchInTmuxSession() error {
	if tmux.SessionExists(tmux.TUISession) {
		if hasTTY() {
			// Session exists and we have a TTY, attach to it
			cmd := exec.Command("tmux", "attach", "-t", tmux.TUISession)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		// No TTY - tell user how to attach
		fmt.Printf("\033[32m✓ TUI session already running\033[0m\n")
		fmt.Printf("  Attach with: tmux attach -t %s\n", tmux.TUISession)
		return nil
	}

	// Get the path to this executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if hasTTY() {
		// Create new session running the TUI and attach
		cmd := exec.Command("tmux", "new-session", "-s", tmux.TUISession, "-n", "tui", exe, "tui")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// No TTY - create detached session
	tuiCmd := fmt.Sprintf("while [ $(tmux list-clients -t %s 2>/dev/null | wc -l) -eq 0 ]; do sleep 0.1; done; %s tui",
		tmux.TUISession, exe)

	cmd := exec.Command("tmux", "new-session", "-d", "-s", tmux.TUISession, "-n", "tui", "sh", "-c", tuiCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create TUI session: %w", err)
	}

	fmt.Printf("\033[32m✓ TUI session started\033[0m\n")
	fmt.Printf("  Attach with: tmux attach -t %s\n", tmux.TUISession)
	return nil
}
