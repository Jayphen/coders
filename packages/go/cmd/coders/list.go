package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/tui"
	"github.com/Jayphen/coders/internal/types"
)

var (
	listJSON   bool
	listStatus string
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List coder sessions",
		Long:  `List all coder sessions with their status and details.`,
		RunE:  runList,
	}

	cmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (active, completed)")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	// Get tmux sessions
	sessions, err := tmux.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Try to get Redis data
	var promises map[string]*types.CoderPromise
	var heartbeats map[string]*types.HeartbeatData

	redisClient, err := redis.NewClient()
	if err == nil {
		defer redisClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		promises, _ = redisClient.GetPromises(ctx)
		heartbeats, _ = redisClient.GetHeartbeats(ctx)
	}

	// Enrich sessions with Redis data
	for i := range sessions {
		s := &sessions[i]

		if promise, ok := promises[s.Name]; ok {
			s.Promise = promise
			s.HasPromise = true
		}

		if hb, ok := heartbeats[s.Name]; ok {
			s.HeartbeatStatus = redis.DetermineHeartbeatStatus(hb)
			if hb.Task != "" && s.Task == "" {
				s.Task = hb.Task
			}
			if hb.ParentSessionID != "" {
				s.ParentSessionID = hb.ParentSessionID
			}
			s.Usage = hb.Usage
		} else if s.IsOrchestrator {
			s.HeartbeatStatus = types.HeartbeatHealthy
		} else {
			s.HeartbeatStatus = types.HeartbeatDead
		}
	}

	// Filter by status if specified
	if listStatus != "" {
		var filtered []types.Session
		for _, s := range sessions {
			switch listStatus {
			case "active":
				if !s.HasPromise {
					filtered = append(filtered, s)
				}
			case "completed":
				if s.HasPromise {
					filtered = append(filtered, s)
				}
			}
		}
		sessions = filtered
	}

	// Sort: orchestrator first, then active, then completed
	sort.Slice(sessions, func(i, j int) bool {
		a, b := sessions[i], sessions[j]
		if a.IsOrchestrator {
			return true
		}
		if b.IsOrchestrator {
			return false
		}
		if a.HasPromise != b.HasPromise {
			return !a.HasPromise
		}
		if a.CreatedAt != nil && b.CreatedAt != nil {
			return a.CreatedAt.After(*b.CreatedAt)
		}
		return false
	})

	// Output
	if listJSON {
		data, err := json.MarshalIndent(sessions, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	if len(sessions) == 0 {
		fmt.Println("No coder sessions found")
		return nil
	}

	printSessionTable(sessions)
	return nil
}

func printSessionTable(sessions []types.Session) {
	// Header
	header := fmt.Sprintf("%-28s %-10s %-20s %-8s", "SESSION", "TOOL", "TASK/SUMMARY", "STATUS")
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(tui.ColorGray).Render(header))
	fmt.Println(strings.Repeat("-", 70))

	for _, s := range sessions {
		// Name
		name := strings.TrimPrefix(s.Name, tmux.SessionPrefix)
		if s.IsOrchestrator {
			name = "🎯 orchestrator"
		}
		if len(name) > 26 {
			name = name[:23] + "..."
		}
		nameStyle := lipgloss.NewStyle()
		if s.IsOrchestrator {
			nameStyle = nameStyle.Foreground(tui.ColorCyan).Bold(true)
		} else if s.HasPromise {
			nameStyle = nameStyle.Foreground(tui.ColorGray)
		}

		// Tool
		toolStyle := tui.GetToolStyle(s.Tool)
		if s.HasPromise {
			toolStyle = toolStyle.Foreground(tui.ColorDimGray)
		}

		// Task/Summary
		displayText := s.Task
		if s.Promise != nil {
			displayText = s.Promise.Summary
		}
		if displayText == "" {
			displayText = "-"
		}
		if len(displayText) > 18 {
			displayText = displayText[:15] + "..."
		}

		// Status
		var status string
		if s.Promise != nil {
			switch s.Promise.Status {
			case types.PromiseCompleted:
				status = tui.PromiseCompleted.Render("✓ completed")
			case types.PromiseBlocked:
				status = tui.PromiseBlocked.Render("! blocked")
			case types.PromiseNeedsReview:
				status = tui.PromiseNeedsReview.Render("? review")
			}
		} else {
			switch s.HeartbeatStatus {
			case types.HeartbeatHealthy:
				status = tui.StatusHealthy.Render("● healthy")
			case types.HeartbeatStale:
				status = tui.StatusStale.Render("◐ stale")
			default:
				status = tui.StatusDead.Render("○ dead")
			}
		}

		fmt.Printf("%-28s %-10s %-20s %s\n",
			nameStyle.Render(name),
			toolStyle.Render(s.Tool),
			displayText,
			status,
		)
	}

	fmt.Println()
	activeCount := 0
	completedCount := 0
	for _, s := range sessions {
		if s.HasPromise {
			completedCount++
		} else {
			activeCount++
		}
	}
	fmt.Printf("Total: %d active, %d completed\n", activeCount, completedCount)
}
