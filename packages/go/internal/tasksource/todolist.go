package tasksource

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// TodolistSource implements TaskSource for markdown/text todolist files.
// Format: [ ] Task description or [x] Completed task
type TodolistSource struct {
	filePath string
	mu       sync.RWMutex
	info     SourceInfo
}

// NewTodolistSource creates a new todolist source from a file.
func NewTodolistSource(filePath string) (*TodolistSource, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("todolist file not found: %w", err)
	}

	return &TodolistSource{
		filePath: absPath,
		info: SourceInfo{
			Type:        SourceTypeTodolist,
			Name:        filepath.Base(absPath),
			Description: fmt.Sprintf("Todolist file: %s", absPath),
			Config: Metadata{
				"path": absPath,
			},
		},
	}, nil
}

// Info returns metadata about this source.
func (t *TodolistSource) Info() SourceInfo {
	return t.info
}

// ListTasks returns tasks from the todolist file.
func (t *TodolistSource) ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	file, err := os.Open(t.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open todolist: %w", err)
	}
	defer file.Close()

	var tasks []Task
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Match tasks: [ ] or [x]
	uncompletedRegex := regexp.MustCompile(`^\[\ \]\s*(.+)$`)
	completedRegex := regexp.MustCompile(`^\[x\]\s*(.+)$`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var status TaskStatus
		var description string

		if matches := uncompletedRegex.FindStringSubmatch(line); len(matches) > 1 {
			status = TaskStatusOpen
			description = strings.TrimSpace(matches[1])
		} else if matches := completedRegex.FindStringSubmatch(line); len(matches) > 1 {
			status = TaskStatusCompleted
			description = strings.TrimSpace(matches[1])
		} else {
			continue // Not a task line
		}

		// Generate task ID from line number
		taskID := fmt.Sprintf("todo-%s-%d", filepath.Base(t.filePath), lineNum)

		task := Task{
			ID:          taskID,
			Title:       description,
			Description: description,
			Status:      status,
			Priority:    PriorityMedium, // Default priority
			Source:      SourceTypeTodolist,
			SourceID:    fmt.Sprintf("%d", lineNum),
			SourceMeta: Metadata{
				"line":     lineNum,
				"filePath": t.filePath,
				"rawLine":  line,
			},
		}

		// Apply filters
		if filter != nil {
			if len(filter.Status) > 0 {
				found := false
				for _, s := range filter.Status {
					if s == status {
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

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading todolist: %w", err)
	}

	// Apply limit
	if filter != nil && filter.Limit > 0 && len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}

	return tasks, nil
}

// GetTask retrieves a specific task by ID.
func (t *TodolistSource) GetTask(ctx context.Context, taskID string) (*Task, error) {
	tasks, err := t.ListTasks(ctx, nil)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.ID == taskID {
			return &task, nil
		}
	}

	return nil, ErrTaskNotFound
}

// UpdateTask updates a task (limited support for todolist files).
func (t *TodolistSource) UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error {
	// Todolist files only support status changes
	if update.Status != nil {
		if *update.Status == TaskStatusCompleted {
			_, err := t.MarkComplete(ctx, taskID)
			return err
		}
	}

	return ErrNotSupported
}

// MarkComplete marks a task as completed by replacing [ ] with [x].
func (t *TodolistSource) MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get the task to find its description
	task, err := t.getTaskUnlocked(ctx, taskID)
	if err != nil {
		return &CompletionResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Read file
	content, err := os.ReadFile(t.filePath)
	if err != nil {
		return &CompletionResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Escape special regex characters in task description
	escaped := regexp.QuoteMeta(task.Title)
	pattern := regexp.MustCompile(`\[\ \]\s*` + escaped)

	// Replace [ ] with [x]
	newContent := pattern.ReplaceAllString(string(content), "[x] "+task.Title)

	// Write back
	if err := os.WriteFile(t.filePath, []byte(newContent), 0644); err != nil {
		return &CompletionResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &CompletionResult{
		Success: true,
		Message: fmt.Sprintf("Marked task complete in %s", t.filePath),
	}, nil
}

// MarkBlocked is not supported for todolist files.
func (t *TodolistSource) MarkBlocked(ctx context.Context, taskID string, reason string) error {
	return ErrNotSupported
}

// Close cleans up resources (no-op for todolist).
func (t *TodolistSource) Close() error {
	return nil
}

// getTaskUnlocked is like GetTask but assumes the lock is already held.
func (t *TodolistSource) getTaskUnlocked(ctx context.Context, taskID string) (*Task, error) {
	file, err := os.Open(t.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open todolist: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	uncompletedRegex := regexp.MustCompile(`^\[\ \]\s*(.+)$`)
	completedRegex := regexp.MustCompile(`^\[x\]\s*(.+)$`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var status TaskStatus
		var description string

		if matches := uncompletedRegex.FindStringSubmatch(line); len(matches) > 1 {
			status = TaskStatusOpen
			description = strings.TrimSpace(matches[1])
		} else if matches := completedRegex.FindStringSubmatch(line); len(matches) > 1 {
			status = TaskStatusCompleted
			description = strings.TrimSpace(matches[1])
		} else {
			continue
		}

		currentID := fmt.Sprintf("todo-%s-%d", filepath.Base(t.filePath), lineNum)
		if currentID == taskID {
			return &Task{
				ID:          taskID,
				Title:       description,
				Description: description,
				Status:      status,
				Priority:    PriorityMedium,
				Source:      SourceTypeTodolist,
				SourceID:    fmt.Sprintf("%d", lineNum),
				SourceMeta: Metadata{
					"line":     lineNum,
					"filePath": t.filePath,
					"rawLine":  line,
				},
			}, nil
		}
	}

	return nil, ErrTaskNotFound
}
