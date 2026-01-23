package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `Manage coders configuration files.`,
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigPathCmd())

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  `Display the current configuration values from all sources.`,
		RunE:  runConfigShow,
	}
}

func newConfigInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create example configuration file",
		Long: `Create an example configuration file at ~/.config/coders/config.yaml.

The generated file contains all available options with their default values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config file")

	return cmd
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		Long:  `Display the paths where configuration files are searched.`,
		RunE:  runConfigPath,
	}
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Get()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Current configuration:")
	fmt.Println()
	fmt.Printf("  default_tool:       %s\n", cfg.DefaultTool)
	fmt.Printf("  heartbeat_interval: %s\n", cfg.HeartbeatInterval)
	fmt.Printf("  redis_url:          %s\n", cfg.RedisURL)
	fmt.Printf("  dashboard_port:     %d\n", cfg.DashboardPort)
	fmt.Printf("  default_model:      %s\n", valueOrDefault(cfg.DefaultModel, "(not set)"))
	fmt.Printf("  default_heartbeat:  %t\n", cfg.DefaultHeartbeat)
	fmt.Println()
	fmt.Println("  Ollama:")
	fmt.Printf("    base_url:   %s\n", valueOrDefault(cfg.Ollama.BaseURL, "(not set)"))
	fmt.Printf("    auth_token: %s\n", maskSecret(cfg.Ollama.AuthToken))
	fmt.Printf("    api_key:    %s\n", maskSecret(cfg.Ollama.APIKey))

	return nil
}

func runConfigInit(force bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".config", "coders", "config.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
	}

	if err := config.WriteExample(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created config file at: %s\n", configPath)
	fmt.Println()
	fmt.Println("Edit this file to customize your settings.")
	fmt.Println("Run 'coders config show' to see current values.")

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuration file search paths (in priority order):")
	fmt.Println()

	paths := config.ConfigPaths()
	for i, p := range paths {
		exists := "not found"
		if _, err := os.Stat(p); err == nil {
			exists = "found"
		}
		fmt.Printf("  %d. %s (%s)\n", i+1, p, exists)
	}

	fmt.Println()
	fmt.Println("Environment variables can override file settings.")
	fmt.Println("Supported env vars:")
	fmt.Println("  CODERS_DEFAULT_TOOL")
	fmt.Println("  CODERS_HEARTBEAT_INTERVAL")
	fmt.Println("  CODERS_REDIS_URL (or REDIS_URL)")
	fmt.Println("  CODERS_DASHBOARD_PORT")
	fmt.Println("  CODERS_DEFAULT_MODEL")
	fmt.Println("  CODERS_DEFAULT_HEARTBEAT")
	fmt.Println("  CODERS_OLLAMA_BASE_URL")
	fmt.Println("  CODERS_OLLAMA_AUTH_TOKEN")
	fmt.Println("  CODERS_OLLAMA_API_KEY")

	return nil
}

func valueOrDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func maskSecret(val string) string {
	if val == "" {
		return "(not set)"
	}
	if len(val) <= 8 {
		return "***"
	}
	return val[:4] + "..." + val[len(val)-4:]
}
