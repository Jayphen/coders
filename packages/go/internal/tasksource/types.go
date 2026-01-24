// Package tasksource provides interfaces and types for multi-source task management.
package tasksource

import (
	"time"
)

// TaskStatus represents the state of a task.
type TaskStatus string

const (
	TaskStatusOpen       TaskStatus = "open"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskPriority represents task priority levels.
type TaskPriority int

const (
	PriorityCritical  TaskPriority = 0 // P0
	PriorityHigh      TaskPriority = 1 // P1
	PriorityMedium    TaskPriority = 2 // P2
	PriorityLow       TaskPriority = 3 // P3
	PriorityBacklog   TaskPriority = 4 // P4
)

// SourceType identifies which system the task originated from.
type SourceType string

const (
	SourceTypeTodolist SourceType = "todolist" // Markdown/text todolist files
	SourceTypeBeads    SourceType = "beads"    // Beads issue tracker
	SourceTypeLinear   SourceType = "linear"   // Linear issues
	SourceTypeGitHub   SourceType = "github"   // GitHub issues
)

// Task represents a normalized task from any source.
type Task struct {
	// Core fields
	ID          string       `json:"id"`          // Source-specific ID (e.g., "beads-123", "todo-1")
	Title       string       `json:"title"`       // Task title/description
	Description string       `json:"description"` // Full description (optional)
	Status      TaskStatus   `json:"status"`      // Current status
	Priority    TaskPriority `json:"priority"`    // Priority level (0-4)

	// Source tracking
	Source     SourceType `json:"source"`     // Which system this came from
	SourceID   string     `json:"sourceId"`   // Original ID in source system
	SourceMeta Metadata   `json:"sourceMeta"` // Source-specific metadata

	// Optional fields
	CreatedAt  *time.Time `json:"createdAt,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	Assignee   string     `json:"assignee,omitempty"`
	Labels     []string   `json:"labels,omitempty"`
	BlockedBy  []string   `json:"blockedBy,omitempty"` // IDs of blocking tasks
	Blocks     []string   `json:"blocks,omitempty"`    // IDs of tasks this blocks
}

// Metadata stores source-specific data.
type Metadata map[string]interface{}

// TaskFilter specifies criteria for listing tasks.
type TaskFilter struct {
	Status    []TaskStatus   `json:"status,omitempty"`    // Filter by status
	Priority  []TaskPriority `json:"priority,omitempty"`  // Filter by priority
	Assignee  string         `json:"assignee,omitempty"`  // Filter by assignee
	Labels    []string       `json:"labels,omitempty"`    // Filter by labels (AND)
	Limit     int            `json:"limit,omitempty"`     // Max tasks to return
	OnlyReady bool           `json:"onlyReady,omitempty"` // Only tasks with no blockers
}

// TaskUpdate contains fields to update on a task.
type TaskUpdate struct {
	Status      *TaskStatus   `json:"status,omitempty"`
	Priority    *TaskPriority `json:"priority,omitempty"`
	Assignee    *string       `json:"assignee,omitempty"`
	AddLabels   []string      `json:"addLabels,omitempty"`
	RemoveLabels []string     `json:"removeLabels,omitempty"`
	Description *string       `json:"description,omitempty"`
}

// SourceInfo provides metadata about a task source.
type SourceInfo struct {
	Type        SourceType `json:"type"`
	Name        string     `json:"name"`        // Display name
	Description string     `json:"description"` // What this source provides
	Config      Metadata   `json:"config"`      // Source-specific config
}

// CompletionResult contains information about completing a task.
type CompletionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   error  `json:"error,omitempty"`
}
