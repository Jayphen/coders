// Package main is the entry point for the coders CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/logging"
)

// Version is set at build time.
var Version = "dev"

func main() {
	// Initialize logging from config
	initLogging()

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
		newHelloCmd(),
		newPromiseCmd(),
		newResumeCmd(),
		newHeartbeatCmd(),
		newHealthcheckCmd(),
		newCrashWatcherCmd(),
		newLoopCmd(),
		newLoopStatusCmd(),
		newTUICmd(),
		newVersionCmd(),
		newConfigCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initLogging initializes the logger from config.
func initLogging() {
	cfg, err := config.Get()
	if err != nil {
		// If config fails, use defaults (console output)
		_ = logging.Init(nil)
		return
	}

	// Convert config.LoggingConfig to logging.LoggingConfig
	lc := logging.LoggingConfig{
		Level:      cfg.Logging.Level,
		FilePath:   cfg.Logging.FilePath,
		JSON:       cfg.Logging.JSON,
		Console:    cfg.Logging.Console,
		MaxSize:    cfg.Logging.MaxSize,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAge,
		Compress:   cfg.Logging.Compress,
	}

	if err := logging.InitFromLogConfig(lc); err != nil {
		// Fall back to defaults on error
		_ = logging.Init(nil)
	}
}
