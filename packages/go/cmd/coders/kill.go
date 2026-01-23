package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
)

var (
	killAll       bool
	killCompleted bool
)

func newKillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill [session]",
		Short: "Kill a coder session",
		Long: `Kill a coder session by name or partial match.

Also cleans up the session's Redis promise if present.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runKill,
	}

	cmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill all coder sessions")
	cmd.Flags().BoolVarP(&killCompleted, "completed", "c", false, "Kill all completed sessions")

	return cmd
}

func runKill(cmd *cobra.Command, args []string) error {
	sessions, err := tmux.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No coder sessions found")
		return nil
	}

	// Set up Redis client for promise cleanup
	var redisClient *redis.Client
	redisClient, _ = redis.NewClient() // Ignore error - Redis cleanup is optional
	if redisClient != nil {
		defer redisClient.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get promises if Redis is available
	var promises map[string]bool
	if redisClient != nil {
		if p, err := redisClient.GetPromises(ctx); err == nil {
			promises = make(map[string]bool)
			for k := range p {
				promises[k] = true
			}
		}
	}

	// Kill all sessions
	if killAll {
		killed := 0
		for _, s := range sessions {
			if err := killSessionWithCleanup(s.Name, redisClient, ctx); err == nil {
				fmt.Printf("Killed: %s\n", s.Name)
				killed++
			} else {
				fmt.Printf("Failed to kill %s: %v\n", s.Name, err)
			}
		}
		fmt.Printf("\nKilled %d session(s)\n", killed)
		return nil
	}

	// Kill completed sessions only
	if killCompleted {
		killed := 0
		for _, s := range sessions {
			if promises[s.Name] && !s.IsOrchestrator {
				if err := killSessionWithCleanup(s.Name, redisClient, ctx); err == nil {
					fmt.Printf("Killed: %s\n", s.Name)
					killed++
				} else {
					fmt.Printf("Failed to kill %s: %v\n", s.Name, err)
				}
			}
		}
		if killed == 0 {
			fmt.Println("No completed sessions to kill")
		} else {
			fmt.Printf("\nKilled %d completed session(s)\n", killed)
		}
		return nil
	}

	// Kill specific session
	if len(args) == 0 {
		return fmt.Errorf("specify a session name or use --all/--completed")
	}

	query := args[0]
	var sessionName string

	// First try exact match
	for _, s := range sessions {
		if s.Name == query || s.Name == tmux.SessionPrefix+query {
			sessionName = s.Name
			break
		}
	}

	// Then try partial match
	if sessionName == "" {
		for _, s := range sessions {
			if strings.Contains(s.Name, query) {
				sessionName = s.Name
				break
			}
		}
	}

	if sessionName == "" {
		return fmt.Errorf("no session matching '%s' found", query)
	}

	if err := killSessionWithCleanup(sessionName, redisClient, ctx); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}

	fmt.Printf("Killed: %s\n", sessionName)
	return nil
}

func killSessionWithCleanup(name string, redisClient *redis.Client, ctx context.Context) error {
	// Kill the tmux session
	if err := tmux.KillSession(name); err != nil {
		return err
	}

	// Clean up Redis promise
	if redisClient != nil {
		redisClient.DeletePromise(ctx, name)
	}

	return nil
}
