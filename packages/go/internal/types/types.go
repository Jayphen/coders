// Package types defines the core data types used throughout the coders CLI.
package types

import "time"

// Session represents a coder session (tmux session running an AI coding tool).
type Session struct {
	Name            string          `json:"name"`
	Tool            string          `json:"tool"` // claude, gemini, codex, opencode, unknown
	Task            string          `json:"task,omitempty"`
	Cwd             string          `json:"cwd"`
	CreatedAt       *time.Time      `json:"createdAt,omitempty"`
	ParentSessionID string          `json:"parentSessionId,omitempty"`
	IsOrchestrator  bool            `json:"isOrchestrator"`
	HeartbeatStatus HeartbeatStatus `json:"heartbeatStatus,omitempty"`
	LastActivity    *time.Time      `json:"lastActivity,omitempty"`
	Promise         *CoderPromise   `json:"promise,omitempty"`
	HasPromise      bool            `json:"hasPromise"`
	Usage           *UsageStats     `json:"usage,omitempty"`
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
	Cost               string  `json:"cost,omitempty"`
	Tokens             int     `json:"tokens,omitempty"`
	APICalls           int     `json:"apiCalls,omitempty"`
	SessionLimitPct    float64 `json:"sessionLimitPercent,omitempty"`
	WeeklyLimitPct     float64 `json:"weeklyLimitPercent,omitempty"`
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
