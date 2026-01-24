package tasksource

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BeadsSource implements TaskSource for beads issue tracker.
type BeadsSource struct {
	cwd  string // Working directory where .beads/ exists
	info SourceInfo
}

// BeadsIssue represents the structure returned by `bd list --json`.
type BeadsIssue struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Assignee    string   `json:"assignee"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	BlockedBy   []string `json:"blockedBy"`
	Blocks      []string `json:"blocks"`
}

// NewBeadsSource creates a new beads source.
func NewBeadsSource(cwd string) (*BeadsSource, error) {
	// Verify bd is available
	if _, err := exec.LookPath("bd"); err != nil {
		return nil, fmt.Errorf("bd command not found: %w", err)
	}

	return &BeadsSource{
		cwd: cwd,
		info: SourceInfo{
			Type:        SourceTypeBeads,
			Name:        "Beads Issue Tracker",
			Description: "Git-backed issue tracker for multi-session work",
			Config: Metadata{
				"cwd": cwd,
			},
		},
	}, nil
}

// Info returns metadata about this source.
func (b *BeadsSource) Info() SourceInfo {
	return b.info
}

// ListTasks returns tasks from beads.
func (b *BeadsSource) ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error) {
	args := []string{"list", "--json"}

	// Apply status filter
	if filter != nil && len(filter.Status) > 0 {
		// Convert TaskStatus to beads status
		statusMap := map[TaskStatus]string{
			TaskStatusOpen:       "open",
			TaskStatusInProgress: "in_progress",
			TaskStatusCompleted:  "closed",
			TaskStatusBlocked:    "blocked",
		}

		for _, status := range filter.Status {
			if beadsStatus, ok := statusMap[status]; ok {
				args = append(args, "--status="+beadsStatus)
			}
		}
	}

	// Execute bd list
	cmd := exec.CommandContext(ctx, "bd", args...)
	cmd.Dir = b.cwd

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd list failed: %w", err)
	}

	// Parse JSON output
	var beadsIssues []BeadsIssue
	if err := json.Unmarshal(output, &beadsIssues); err != nil {
		return nil, fmt.Errorf("failed to parse bd output: %w", err)
	}

	// Convert to normalized tasks
	var tasks []Task
	for _, issue := range beadsIssues {
		task := b.convertBeadsIssue(issue)

		// Apply additional filters
		if filter != nil {
			if filter.OnlyReady && len(task.BlockedBy) > 0 {
				continue
			}
			if filter.Assignee != "" && task.Assignee != filter.Assignee {
				continue
			}
			if len(filter.Priority) > 0 {
				found := false
				for _, p := range filter.Priority {
					if p == task.Priority {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}

		tasks = append(tasks, task)
	}

	// Apply limit
	if filter != nil && filter.Limit > 0 && len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}

	return tasks, nil
}

// GetTask retrieves a specific beads issue.
func (b *BeadsSource) GetTask(ctx context.Context, taskID string) (*Task, error) {
	// Use bd show to get detailed issue info
	cmd := exec.CommandContext(ctx, "bd", "show", taskID, "--json")
	cmd.Dir = b.cwd

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd show failed: %w", err)
	}

	var issue BeadsIssue
	if err := json.Unmarshal(output, &issue); err != nil {
		return nil, fmt.Errorf("failed to parse bd output: %w", err)
	}

	task := b.convertBeadsIssue(issue)
	return &task, nil
}

// UpdateTask updates a beads issue.
func (b *BeadsSource) UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error {
	args := []string{"update", taskID}

	if update.Status != nil {
		statusMap := map[TaskStatus]string{
			TaskStatusOpen:       "open",
			TaskStatusInProgress: "in_progress",
			TaskStatusCompleted:  "closed",
			TaskStatusBlocked:    "blocked",
		}
		if beadsStatus, ok := statusMap[*update.Status]; ok {
			args = append(args, "--status="+beadsStatus)
		}
	}

	if update.Priority != nil {
		args = append(args, fmt.Sprintf("--priority=%d", *update.Priority))
	}

	if update.Assignee != nil {
		args = append(args, "--assignee="+*update.Assignee)
	}

	if update.Description != nil {
		args = append(args, "--description="+*update.Description)
	}

	cmd := exec.CommandContext(ctx, "bd", args...)
	cmd.Dir = b.cwd

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd update failed: %w", err)
	}

	return nil
}

// MarkComplete marks a beads issue as completed.
func (b *BeadsSource) MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error) {
	cmd := exec.CommandContext(ctx, "bd", "close", taskID)
	cmd.Dir = b.cwd

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &CompletionResult{
			Success: false,
			Message: string(output),
			Error:   err,
		}, err
	}

	return &CompletionResult{
		Success: true,
		Message: fmt.Sprintf("Closed beads issue: %s", taskID),
	}, nil
}

// MarkBlocked marks a beads issue as blocked.
func (b *BeadsSource) MarkBlocked(ctx context.Context, taskID string, reason string) error {
	args := []string{"update", taskID, "--status=blocked"}
	if reason != "" {
		args = append(args, "--reason="+reason)
	}

	cmd := exec.CommandContext(ctx, "bd", args...)
	cmd.Dir = b.cwd

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd update failed: %w", err)
	}

	return nil
}

// Close cleans up resources (no-op for beads).
func (b *BeadsSource) Close() error {
	return nil
}

// convertBeadsIssue converts a BeadsIssue to a normalized Task.
func (b *BeadsSource) convertBeadsIssue(issue BeadsIssue) Task {
	// Convert status
	status := TaskStatusOpen
	switch strings.ToLower(issue.Status) {
	case "open":
		status = TaskStatusOpen
	case "in_progress":
		status = TaskStatusInProgress
	case "closed":
		status = TaskStatusCompleted
	case "blocked":
		status = TaskStatusBlocked
	case "cancelled":
		status = TaskStatusCancelled
	}

	// Parse timestamps
	var createdAt, updatedAt *time.Time
	if issue.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			createdAt = &t
		}
	}
	if issue.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, issue.UpdatedAt); err == nil {
			updatedAt = &t
		}
	}

	// Ensure priority is in valid range (0-4)
	priority := TaskPriority(issue.Priority)
	if priority < PriorityCritical {
		priority = PriorityCritical
	}
	if priority > PriorityBacklog {
		priority = PriorityBacklog
	}

	return Task{
		ID:          issue.ID,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      status,
		Priority:    priority,
		Source:      SourceTypeBeads,
		SourceID:    issue.ID,
		SourceMeta: Metadata{
			"type": issue.Type,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Assignee:  issue.Assignee,
		Labels:    issue.Labels,
		BlockedBy: issue.BlockedBy,
		Blocks:    issue.Blocks,
	}
}

// ParseBeadsPriority converts a beads priority (0-4 or P0-P4) to TaskPriority.
func ParseBeadsPriority(p string) TaskPriority {
	// Remove P prefix if present
	p = strings.TrimPrefix(strings.ToUpper(p), "P")

	// Try to parse as int
	if val, err := strconv.Atoi(p); err == nil {
		priority := TaskPriority(val)
		if priority >= PriorityCritical && priority <= PriorityBacklog {
			return priority
		}
	}

	return PriorityMedium
}
