package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/redis"
)

var loopStatusID string

func newLoopStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loop-status",
		Short: "Check the status of a running loop",
		Long: `Check the status of a loop runner.

Shows the current state including:
  - Current task index and total tasks
  - Current tool being used
  - Loop status (running, completed, paused)

Examples:
  coders loop-status --loop-id loop-1234567890
  coders loop-status  # Lists all active loops`,
		RunE: runLoopStatus,
	}

	cmd.Flags().StringVar(&loopStatusID, "loop-id", "", "Loop ID to check (lists all if not specified)")

	return cmd
}

func runLoopStatus(cmd *cobra.Command, args []string) error {
	rdb, err := redis.GetClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	ctx := context.Background()

	if loopStatusID != "" {
		// Show specific loop
		return showLoopStatus(ctx, rdb, loopStatusID)
	}

	// List all loops
	return listAllLoops(ctx, rdb)
}

func showLoopStatus(ctx context.Context, rdb *redis.Client, loopID string) error {
	state, err := getLoopState(ctx, rdb, loopID)
	if err != nil {
		return fmt.Errorf("failed to get loop state: %w", err)
	}

	if state == nil {
		fmt.Printf("\033[33m⚠️  No loop found with ID: %s\033[0m\n", loopID)
		return nil
	}

	printLoopState(state)
	return nil
}

func listAllLoops(ctx context.Context, rdb *redis.Client) error {
	states, err := getAllLoopStates(ctx, rdb)
	if err != nil {
		return fmt.Errorf("failed to get loop states: %w", err)
	}

	if len(states) == 0 {
		fmt.Println("No active loops found.")
		return nil
	}

	fmt.Printf("Found %d loop(s):\n\n", len(states))

	for _, state := range states {
		printLoopState(state)
		fmt.Println()
	}

	return nil
}

func printLoopState(state *LoopState) {
	statusColor := "\033[33m" // yellow
	statusIcon := "⏸️"

	switch state.Status {
	case "running":
		statusColor = "\033[34m" // blue
		statusIcon = "🔄"
	case "completed":
		statusColor = "\033[32m" // green
		statusIcon = "✅"
	case "paused":
		statusColor = "\033[33m" // yellow
		statusIcon = "⏸️"
	}

	fmt.Printf("%s%s Loop: %s\033[0m\n", statusColor, statusIcon, state.LoopID)
	fmt.Printf("   📋 Status: %s\n", state.Status)
	fmt.Printf("   📂 Todolist: %s\n", state.TodolistPath)
	fmt.Printf("   📁 Working directory: %s\n", state.Cwd)
	fmt.Printf("   🤖 Tool: %s\n", state.CurrentTool)
	fmt.Printf("   📊 Progress: %d/%d tasks\n", state.CurrentTaskIndex, state.TotalTasks)

	if state.TotalTasks > 0 {
		pct := float64(state.CurrentTaskIndex) / float64(state.TotalTasks) * 100
		fmt.Printf("   📈 %.0f%% complete\n", pct)
	}
}

// getLoopState retrieves the state of a specific loop.
func getLoopState(ctx context.Context, rdb *redis.Client, loopID string) (*LoopState, error) {
	key := loopStateKeyPrefix + loopID

	data, err := rdb.GetRaw(ctx, key)
	if err != nil {
		return nil, nil // Key doesn't exist
	}

	var state LoopState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// getAllLoopStates retrieves all loop states.
func getAllLoopStates(ctx context.Context, rdb *redis.Client) ([]*LoopState, error) {
	keys, err := rdb.ScanKeys(ctx, loopStateKeyPrefix+"*")
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, nil
	}

	values, err := rdb.MGetRaw(ctx, keys)
	if err != nil {
		return nil, err
	}

	var states []*LoopState
	for _, val := range values {
		if val == "" {
			continue
		}

		var state LoopState
		if err := json.Unmarshal([]byte(val), &state); err != nil {
			continue
		}
		states = append(states, &state)
	}

	return states, nil
}
