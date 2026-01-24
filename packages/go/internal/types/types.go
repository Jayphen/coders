// Package types defines the core data types used throughout the coders CLI.
package types

import "time"

// Session represents a coder session (tmux session running an AI coding tool).
type Session struct {
	Name            string             `json:"name"`
	Tool            string             `json:"tool"` // claude, gemini, codex, opencode, unknown
	Task            string             `json:"task,omitempty"`
	Cwd             string             `json:"cwd"`
	CreatedAt       *time.Time         `json:"createdAt,omitempty"`
	ParentSessionID string             `json:"parentSessionId,omitempty"`
	IsOrchestrator  bool               `json:"isOrchestrator"`
	HeartbeatStatus HeartbeatStatus    `json:"heartbeatStatus,omitempty"`
	HealthCheck     *HealthCheckResult `json:"healthCheck,omitempty"`
	LastActivity    *time.Time         `json:"lastActivity,omitempty"`
	Promise         *CoderPromise      `json:"promise,omitempty"`
	HasPromise      bool               `json:"hasPromise"`
	Usage           *UsageStats        `json:"usage,omitempty"`
}

// HeartbeatStatus indicates the health of a session based on heartbeat age.
type HeartbeatStatus string

const (
	HeartbeatHealthy HeartbeatStatus = "healthy" // < 1 min
	HeartbeatStale   HeartbeatStatus = "stale"   // < 5 min
	HeartbeatDead    HeartbeatStatus = "dead"    // >= 5 min or no heartbeat
)

// HeartbeatData is the data stored in Redis for session heartbeats.
type HeartbeatData struct {
	PaneID          string      `json:"paneId"`
	SessionID       string      `json:"sessionId"`
	Timestamp       int64       `json:"timestamp"`
	Status          string      `json:"status"`
	LastActivity    string      `json:"lastActivity,omitempty"`
	ParentSessionID string      `json:"parentSessionId,omitempty"`
	Task            string      `json:"task,omitempty"`
	Usage           *UsageStats `json:"usage,omitempty"`
}

// CoderPromise represents a completion promise published by a coder session.
type CoderPromise struct {
	SessionID    string        `json:"sessionId"`
	Timestamp    int64         `json:"timestamp"`
	Summary      string        `json:"summary"`
	Status       PromiseStatus `json:"status"`
	FilesChanged []string      `json:"filesChanged,omitempty"`
	Blockers     []string      `json:"blockers,omitempty"`
}

// PromiseStatus indicates the state of a completion promise.
type PromiseStatus string

const (
	PromiseCompleted   PromiseStatus = "completed"
	PromiseBlocked     PromiseStatus = "blocked"
	PromiseNeedsReview PromiseStatus = "needs-review"
)

// UsageStats contains usage statistics for a session.
type UsageStats struct {
	Cost            string  `json:"cost,omitempty"`
	Tokens          int     `json:"tokens,omitempty"`
	APICalls        int     `json:"apiCalls,omitempty"`
	SessionLimitPct float64 `json:"sessionLimitPercent,omitempty"`
	WeeklyLimitPct  float64 `json:"weeklyLimitPercent,omitempty"`
}

// HealthStatus indicates the overall health of a session.
type HealthStatus string

const (
	HealthHealthy      HealthStatus = "healthy"      // Active and responsive
	HealthStale        HealthStatus = "stale"        // Heartbeat aging (1-5 min)
	HealthDead         HealthStatus = "dead"         // No heartbeat (>= 5 min)
	HealthStuck        HealthStatus = "stuck"        // Pane output not changing for too long
	HealthUnresponsive HealthStatus = "unresponsive" // tmux session exists but process seems hung
)

// HealthCheckResult contains the results of a health check for a session.
type HealthCheckResult struct {
	SessionID       string       `json:"sessionId"`
	Timestamp       int64        `json:"timestamp"`
	Status          HealthStatus `json:"status"`
	HeartbeatAge    int64        `json:"heartbeatAgeMs,omitempty"`    // Milliseconds since last heartbeat
	OutputHash      string       `json:"outputHash,omitempty"`        // Hash of recent pane output for change detection
	OutputStaleFor  int64        `json:"outputStaleForMs,omitempty"`  // How long output has been unchanged
	ProcessRunning  bool         `json:"processRunning"`              // Whether tmux pane process is alive
	TmuxAlive       bool         `json:"tmuxAlive"`                   // Whether tmux session exists
	Message         string       `json:"message,omitempty"`           // Human-readable status message
	LastOutputHash  string       `json:"lastOutputHash,omitempty"`    // Previous output hash for comparison
	LastCheckTime   int64        `json:"lastCheckTime,omitempty"`     // When last check was performed
}

// HealthCheckSummary provides an overview of all session health.
type HealthCheckSummary struct {
	Timestamp     int64               `json:"timestamp"`
	TotalSessions int                 `json:"totalSessions"`
	Healthy       int                 `json:"healthy"`
	Stale         int                 `json:"stale"`
	Dead          int                 `json:"dead"`
	Stuck         int                 `json:"stuck"`
	Unresponsive  int                 `json:"unresponsive"`
	Sessions      []HealthCheckResult `json:"sessions"`
}

// ValidTools is the list of known AI coding tools.
var ValidTools = []string{"claude", "gemini", "codex", "opencode"}

// IsValidTool checks if a tool name is recognized.
func IsValidTool(tool string) bool {
	for _, t := range ValidTools {
		if t == tool {
			return true
		}
	}
	return false
}

// ToolColors maps tool names to their display colors.
var ToolColors = map[string]string{
	"claude":   "magenta",
	"gemini":   "blue",
	"codex":    "green",
	"opencode": "yellow",
	"unknown":  "gray",
}

// SessionState stores the state needed to restart a crashed session.
type SessionState struct {
	SessionID       string `json:"sessionId"`
	SessionName     string `json:"sessionName"`
	Tool            string `json:"tool"`
	Task            string `json:"task"`
	Cwd             string `json:"cwd"`
	Model           string `json:"model,omitempty"`
	UseOllama       bool   `json:"useOllama,omitempty"`
	HeartbeatEnabled bool   `json:"heartbeatEnabled"`
	RestartOnCrash  bool   `json:"restartOnCrash"`
	RestartCount    int    `json:"restartCount"`
	MaxRestarts     int    `json:"maxRestarts"`
	CreatedAt       int64  `json:"createdAt"`
	LastRestartAt   int64  `json:"lastRestartAt,omitempty"`
}

// CrashEvent records when a session crashed.
type CrashEvent struct {
	SessionID   string `json:"sessionId"`
	Timestamp   int64  `json:"timestamp"`
	Reason      string `json:"reason"`
	WillRestart bool   `json:"willRestart"`
}

// LoopNotification represents a notification sent when a loop completes.
type LoopNotification struct {
	LoopID    string `json:"loopId"`
	Timestamp int64  `json:"timestamp"`
	TaskCount int    `json:"taskCount"`
	Status    string `json:"status"` // completed, paused, failed
	Message   string `json:"message,omitempty"`
}
