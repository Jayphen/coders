package tasksource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const githubAPIURL = "https://api.github.com"

// GitHubSource implements TaskSource for GitHub issues.
type GitHubSource struct {
	token  string
	owner  string
	repo   string
	client *http.Client
	info   SourceInfo
}

// GitHubConfig holds configuration for GitHub source.
type GitHubConfig struct {
	Token string // GitHub token (or read from GITHUB_TOKEN env var)
	Owner string // Repository owner
	Repo  string // Repository name
}

// GitHubIssue represents a GitHub issue from the API.
type GitHubIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Assignee  *struct {
		Login string `json:"login"`
	} `json:"assignee"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HTMLURL string `json:"html_url"`
}

// NewGitHubSource creates a new GitHub source.
func NewGitHubSource(config GitHubConfig) (*GitHubSource, error) {
	token := config.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token == "" {
		return nil, fmt.Errorf("%w: GITHUB_TOKEN not set", ErrInvalidConfig)
	}

	if config.Owner == "" || config.Repo == "" {
		return nil, fmt.Errorf("%w: owner and repo are required", ErrInvalidConfig)
	}

	return &GitHubSource{
		token: token,
		owner: config.Owner,
		repo:  config.Repo,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		info: SourceInfo{
			Type:        SourceTypeGitHub,
			Name:        fmt.Sprintf("GitHub: %s/%s", config.Owner, config.Repo),
			Description: fmt.Sprintf("GitHub issues for %s/%s", config.Owner, config.Repo),
			Config: Metadata{
				"owner": config.Owner,
				"repo":  config.Repo,
			},
		},
	}, nil
}

// Info returns metadata about this source.
func (g *GitHubSource) Info() SourceInfo {
	return g.info
}

// ListTasks returns tasks from GitHub issues.
func (g *GitHubSource) ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues", githubAPIURL, g.owner, g.repo)

	// Build query parameters
	params := "?per_page=100"

	// Apply status filter
	if filter != nil && len(filter.Status) > 0 {
		// GitHub only supports "open" or "closed"
		hasOpen := false
		hasClosed := false
		for _, status := range filter.Status {
			if status == TaskStatusOpen || status == TaskStatusInProgress || status == TaskStatusBlocked {
				hasOpen = true
			}
			if status == TaskStatusCompleted || status == TaskStatusCancelled {
				hasClosed = true
			}
		}

		if hasOpen && !hasClosed {
			params += "&state=open"
		} else if !hasOpen && hasClosed {
			params += "&state=closed"
		} else {
			params += "&state=all"
		}
	} else {
		params += "&state=open" // Default to open issues
	}

	// Apply assignee filter
	if filter != nil && filter.Assignee != "" {
		params += "&assignee=" + filter.Assignee
	}

	// Apply labels filter
	if filter != nil && len(filter.Labels) > 0 {
		params += "&labels=" + strings.Join(filter.Labels, ",")
	}

	url += params

	// Execute request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var issues []GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	// Convert to tasks
	var tasks []Task
	for _, issue := range issues {
		task := g.convertGitHubIssue(issue)

		// Apply additional filters
		if filter != nil {
			if filter.OnlyReady {
				// Check if issue has "blocked" label
				isBlocked := false
				for _, label := range task.Labels {
					if strings.ToLower(label) == "blocked" {
						isBlocked = true
						break
					}
				}
				if isBlocked {
					continue
				}
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

// GetTask retrieves a specific GitHub issue.
func (g *GitHubSource) GetTask(ctx context.Context, taskID string) (*Task, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s", githubAPIURL, g.owner, g.repo, taskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issue GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	task := g.convertGitHubIssue(issue)
	return &task, nil
}

// UpdateTask updates a GitHub issue.
func (g *GitHubSource) UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s", githubAPIURL, g.owner, g.repo, taskID)

	// Build update payload
	payload := map[string]interface{}{}

	if update.Status != nil {
		// Map status to GitHub state
		switch *update.Status {
		case TaskStatusCompleted, TaskStatusCancelled:
			payload["state"] = "closed"
		default:
			payload["state"] = "open"
		}
	}

	if update.Description != nil {
		payload["body"] = *update.Description
	}

	if update.Assignee != nil {
		payload["assignee"] = *update.Assignee
	}

	// Handle labels
	if len(update.AddLabels) > 0 || len(update.RemoveLabels) > 0 {
		// Get current labels
		task, err := g.GetTask(ctx, taskID)
		if err != nil {
			return err
		}

		labels := task.Labels

		// Add new labels
		for _, label := range update.AddLabels {
			found := false
			for _, existing := range labels {
				if existing == label {
					found = true
					break
				}
			}
			if !found {
				labels = append(labels, label)
			}
		}

		// Remove labels
		var filteredLabels []string
		for _, label := range labels {
			remove := false
			for _, toRemove := range update.RemoveLabels {
				if label == toRemove {
					remove = true
					break
				}
			}
			if !remove {
				filteredLabels = append(filteredLabels, label)
			}
		}

		payload["labels"] = filteredLabels
	}

	// Marshal payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Execute request
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// MarkComplete marks a GitHub issue as completed.
func (g *GitHubSource) MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error) {
	status := TaskStatusCompleted
	err := g.UpdateTask(ctx, taskID, TaskUpdate{
		Status: &status,
	})

	if err != nil {
		return &CompletionResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &CompletionResult{
		Success: true,
		Message: fmt.Sprintf("Closed GitHub issue #%s", taskID),
	}, nil
}

// MarkBlocked marks a GitHub issue as blocked.
func (g *GitHubSource) MarkBlocked(ctx context.Context, taskID string, reason string) error {
	update := TaskUpdate{
		AddLabels: []string{"blocked"},
	}

	if reason != "" {
		update.Description = &reason
	}

	return g.UpdateTask(ctx, taskID, update)
}

// Close cleans up resources.
func (g *GitHubSource) Close() error {
	return nil
}

// convertGitHubIssue converts a GitHub issue to a normalized Task.
func (g *GitHubSource) convertGitHubIssue(issue GitHubIssue) Task {
	// Convert status
	status := TaskStatusOpen
	switch strings.ToLower(issue.State) {
	case "open":
		// Check labels for more specific status
		for _, label := range issue.Labels {
			switch strings.ToLower(label.Name) {
			case "in progress", "in-progress":
				status = TaskStatusInProgress
			case "blocked":
				status = TaskStatusBlocked
			}
		}
	case "closed":
		status = TaskStatusCompleted
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

	// Extract labels
	var labels []string
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}

	// Determine priority from labels (GitHub doesn't have native priority)
	priority := PriorityMedium
	for _, label := range issue.Labels {
		switch strings.ToLower(label.Name) {
		case "priority: critical", "p0":
			priority = PriorityCritical
		case "priority: high", "p1":
			priority = PriorityHigh
		case "priority: medium", "p2":
			priority = PriorityMedium
		case "priority: low", "p3":
			priority = PriorityLow
		case "priority: backlog", "p4":
			priority = PriorityBacklog
		}
	}

	// Get assignee
	assignee := ""
	if issue.Assignee != nil {
		assignee = issue.Assignee.Login
	}

	taskID := fmt.Sprintf("%d", issue.Number)

	return Task{
		ID:          taskID,
		Title:       issue.Title,
		Description: issue.Body,
		Status:      status,
		Priority:    priority,
		Source:      SourceTypeGitHub,
		SourceID:    taskID,
		SourceMeta: Metadata{
			"number":  issue.Number,
			"htmlUrl": issue.HTMLURL,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Assignee:  assignee,
		Labels:    labels,
	}
}
