// Package main is the entry point for the coders CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "coders",
		Short: "Manage AI coding sessions",
		Long: `Coders is a CLI for managing AI coding sessions in tmux.

It supports spawning, listing, attaching to, and killing sessions
running various AI coding tools like Claude, Gemini, Codex, and OpenCode.`,
	}

	// Add subcommands
	rootCmd.AddCommand(
		newInitCmd(),
		newOrchestratorCmd(),
		newSpawnCmd(),
		newListCmd(),
		newAttachCmd(),
		newKillCmd(),
		newPromiseCmd(),
		newResumeCmd(),
		newHeartbeatCmd(),
		newTUICmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
