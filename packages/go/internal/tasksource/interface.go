package tasksource

import "context"

// TaskSource is the interface that all task sources must implement.
// This allows the orchestrator to work with tasks from any source
// (todolist files, beads, Linear, GitHub, etc.) in a uniform way.
type TaskSource interface {
	// Info returns metadata about this task source.
	Info() SourceInfo

	// ListTasks returns tasks matching the filter.
	// If filter is nil, returns all tasks.
	ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error)

	// GetTask retrieves a specific task by ID.
	GetTask(ctx context.Context, taskID string) (*Task, error)

	// UpdateTask updates a task with the given changes.
	UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error

	// MarkComplete marks a task as completed.
	// This is a convenience method that updates status to completed
	// and may perform source-specific completion actions.
	MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error)

	// MarkBlocked marks a task as blocked with an optional reason.
	MarkBlocked(ctx context.Context, taskID string, reason string) error

	// Close cleans up any resources held by this source.
	Close() error
}

// MultiSource allows combining multiple task sources into one.
type MultiSource struct {
	sources []TaskSource
}

// NewMultiSource creates a new multi-source aggregator.
func NewMultiSource(sources ...TaskSource) *MultiSource {
	return &MultiSource{
		sources: sources,
	}
}

// Info returns combined metadata about all sources.
func (m *MultiSource) Info() SourceInfo {
	return SourceInfo{
		Type:        "multi",
		Name:        "Multi-Source Aggregator",
		Description: "Combines multiple task sources",
	}
}

// ListTasks returns tasks from all sources.
func (m *MultiSource) ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error) {
	var allTasks []Task

	for _, source := range m.sources {
		tasks, err := source.ListTasks(ctx, filter)
		if err != nil {
			// Log error but continue with other sources
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	// Apply limit if specified
	if filter != nil && filter.Limit > 0 && len(allTasks) > filter.Limit {
		allTasks = allTasks[:filter.Limit]
	}

	return allTasks, nil
}

// GetTask tries to find the task in any source.
func (m *MultiSource) GetTask(ctx context.Context, taskID string) (*Task, error) {
	for _, source := range m.sources {
		task, err := source.GetTask(ctx, taskID)
		if err == nil && task != nil {
			return task, nil
		}
	}
	return nil, ErrTaskNotFound
}

// UpdateTask updates the task in its source.
func (m *MultiSource) UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error {
	task, err := m.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	// Find the source and update
	for _, source := range m.sources {
		if source.Info().Type == task.Source {
			return source.UpdateTask(ctx, taskID, update)
		}
	}

	return ErrSourceNotFound
}

// MarkComplete marks the task complete in its source.
func (m *MultiSource) MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error) {
	task, err := m.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Find the source and mark complete
	for _, source := range m.sources {
		if source.Info().Type == task.Source {
			return source.MarkComplete(ctx, taskID)
		}
	}

	return nil, ErrSourceNotFound
}

// MarkBlocked marks the task as blocked in its source.
func (m *MultiSource) MarkBlocked(ctx context.Context, taskID string, reason string) error {
	task, err := m.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	// Find the source and mark blocked
	for _, source := range m.sources {
		if source.Info().Type == task.Source {
			return source.MarkBlocked(ctx, taskID, reason)
		}
	}

	return ErrSourceNotFound
}

// Close closes all sources.
func (m *MultiSource) Close() error {
	for _, source := range m.sources {
		if err := source.Close(); err != nil {
			// Log but continue closing others
			continue
		}
	}
	return nil
}

// AddSource adds a new task source to the multi-source.
func (m *MultiSource) AddSource(source TaskSource) {
	m.sources = append(m.sources, source)
}

// Sources returns all registered sources.
func (m *MultiSource) Sources() []TaskSource {
	return m.sources
}
