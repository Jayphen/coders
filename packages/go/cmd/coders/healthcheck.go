package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/tui"
	"github.com/Jayphen/coders/internal/types"
)

const (
	// healthCheckInterval is how often to run health checks in watch mode.
	healthCheckInterval = 30 * time.Second
	// outputStaleThreshold is how long output can remain unchanged before being considered stuck.
	outputStaleThreshold = 5 * time.Minute
)

var (
	healthCheckJSON  bool
	healthCheckWatch bool
	healthCheckQuiet bool
)

func newHealthcheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "healthcheck",
		Short: "Check health of coder sessions",
		Long: `Run health checks on all coder sessions to detect stuck or unresponsive sessions.

Health checks examine:
- Heartbeat timestamps to detect sessions that haven't reported recently
- tmux session existence to detect terminated sessions
- Pane output changes to detect stuck sessions (output hasn't changed for 5+ minutes)
- Process state to detect unresponsive sessions

Use --watch to run continuously and publish health data to Redis for the dashboard.`,
		RunE: runHealthcheck,
	}

	cmd.Flags().BoolVar(&healthCheckJSON, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&healthCheckWatch, "watch", false, "Run continuously, publishing to Redis")
	cmd.Flags().BoolVar(&healthCheckQuiet, "quiet", false, "Only output problems (non-healthy sessions)")

	return cmd
}

func runHealthcheck(cmd *cobra.Command, args []string) error {
	// Connect to Redis
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	if healthCheckWatch {
		return runHealthcheckWatch(redisClient)
	}

	// One-shot health check
	summary, err := performHealthCheck(redisClient)
	if err != nil {
		return err
	}

	return outputHealthSummary(summary)
}

func runHealthcheckWatch(redisClient *redis.Client) error {
	fmt.Printf("[Healthcheck] Starting watch mode, checking every %v\n", healthCheckInterval)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	// Run immediately, then on interval
	summary, err := performHealthCheck(redisClient)
	if err != nil {
		fmt.Printf("[Healthcheck] Error: %v\n", err)
	} else {
		publishHealthSummary(redisClient, summary)
		fmt.Printf("[Healthcheck] Published at %s - %d healthy, %d stale, %d dead, %d stuck\n",
			time.Now().Format("15:04:05"),
			summary.Healthy, summary.Stale, summary.Dead, summary.Stuck)
	}

	for {
		select {
		case <-ticker.C:
			summary, err := performHealthCheck(redisClient)
			if err != nil {
				fmt.Printf("[Healthcheck] Error: %v\n", err)
				continue
			}
			publishHealthSummary(redisClient, summary)
			fmt.Printf("[Healthcheck] Published at %s - %d healthy, %d stale, %d dead, %d stuck\n",
				time.Now().Format("15:04:05"),
				summary.Healthy, summary.Stale, summary.Dead, summary.Stuck)
		case sig := <-sigChan:
			fmt.Printf("\n[Healthcheck] Received %v, shutting down...\n", sig)
			return nil
		}
	}
}

func performHealthCheck(redisClient *redis.Client) (*types.HealthCheckSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get current tmux sessions
	sessions, err := tmux.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// Get heartbeats and previous health checks from Redis
	heartbeats, _ := redisClient.GetHeartbeats(ctx)
	prevHealthChecks, _ := redisClient.GetHealthChecks(ctx)
	promises, _ := redisClient.GetPromises(ctx)

	now := time.Now()
	summary := &types.HealthCheckSummary{
		Timestamp:     now.UnixMilli(),
		TotalSessions: len(sessions),
		Sessions:      make([]types.HealthCheckResult, 0, len(sessions)),
	}

	for _, session := range sessions {
		result := checkSessionHealth(session, heartbeats[session.Name], prevHealthChecks[session.Name], promises[session.Name], now)

		// Store individual health check result
		if err := redisClient.SetHealthCheck(ctx, &result); err != nil {
			fmt.Printf("[Healthcheck] Failed to store result for %s: %v\n", session.Name, err)
		}

		summary.Sessions = append(summary.Sessions, result)

		// Update counts
		switch result.Status {
		case types.HealthHealthy:
			summary.Healthy++
		case types.HealthStale:
			summary.Stale++
		case types.HealthDead:
			summary.Dead++
		case types.HealthStuck:
			summary.Stuck++
		case types.HealthUnresponsive:
			summary.Unresponsive++
		}
	}

	return summary, nil
}

