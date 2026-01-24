package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/logging"
	"github.com/Jayphen/coders/internal/redis"
	"github.com/Jayphen/coders/internal/types"
)

var (
	heartbeatSessionID string
	heartbeatPaneID    string
	heartbeatTask      string
	heartbeatParent    string
)

func newHeartbeatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Run heartbeat monitor for a session",
		Long: `Run a background heartbeat monitor that publishes session status to Redis.

This is typically started automatically by 'coders spawn' when --heartbeat is enabled.
It publishes heartbeat data every 30 seconds including usage statistics.`,
		RunE: runHeartbeat,
	}

	cmd.Flags().StringVar(&heartbeatSessionID, "session", "", "Session ID (or use CODERS_SESSION_ID env)")
	cmd.Flags().StringVar(&heartbeatPaneID, "pane", "", "Pane ID (auto-generated if not provided)")
	cmd.Flags().StringVar(&heartbeatTask, "task", "", "Task description (or use CODERS_TASK_DESC env)")
	cmd.Flags().StringVar(&heartbeatParent, "parent", "", "Parent session ID (or use CODERS_PARENT_SESSION_ID env)")

	return cmd
}

func runHeartbeat(cmd *cobra.Command, args []string) error {
	log := logging.WithCommand("heartbeat")

	// Load config for heartbeat interval
	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Error("failed to load config")
		return fmt.Errorf("failed to load config: %w", err)
	}
	heartbeatInterval := cfg.HeartbeatInterval

	// Get session ID from flag, env, or args
	sessionID := heartbeatSessionID
	if sessionID == "" {
		sessionID = os.Getenv("CODERS_SESSION_ID")
	}
	if sessionID == "" && len(args) > 0 {
		sessionID = args[0]
	}
	if sessionID == "" {
		log.Error("session ID required")
		return fmt.Errorf("session ID required (use --session, CODERS_SESSION_ID env, or pass as argument)")
	}

	// Create logger with session context
	log = log.WithSessionID(sessionID)

	// Get other params from flags or env
	paneID := heartbeatPaneID
	if paneID == "" {
		paneID = os.Getenv("PANE_ID")
	}
	if paneID == "" {
		paneID = fmt.Sprintf("pane-%d", time.Now().UnixNano())
	}

	task := heartbeatTask
	if task == "" {
		task = os.Getenv("CODERS_TASK_DESC")
	}

	parent := heartbeatParent
	if parent == "" {
		parent = os.Getenv("CODERS_PARENT_SESSION_ID")
	}

	// Connect to Redis
	redisClient, err := redis.NewClient()
	if err != nil {
		log.WithError(err).Error("failed to connect to Redis")
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	log.WithField("interval", heartbeatInterval.String()).Info("heartbeat started")
	fmt.Printf("[Heartbeat] Started for session: %s\n", sessionID)
	fmt.Printf("[Heartbeat] Publishing every %v\n", heartbeatInterval)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for heartbeat
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Publish immediately, then on interval
	publishHeartbeat(log, redisClient, sessionID, paneID, task, parent)

	for {
		select {
		case <-ticker.C:
			publishHeartbeat(log, redisClient, sessionID, paneID, task, parent)
		case sig := <-sigChan:
			log.WithField("signal", sig.String()).Info("received shutdown signal")
			fmt.Printf("\n[Heartbeat] Received %v, shutting down...\n", sig)
			return nil
		}
	}
}

func publishHeartbeat(log *logging.Logger, client *redis.Client, sessionID, paneID, task, parent string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get usage stats from tmux pane
	usage := getUsageStats(sessionID)

	hb := &types.HeartbeatData{
		PaneID:          paneID,
		SessionID:       sessionID,
		Timestamp:       time.Now().UnixMilli(),
		Status:          "running",
		Task:            task,
		ParentSessionID: parent,
		Usage:           usage,
	}

	if err := client.SetHeartbeat(ctx, hb); err != nil {
		log.WithError(err).Warn("failed to publish heartbeat")
		fmt.Printf("[Heartbeat] Failed to publish: %v\n", err)
		return
	}

	log.Debug("heartbeat published")
	fmt.Printf("[Heartbeat] Published at %s\n", time.Now().Format("15:04:05"))
}

// getUsageStats captures and parses usage statistics from the tmux pane.
func getUsageStats(sessionID string) *types.UsageStats {
	// Capture last 100 lines of the pane
	out, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionID, "-S", "-100").Output()
	if err != nil {
		return nil
	}

	output := string(out)
	if output == "" {
		return nil
	}

	stats := &types.UsageStats{}
	lines := strings.Split(output, "\n")

	// Check for Claude TUI visual usage patterns (multi-line)
	sessionPercentRe := regexp.MustCompile(`Current session\s*\n[█\s]*(\d+)%\s*used`)
	if match := sessionPercentRe.FindStringSubmatch(output); len(match) > 1 {
		if pct, err := strconv.ParseFloat(match[1], 64); err == nil {
			stats.SessionLimitPct = pct
		}
	}

	weeklyPercentRe := regexp.MustCompile(`Current week \(all models\)\s*\n[█\s]*(\d+)%\s*used`)
	if match := weeklyPercentRe.FindStringSubmatch(output); len(match) > 1 {
		if pct, err := strconv.ParseFloat(match[1], 64); err == nil {
			stats.WeeklyLimitPct = pct
		}
	}

	// Reverse iterate to find the most recent stats
	costRe := regexp.MustCompile(`(?i)(?:Total )?[Cc]ost:\s*\$([0-9.]+)`)
	tokensRe := regexp.MustCompile(`(?i)(?:Total )?[Tt]okens:\s*(\d+)`)
	apiCallsRe := regexp.MustCompile(`(?i)API calls:\s*(\d+)`)

	for i := len(lines) - 1; i >= 0 && i > len(lines)-50; i-- {
		line := strings.TrimSpace(lines[i])

		if stats.Cost == "" {
			if match := costRe.FindStringSubmatch(line); len(match) > 1 {
				stats.Cost = "$" + match[1]
			}
		}

		if stats.Tokens == 0 {
			if match := tokensRe.FindStringSubmatch(line); len(match) > 1 {
				if tokens, err := strconv.Atoi(match[1]); err == nil {
					stats.Tokens = tokens
				}
			}
		}

		if stats.APICalls == 0 {
			if match := apiCallsRe.FindStringSubmatch(line); len(match) > 1 {
				if calls, err := strconv.Atoi(match[1]); err == nil {
					stats.APICalls = calls
				}
			}
		}

		// Stop if we found cost and tokens
		if stats.Cost != "" && stats.Tokens > 0 {
			break
		}
	}

	// Return nil if no stats found
	if stats.Cost == "" && stats.Tokens == 0 && stats.APICalls == 0 &&
		stats.SessionLimitPct == 0 && stats.WeeklyLimitPct == 0 {
		return nil
	}

	return stats
}
