package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/types"
)

var (
	promiseStatus   string
	promiseBlockers []string
)

func newPromiseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promise <summary>",
		Short: "Publish a completion promise",
		Long: `Publish a completion promise for the current session.

This marks the session as completed and notifies the orchestrator/dashboard.

Examples:
  coders promise "Fixed the authentication bug"
  coders promise "Waiting for API credentials" --status blocked --blockers "Need API key"
  coders promise "Ready for review" --status needs-review`,
		Args: cobra.MinimumNArgs(1),
		RunE: runPromise,
	}

	cmd.Flags().StringVar(&promiseStatus, "status", "completed", "Promise status: completed, blocked, needs-review")
	cmd.Flags().StringSliceVar(&promiseBlockers, "blockers", nil, "Blockers (for blocked status)")

	return cmd
}

func runPromise(cmd *cobra.Command, args []string) error {
	summary := strings.Join(args, " ")

	// Validate status
	var status types.PromiseStatus
	switch promiseStatus {
	case "completed":
		status = types.PromiseCompleted
	case "blocked":
		status = types.PromiseBlocked
	case "needs-review":
		status = types.PromiseNeedsReview
	default:
		return fmt.Errorf("invalid status '%s': must be completed, blocked, or needs-review", promiseStatus)
	}

	// Get session ID from environment or detect from tmux
	sessionID := os.Getenv("CODERS_SESSION_ID")
	if sessionID == "" {
		// Try to detect from current tmux session
		current, err := tmux.GetCurrentSession()
		if err != nil || current == "" {
			return fmt.Errorf("could not determine session ID (set CODERS_SESSION_ID or run inside a coder session)")
		}
		if !strings.HasPrefix(current, tmux.SessionPrefix) {
			return fmt.Errorf("current session '%s' is not a coder session", current)
		}
		sessionID = current
	}

	// Connect to Redis
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create and store the promise
	promise := &types.CoderPromise{
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
		Summary:   summary,
		Status:    status,
		Blockers:  promiseBlockers,
	}

	if err := redisClient.SetPromise(ctx, promise); err != nil {
		return fmt.Errorf("failed to publish promise: %w", err)
	}

	// Print confirmation
	fmt.Printf("\n\033[32m✅ Promise published for: %s\033[0m\n", sessionID)
	fmt.Printf("\033[34m   Summary: %s\033[0m\n", summary)
	fmt.Printf("\033[34m   Status: %s\033[0m\n", promiseStatus)
	if len(promiseBlockers) > 0 {
		fmt.Printf("\033[34m   Blockers: %s\033[0m\n", strings.Join(promiseBlockers, ", "))
	}
	fmt.Printf("\n\033[32mThe orchestrator and dashboard have been notified.\033[0m\n")

	return nil
}
