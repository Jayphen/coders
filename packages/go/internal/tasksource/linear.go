package tasksource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const linearAPIURL = "https://api.linear.app/graphql"

// LinearSource implements TaskSource for Linear issues.
type LinearSource struct {
	apiKey  string
	teamID  string
	client  *http.Client
	info    SourceInfo
}

// LinearConfig holds configuration for Linear source.
type LinearConfig struct {
	APIKey string // Linear API key (or read from LINEAR_API_KEY env var)
	TeamID string // Linear team ID (optional, filters by team)
}

// NewLinearSource creates a new Linear source.
func NewLinearSource(config LinearConfig) (*LinearSource, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("LINEAR_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("%w: LINEAR_API_KEY not set", ErrInvalidConfig)
	}

	return &LinearSource{
		apiKey: apiKey,
		teamID: config.TeamID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		info: SourceInfo{
			Type:        SourceTypeLinear,
			Name:        "Linear",
			Description: "Linear issue tracking",
			Config: Metadata{
				"teamId": config.TeamID,
			},
		},
	}, nil
}

// Info returns metadata about this source.
func (l *LinearSource) Info() SourceInfo {
	return l.info
}

// ListTasks returns tasks from Linear.
func (l *LinearSource) ListTasks(ctx context.Context, filter *TaskFilter) ([]Task, error) {
	// Build GraphQL query
	query := `
		query($teamId: String, $first: Int) {
			issues(
				filter: { team: { id: { eq: $teamId } } }
				first: $first
			) {
				nodes {
					id
					identifier
					title
					description
					priority
					createdAt
					updatedAt
					state {
						name
						type
					}
					assignee {
						name
						email
					}
					labels {
						nodes {
							name
						}
					}
				}
			}
		}
	`

	variables := map[string]interface{}{}
	if l.teamID != "" {
		variables["teamId"] = l.teamID
	}
	if filter != nil && filter.Limit > 0 {
		variables["first"] = filter.Limit
	} else {
		variables["first"] = 50 // Default limit
	}

	// Execute query
	resp, err := l.executeQuery(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result struct {
		Data struct {
			Issues struct {
				Nodes []struct {
					ID          string `json:"id"`
					Identifier  string `json:"identifier"`
					Title       string `json:"title"`
					Description string `json:"description"`
					Priority    int    `json:"priority"`
					CreatedAt   string `json:"createdAt"`
					UpdatedAt   string `json:"updatedAt"`
					State       struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"state"`
					Assignee *struct {
						Name  string `json:"name"`
						Email string `json:"email"`
					} `json:"assignee"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Linear response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("Linear API error: %s", result.Errors[0].Message)
	}

	// Convert to tasks
	var tasks []Task
	for _, issue := range result.Data.Issues.Nodes {
		task := l.convertLinearIssue(issue)

		// Apply filters
		if filter != nil {
			if len(filter.Status) > 0 {
				found := false
				for _, s := range filter.Status {
					if s == task.Status {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if filter.Assignee != "" && task.Assignee != filter.Assignee {
				continue
			}

			if len(filter.Labels) > 0 {
				hasAllLabels := true
				for _, requiredLabel := range filter.Labels {
					found := false
					for _, taskLabel := range task.Labels {
						if taskLabel == requiredLabel {
							found = true
							break
						}
					}
					if !found {
						hasAllLabels = false
						break
					}
				}
				if !hasAllLabels {
					continue
				}
			}
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTask retrieves a specific Linear issue.
func (l *LinearSource) GetTask(ctx context.Context, taskID string) (*Task, error) {
	query := `
		query($id: String!) {
			issue(id: $id) {
				id
				identifier
				title
				description
				priority
				createdAt
				updatedAt
				state {
					name
					type
				}
				assignee {
					name
					email
				}
				labels {
					nodes {
						name
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"id": taskID,
	}

	resp, err := l.executeQuery(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Issue struct {
				ID          string `json:"id"`
				Identifier  string `json:"identifier"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int    `json:"priority"`
				CreatedAt   string `json:"createdAt"`
				UpdatedAt   string `json:"updatedAt"`
				State       struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"state"`
				Assignee *struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"assignee"`
				Labels struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"issue"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Linear response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("Linear API error: %s", result.Errors[0].Message)
	}

	task := l.convertLinearIssue(result.Data.Issue)
	return &task, nil
}

// UpdateTask updates a Linear issue.
func (l *LinearSource) UpdateTask(ctx context.Context, taskID string, update TaskUpdate) error {
	// Linear uses mutations for updates
	mutation := `
		mutation($id: String!, $input: IssueUpdateInput!) {
			issueUpdate(id: $id, input: $input) {
				success
				issue {
					id
				}
			}
		}
	`

	input := map[string]interface{}{}

	if update.Status != nil {
		// Map status to Linear state names (this is simplified)
		stateMap := map[TaskStatus]string{
			TaskStatusOpen:       "Todo",
			TaskStatusInProgress: "In Progress",
			TaskStatusCompleted:  "Done",
			TaskStatusBlocked:    "Blocked",
			TaskStatusCancelled:  "Cancelled",
		}
		if stateName, ok := stateMap[*update.Status]; ok {
			input["stateId"] = stateName // Note: Linear requires state ID, not name
		}
	}

	if update.Priority != nil {
		input["priority"] = int(*update.Priority)
	}

	if update.Description != nil {
		input["description"] = *update.Description
	}

	variables := map[string]interface{}{
		"id":    taskID,
		"input": input,
	}

	_, err := l.executeQuery(ctx, mutation, variables)
	return err
}

// MarkComplete marks a Linear issue as completed.
func (l *LinearSource) MarkComplete(ctx context.Context, taskID string) (*CompletionResult, error) {
	status := TaskStatusCompleted
	err := l.UpdateTask(ctx, taskID, TaskUpdate{
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
		Message: fmt.Sprintf("Marked Linear issue complete: %s", taskID),
	}, nil
}

// MarkBlocked marks a Linear issue as blocked.
func (l *LinearSource) MarkBlocked(ctx context.Context, taskID string, reason string) error {
	status := TaskStatusBlocked
	update := TaskUpdate{
		Status: &status,
	}

	if reason != "" {
		update.Description = &reason
	}

	return l.UpdateTask(ctx, taskID, update)
}

// Close cleans up resources.
func (l *LinearSource) Close() error {
	return nil
}

// executeQuery executes a GraphQL query against Linear API.
func (l *LinearSource) executeQuery(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	reqBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", linearAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", l.apiKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Linear API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// convertLinearIssue converts a Linear issue to a normalized Task.
func (l *LinearSource) convertLinearIssue(issue interface{}) Task {
	// Type assertion to extract fields (simplified for interface{})
	issueMap, ok := issue.(map[string]interface{})
	if !ok {
		// If it's a struct, we need to handle it differently
		// For now, return empty task
		return Task{}
	}

	id := getString(issueMap, "id")
	identifier := getString(issueMap, "identifier")
	title := getString(issueMap, "title")
	description := getString(issueMap, "description")
	priority := getInt(issueMap, "priority")

	// Parse state
	status := TaskStatusOpen
	if state, ok := issueMap["state"].(map[string]interface{}); ok {
		stateType := getString(state, "type")
		switch strings.ToLower(stateType) {
		case "backlog", "unstarted":
			status = TaskStatusOpen
		case "started":
			status = TaskStatusInProgress
		case "completed":
			status = TaskStatusCompleted
		case "canceled":
			status = TaskStatusCancelled
		}
	}

	// Parse timestamps
	var createdAt, updatedAt *time.Time
	if createdAtStr := getString(issueMap, "createdAt"); createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			createdAt = &t
		}
	}
	if updatedAtStr := getString(issueMap, "updatedAt"); updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			updatedAt = &t
		}
	}

	// Parse assignee
	assignee := ""
	if assigneeMap, ok := issueMap["assignee"].(map[string]interface{}); ok {
		assignee = getString(assigneeMap, "name")
	}

	// Parse labels
	var labels []string
	if labelsMap, ok := issueMap["labels"].(map[string]interface{}); ok {
		if nodes, ok := labelsMap["nodes"].([]interface{}); ok {
			for _, node := range nodes {
				if labelMap, ok := node.(map[string]interface{}); ok {
					labels = append(labels, getString(labelMap, "name"))
				}
			}
		}
	}

	// Map Linear priority (0-4) to TaskPriority
	taskPriority := TaskPriority(priority)
	if taskPriority < PriorityCritical {
		taskPriority = PriorityCritical
	}
	if taskPriority > PriorityBacklog {
		taskPriority = PriorityBacklog
	}

	return Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    taskPriority,
		Source:      SourceTypeLinear,
		SourceID:    id,
		SourceMeta: Metadata{
			"identifier": identifier,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Assignee:  assignee,
		Labels:    labels,
	}
}

// Helper functions for extracting values from interface{} maps
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}