func checkSessionHealth(session types.Session, heartbeat *types.HeartbeatData, prevCheck *types.HealthCheckResult, promise *types.CoderPromise, now time.Time) types.HealthCheckResult {
	result := types.HealthCheckResult{
		SessionID:      session.Name,
		Timestamp:      now.UnixMilli(),
		TmuxAlive:      true, // We got this session from tmux.ListSessions
		ProcessRunning: true, // Assume running until proven otherwise
	}

	// Check if session has a promise (completed) - skip deep health checks
	if promise != nil {
		result.Status = types.HealthHealthy
		result.Message = "Session completed with promise"
		return result
	}

	// Check tmux pane process
	pids, err := tmux.GetPanePIDs(session.Name)
	if err != nil || len(pids) == 0 {
		result.ProcessRunning = false
		result.Status = types.HealthUnresponsive
		result.Message = "No processes found in tmux pane"
		return result
	}

	// Get current pane output hash for stuck detection
	outputHash := getPaneOutputHash(session.Name)
	result.OutputHash = outputHash

	// Check heartbeat status
	if heartbeat != nil {
		result.HeartbeatAge = now.UnixMilli() - heartbeat.Timestamp

		heartbeatStatus := redis.DetermineHeartbeatStatus(heartbeat)
		switch heartbeatStatus {
		case types.HeartbeatHealthy:
			// Check for stuck output (output unchanged for too long)
			if prevCheck != nil && prevCheck.OutputHash == outputHash && prevCheck.OutputHash != "" {
				// Output hasn't changed since last check
				if prevCheck.OutputStaleFor > 0 {
					result.OutputStaleFor = prevCheck.OutputStaleFor + (now.UnixMilli() - prevCheck.Timestamp)
				} else {
					result.OutputStaleFor = now.UnixMilli() - prevCheck.Timestamp
				}
				result.LastOutputHash = prevCheck.OutputHash
				result.LastCheckTime = prevCheck.Timestamp

				if result.OutputStaleFor > outputStaleThreshold.Milliseconds() {
					result.Status = types.HealthStuck
					result.Message = fmt.Sprintf("Output unchanged for %s", formatDuration(time.Duration(result.OutputStaleFor)*time.Millisecond))
					return result
				}
			} else {
				// Output changed, reset stale counter
				result.OutputStaleFor = 0
			}

			result.Status = types.HealthHealthy
			result.Message = "Session healthy"

		case types.HeartbeatStale:
			result.Status = types.HealthStale
			result.Message = fmt.Sprintf("Heartbeat stale (%s old)", formatDuration(time.Duration(result.HeartbeatAge)*time.Millisecond))

		case types.HeartbeatDead:
			result.Status = types.HealthDead
			result.Message = fmt.Sprintf("No heartbeat for %s", formatDuration(time.Duration(result.HeartbeatAge)*time.Millisecond))
		}
	} else {
		// No heartbeat at all
		// Special case: orchestrator doesn't always have heartbeat
		if session.IsOrchestrator {
			result.Status = types.HealthHealthy
			result.Message = "Orchestrator session"
			return result
		}

		result.Status = types.HealthDead
		result.Message = "No heartbeat data"
	}

	return result
}

func getPaneOutputHash(sessionName string) string {
	// Capture last 50 lines of the pane
	out, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-50").Output()
	if err != nil {
		return ""
	}

	// Hash the output for comparison
	output := strings.TrimSpace(string(out))
	if output == "" {
		return ""
	}

	hash := md5.Sum([]byte(output))
	return hex.EncodeToString(hash[:])
}

func publishHealthSummary(redisClient *redis.Client, summary *types.HealthCheckSummary) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.SetHealthSummary(ctx, summary); err != nil {
		fmt.Printf("[Healthcheck] Failed to publish summary: %v\n", err)
	}
}

func outputHealthSummary(summary *types.HealthCheckSummary) error {
	if healthCheckJSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	if summary.TotalSessions == 0 {
		fmt.Println("No coder sessions found")
		return nil
	}

	// Header
	header := fmt.Sprintf("%-28s %-12s %-12s %s", "SESSION", "STATUS", "HEARTBEAT", "MESSAGE")
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(tui.ColorGray).Render(header))
	fmt.Println(strings.Repeat("-", 80))

	for _, result := range summary.Sessions {
		if healthCheckQuiet && result.Status == types.HealthHealthy {
			continue
		}

		name := strings.TrimPrefix(result.SessionID, tmux.SessionPrefix)
		if len(name) > 26 {
			name = name[:23] + "..."
		}

		// Status styling
		var statusStr string
		switch result.Status {
		case types.HealthHealthy:
			statusStr = tui.StatusHealthy.Render("● healthy")
		case types.HealthStale:
			statusStr = tui.StatusStale.Render("◐ stale")
		case types.HealthDead:
			statusStr = tui.StatusDead.Render("○ dead")
		case types.HealthStuck:
			statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render("◉ stuck")
		case types.HealthUnresponsive:
			statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render("✗ unresponsive")
		}

		// Heartbeat age
		heartbeatStr := "-"
		if result.HeartbeatAge > 0 {
			heartbeatStr = formatDuration(time.Duration(result.HeartbeatAge) * time.Millisecond)
		}

		// Message
		message := result.Message
		if len(message) > 30 {
			message = message[:27] + "..."
		}

		fmt.Printf("%-28s %-12s %-12s %s\n", name, statusStr, heartbeatStr, message)
	}

	fmt.Println()
	fmt.Printf("Summary: %d total, %d healthy, %d stale, %d dead, %d stuck, %d unresponsive\n",
		summary.TotalSessions, summary.Healthy, summary.Stale, summary.Dead, summary.Stuck, summary.Unresponsive)

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
