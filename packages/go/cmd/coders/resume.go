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

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume [session]",
		Short: "Resume a completed session",
		Long: `Resume a completed session by clearing its promise.

This marks the session as active again so it can continue working.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runResume,
	}
}

func runResume(cmd *cobra.Command, args []string) error {
	// Set up Redis client
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all sessions and promises
	sessions, err := tmux.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	promises, err := redisClient.GetPromises(ctx)
	if err != nil {
		return fmt.Errorf("failed to get promises: %w", err)
	}

	// Find completed sessions
	var completedSessions []string
	for _, s := range sessions {
		if _, hasPromise := promises[s.Name]; hasPromise {
			completedSessions = append(completedSessions, s.Name)
		}
	}

	if len(completedSessions) == 0 {
		fmt.Println("No completed sessions to resume")
		return nil
	}

	var sessionName string

	if len(args) == 0 {
		// No argument - resume first completed session
		sessionName = completedSessions[0]
	} else {
		// Find session by name or partial match
		query := args[0]

		// First try exact match
		for _, name := range completedSessions {
			if name == query || name == tmux.SessionPrefix+query {
				sessionName = name
				break
			}
		}

		// Then try partial match
		if sessionName == "" {
			for _, name := range completedSessions {
				if strings.Contains(name, query) {
					sessionName = name
					break
				}
			}
		}

		if sessionName == "" {
			return fmt.Errorf("no completed session matching '%s' found", query)
		}
	}

	// Delete the promise to resume the session
	if err := redisClient.DeletePromise(ctx, sessionName); err != nil {
		return fmt.Errorf("failed to delete promise: %w", err)
	}

	shortName := strings.TrimPrefix(sessionName, tmux.SessionPrefix)
	fmt.Printf("Resumed: %s\n", shortName)
	fmt.Printf("Session is now marked as active\n")

	return nil
}
