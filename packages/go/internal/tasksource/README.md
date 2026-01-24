# Task Source Package

The `tasksource` package provides a unified interface for working with tasks from multiple sources (todolist files, beads, Linear, GitHub, etc.) in the coders orchestrator.

## Architecture

### Core Interface

The `TaskSource` interface defines the contract that all task sources must implement:

```go
type TaskSource interface {
    Info() SourceInfo
    ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error)
    GetTask(ctx context.Context, taskID string) (*Task, error)
    UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error
    MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error)
    MarkBlocked(ctx context.Context, taskID string, reason string) error
    Close() error
}
```

### Normalized Task Model

Tasks from all sources are normalized into a common `Task` struct:

```go
type Task struct {
    ID          string       // Unique ID
    Title       string       // Task title
    Description string       // Full description
    Status      TaskStatus   // open, in_progress, completed, blocked, cancelled
    Priority    TaskPriority // 0-4 (P0-P4)
    Source      SourceType   // Which system this came from
    SourceID    string       // Original ID in source system
    SourceMeta  Metadata     // Source-specific metadata
    CreatedAt   *time.Time
    UpdatedAt   *time.Time
    Assignee    string
    Labels      []string
    BlockedBy   []string
    Blocks      []string
}
```

## Supported Sources

### 1. Todolist Files (`TodolistSource`)

Markdown/text files with checkbox format:

```markdown
[ ] Task to do
[x] Completed task
```

**Usage:**
```bash
coders loop --source "todolist:path=tasks.txt" --cwd .
```

**Features:**
- ✅ List tasks
- ✅ Mark complete (converts `[ ]` to `[x]`)
- ❌ Update (limited)
- ❌ Mark blocked

### 2. Beads Issues (`BeadsSource`)

Git-backed issue tracker via `bd` CLI:

**Usage:**
```bash
coders loop --source "beads:cwd=." --cwd .
```

**Features:**
- ✅ List tasks with filters
- ✅ Get task details
- ✅ Update task (status, priority, assignee)
- ✅ Mark complete (closes issue)
- ✅ Mark blocked
- ✅ Dependencies (blockedBy, blocks)

### 3. Linear (`LinearSource`)

Linear issue tracking via GraphQL API:

**Usage:**
```bash
# Requires LINEAR_API_KEY environment variable
export LINEAR_API_KEY=lin_api_xxx
coders loop --source "linear:team=TEAM123" --cwd .
```

**Features:**
- ✅ List tasks
- ✅ Get task details
- ✅ Update task
- ✅ Mark complete
- ✅ Mark blocked
- ⚠️ Requires API key

### 4. GitHub Issues (`GitHubSource`)

GitHub issues via REST API:

**Usage:**
```bash
# Requires GITHUB_TOKEN environment variable
export GITHUB_TOKEN=ghp_xxx
coders loop --source "github:owner=user,repo=myrepo" --cwd .
```

**Features:**
- ✅ List tasks
- ✅ Get task details
- ✅ Update task
- ✅ Mark complete (closes issue)
- ✅ Mark blocked (adds "blocked" label)
- ⚠️ Requires API token

## Multi-Source Usage

Combine multiple sources in a single loop:

```bash
# Process tasks from both todolist and beads
coders loop \
  --source "todolist:path=urgent.txt" \
  --source "beads:cwd=." \
  --cwd ~/project

# Only ready tasks (no blockers) from beads
coders loop \
  --source "beads:cwd=." \
  --only-ready \
  --cwd ~/project

# Mix all sources
coders loop \
  --source "todolist:path=tasks.txt" \
  --source "beads:cwd=." \
  --source "linear:team=TEAM123" \
  --source "github:owner=user,repo=myrepo" \
  --cwd ~/project
```

## Source Specification Format

Format: `type:param1=value1,param2=value2`

Examples:
- `todolist:path=/path/to/tasks.txt`
- `beads:cwd=.`
- `beads:cwd=/path/to/project`
- `linear:team=TEAM123`
- `linear:team=TEAM123,apiKey=lin_api_xxx`
- `github:owner=user,repo=myrepo`
- `github:owner=user,repo=myrepo,token=ghp_xxx`

## Task Filters

When listing tasks, you can filter by:

```go
filter := &TaskFilter{
    Status:    []TaskStatus{TaskStatusOpen, TaskStatusInProgress},
    Priority:  []TaskPriority{PriorityCritical, PriorityHigh},
    Assignee:  "username",
    Labels:    []string{"bug", "urgent"},
    Limit:     50,
    OnlyReady: true, // Only tasks with no blockers
}
```

## Provenance Tracking

Each task maintains its source information:

- `task.Source` - Source type (todolist, beads, linear, github)
- `task.SourceID` - Original ID in source system
- `task.SourceMeta` - Source-specific metadata

When a task is completed, the orchestrator automatically updates the original source.

## Adding New Sources

To add a new task source:

1. Create a new file (e.g., `newsource.go`)
2. Implement the `TaskSource` interface
3. Add to `factory.go`:

```go
case SourceTypeNewSource:
    return NewNewSource(config)
```

4. Update `SourceType` constants in `types.go`

Example implementation:

```go
type NewSource struct {
    config SourceConfig
    info   SourceInfo
}

func NewNewSource(config SourceConfig) (*NewSource, error) {
    return &NewSource{
        config: config,
        info: SourceInfo{
            Type:        "newsource",
            Name:        "New Source",
            Description: "Description of new source",
        },
    }, nil
}

// Implement all TaskSource interface methods...
```

## Error Handling

Common errors:

- `ErrTaskNotFound` - Task doesn't exist
- `ErrSourceNotFound` - Source doesn't exist
- `ErrInvalidConfig` - Invalid configuration
- `ErrReadOnly` - Attempted write on read-only source
- `ErrNotSupported` - Operation not supported by source

## Testing

Run tests:

```bash
go test ./internal/tasksource/...
```

## Future Enhancements

- [ ] Jira integration
- [ ] Asana integration
- [ ] Trello integration
- [ ] Notion integration
- [ ] Concurrent task execution
- [ ] Task caching/indexing
- [ ] Smart task prioritization
- [ ] Cross-source dependencies
