package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Jayphen/coders/internal/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis creates a test Redis client with miniredis
func setupTestRedis(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	client := &Client{rdb: rdb}
	return client, mr
}

func TestSetAndGetPromise(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		promise *types.CoderPromise
	}{
		{
			name: "basic promise",
			promise: &types.CoderPromise{
				SessionID: "test-session-1",
				Timestamp: time.Now().UnixMilli(),
				Summary:   "Completed task",
				Status:    types.PromiseCompleted,
			},
		},
		{
			name: "promise with files and blockers",
			promise: &types.CoderPromise{
				SessionID:    "test-session-2",
				Timestamp:    time.Now().UnixMilli(),
				Summary:      "Blocked on dependency",
				Status:       types.PromiseBlocked,
				FilesChanged: []string{"file1.go", "file2.go"},
				Blockers:     []string{"task-123", "task-456"},
			},
		},
		{
			name: "promise needs review",
			promise: &types.CoderPromise{
				SessionID: "test-session-3",
				Timestamp: time.Now().UnixMilli(),
				Summary:   "Ready for review",
				Status:    types.PromiseNeedsReview,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set promise
			err := client.SetPromise(ctx, tt.promise)
			if err != nil {
				t.Fatalf("SetPromise failed: %v", err)
			}

			// Get promise
			retrieved, err := client.GetPromise(tt.promise.SessionID)
			if err != nil {
				t.Fatalf("GetPromise failed: %v", err)
			}

			if retrieved == nil {
				t.Fatal("GetPromise returned nil")
			}

			// Verify fields
			if retrieved.SessionID != tt.promise.SessionID {
				t.Errorf("SessionID mismatch: got %v, want %v", retrieved.SessionID, tt.promise.SessionID)
			}
			if retrieved.Summary != tt.promise.Summary {
				t.Errorf("Summary mismatch: got %v, want %v", retrieved.Summary, tt.promise.Summary)
			}
			if retrieved.Status != tt.promise.Status {
				t.Errorf("Status mismatch: got %v, want %v", retrieved.Status, tt.promise.Status)
			}
		})
	}
}

func TestGetPromise_NotFound(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	promise, err := client.GetPromise("nonexistent")
	if err != nil {
		t.Fatalf("GetPromise returned error: %v", err)
	}
	if promise != nil {
		t.Errorf("Expected nil promise for nonexistent session, got %v", promise)
	}
}

func TestDeletePromise(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	promise := &types.CoderPromise{
		SessionID: "test-session",
		Timestamp: time.Now().UnixMilli(),
		Summary:   "Test",
		Status:    types.PromiseCompleted,
	}

	// Set promise
	err := client.SetPromise(ctx, promise)
	if err != nil {
		t.Fatalf("SetPromise failed: %v", err)
	}

	// Verify it exists
	retrieved, err := client.GetPromise(promise.SessionID)
	if err != nil || retrieved == nil {
		t.Fatal("Promise should exist before deletion")
	}

	// Delete promise
	err = client.DeletePromise(ctx, promise.SessionID)
	if err != nil {
		t.Fatalf("DeletePromise failed: %v", err)
	}

	// Verify it's gone
	retrieved, err = client.GetPromise(promise.SessionID)
	if err != nil {
		t.Fatalf("GetPromise after delete failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Promise should be nil after deletion")
	}
}

func TestGetPromises(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Empty case
	promises, err := client.GetPromises(ctx)
	if err != nil {
		t.Fatalf("GetPromises failed: %v", err)
	}
	if len(promises) != 0 {
		t.Errorf("Expected 0 promises, got %d", len(promises))
	}

	// Add multiple promises
	testPromises := []*types.CoderPromise{
		{
			SessionID: "session-1",
			Timestamp: time.Now().UnixMilli(),
			Summary:   "Task 1",
			Status:    types.PromiseCompleted,
		},
		{
			SessionID: "session-2",
			Timestamp: time.Now().UnixMilli(),
			Summary:   "Task 2",
			Status:    types.PromiseBlocked,
		},
		{
			SessionID: "session-3",
			Timestamp: time.Now().UnixMilli(),
			Summary:   "Task 3",
			Status:    types.PromiseNeedsReview,
		},
	}

	for _, p := range testPromises {
		err := client.SetPromise(ctx, p)
		if err != nil {
			t.Fatalf("SetPromise failed: %v", err)
		}
	}

	// Get all promises
	promises, err = client.GetPromises(ctx)
	if err != nil {
		t.Fatalf("GetPromises failed: %v", err)
	}

	if len(promises) != len(testPromises) {
		t.Errorf("Expected %d promises, got %d", len(testPromises), len(promises))
	}

	// Verify all promises are present
	for _, expected := range testPromises {
		found, ok := promises[expected.SessionID]
		if !ok {
			t.Errorf("Promise for session %s not found", expected.SessionID)
			continue
		}
		if found.Summary != expected.Summary {
			t.Errorf("Summary mismatch for %s: got %v, want %v", expected.SessionID, found.Summary, expected.Summary)
		}
	}
}

func TestSetAndGetHeartbeat(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name      string
		heartbeat *types.HeartbeatData
	}{
		{
			name: "basic heartbeat",
			heartbeat: &types.HeartbeatData{
				PaneID:    "pane-1",
				SessionID: "session-1",
				Timestamp: time.Now().UnixMilli(),
				Status:    "running",
			},
		},
		{
			name: "heartbeat with usage",
			heartbeat: &types.HeartbeatData{
				PaneID:       "pane-2",
				SessionID:    "session-2",
				Timestamp:    time.Now().UnixMilli(),
				Status:       "active",
				LastActivity: "Working on tests",
				Task:         "Write unit tests",
				Usage: &types.UsageStats{
					Cost:     "$1.25",
					Tokens:   50000,
					APICalls: 10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetHeartbeat(ctx, tt.heartbeat)
			if err != nil {
				t.Fatalf("SetHeartbeat failed: %v", err)
			}

			// Verify it was stored (get raw data)
			key := PaneKeyPrefix + tt.heartbeat.SessionID
			data, err := client.rdb.Get(ctx, key).Result()
			if err != nil {
				t.Fatalf("Failed to get heartbeat: %v", err)
			}

			var retrieved types.HeartbeatData
			err = json.Unmarshal([]byte(data), &retrieved)
			if err != nil {
				t.Fatalf("Failed to unmarshal heartbeat: %v", err)
			}

			if retrieved.SessionID != tt.heartbeat.SessionID {
				t.Errorf("SessionID mismatch: got %v, want %v", retrieved.SessionID, tt.heartbeat.SessionID)
			}
			if retrieved.Status != tt.heartbeat.Status {
				t.Errorf("Status mismatch: got %v, want %v", retrieved.Status, tt.heartbeat.Status)
			}
		})
	}
}

func TestGetHeartbeats(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Empty case
	heartbeats, err := client.GetHeartbeats(ctx)
	if err != nil {
		t.Fatalf("GetHeartbeats failed: %v", err)
	}
	if len(heartbeats) != 0 {
		t.Errorf("Expected 0 heartbeats, got %d", len(heartbeats))
	}

	// Add multiple heartbeats
	testHeartbeats := []*types.HeartbeatData{
		{
			PaneID:    "pane-1",
			SessionID: "session-1",
			Timestamp: time.Now().UnixMilli(),
			Status:    "running",
		},
		{
			PaneID:    "pane-2",
			SessionID: "session-2",
			Timestamp: time.Now().UnixMilli(),
			Status:    "active",
		},
	}

	for _, hb := range testHeartbeats {
		err := client.SetHeartbeat(ctx, hb)
		if err != nil {
			t.Fatalf("SetHeartbeat failed: %v", err)
		}
	}

	// Get all heartbeats
	heartbeats, err = client.GetHeartbeats(ctx)
	if err != nil {
		t.Fatalf("GetHeartbeats failed: %v", err)
	}

	if len(heartbeats) != len(testHeartbeats) {
		t.Errorf("Expected %d heartbeats, got %d", len(testHeartbeats), len(heartbeats))
	}

	// Verify all heartbeats are present
	for _, expected := range testHeartbeats {
		found, ok := heartbeats[expected.SessionID]
		if !ok {
			t.Errorf("Heartbeat for session %s not found", expected.SessionID)
			continue
		}
		if found.Status != expected.Status {
			t.Errorf("Status mismatch for %s: got %v, want %v", expected.SessionID, found.Status, expected.Status)
		}
	}
}

func TestDetermineHeartbeatStatus(t *testing.T) {
	tests := []struct {
		name     string
		hb       *types.HeartbeatData
		expected types.HeartbeatStatus
	}{
		{
			name:     "nil heartbeat",
			hb:       nil,
			expected: types.HeartbeatDead,
		},
		{
			name: "fresh heartbeat",
			hb: &types.HeartbeatData{
				Timestamp: time.Now().UnixMilli(),
			},
			expected: types.HeartbeatHealthy,
		},
		{
			name: "stale heartbeat",
			hb: &types.HeartbeatData{
				Timestamp: time.Now().Add(-2 * time.Minute).UnixMilli(),
			},
			expected: types.HeartbeatStale,
		},
		{
			name: "dead heartbeat",
			hb: &types.HeartbeatData{
				Timestamp: time.Now().Add(-10 * time.Minute).UnixMilli(),
			},
			expected: types.HeartbeatDead,
		},
		{
			name: "edge case - 59 seconds",
			hb: &types.HeartbeatData{
				Timestamp: time.Now().Add(-59 * time.Second).UnixMilli(),
			},
			expected: types.HeartbeatHealthy,
		},
		{
			name: "edge case - 4 minutes 59 seconds",
			hb: &types.HeartbeatData{
				Timestamp: time.Now().Add(-4*time.Minute - 59*time.Second).UnixMilli(),
			},
			expected: types.HeartbeatStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineHeartbeatStatus(tt.hb)
			if result != tt.expected {
				t.Errorf("DetermineHeartbeatStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHealthCheckOperations(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	healthCheck := &types.HealthCheckResult{
		SessionID:      "session-1",
		Timestamp:      time.Now().UnixMilli(),
		Status:         types.HealthHealthy,
		HeartbeatAge:   5000,
		OutputHash:     "abc123",
		ProcessRunning: true,
		TmuxAlive:      true,
		Message:        "All systems operational",
	}

	// Set health check
	err := client.SetHealthCheck(ctx, healthCheck)
	if err != nil {
		t.Fatalf("SetHealthCheck failed: %v", err)
	}

	// Get health checks
	checks, err := client.GetHealthChecks(ctx)
	if err != nil {
		t.Fatalf("GetHealthChecks failed: %v", err)
	}

	if len(checks) != 1 {
		t.Fatalf("Expected 1 health check, got %d", len(checks))
	}

	retrieved, ok := checks[healthCheck.SessionID]
	if !ok {
		t.Fatal("Health check not found")
	}

	if retrieved.Status != healthCheck.Status {
		t.Errorf("Status mismatch: got %v, want %v", retrieved.Status, healthCheck.Status)
	}
	if retrieved.Message != healthCheck.Message {
		t.Errorf("Message mismatch: got %v, want %v", retrieved.Message, healthCheck.Message)
	}

	// Delete health check
	err = client.DeleteHealthCheck(ctx, healthCheck.SessionID)
	if err != nil {
		t.Fatalf("DeleteHealthCheck failed: %v", err)
	}

	// Verify deletion
	checks, err = client.GetHealthChecks(ctx)
	if err != nil {
		t.Fatalf("GetHealthChecks failed: %v", err)
	}

	if len(checks) != 0 {
		t.Errorf("Expected 0 health checks after deletion, got %d", len(checks))
	}
}

func TestHealthSummaryOperations(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	summary := &types.HealthCheckSummary{
		Timestamp:     time.Now().UnixMilli(),
		TotalSessions: 5,
		Healthy:       3,
		Stale:         1,
		Dead:          1,
		Stuck:         0,
		Unresponsive:  0,
		Sessions: []types.HealthCheckResult{
			{
				SessionID: "session-1",
				Status:    types.HealthHealthy,
			},
		},
	}

	// Set summary
	err := client.SetHealthSummary(ctx, summary)
	if err != nil {
		t.Fatalf("SetHealthSummary failed: %v", err)
	}

	// Get summary
	retrieved, err := client.GetHealthSummary(ctx)
	if err != nil {
		t.Fatalf("GetHealthSummary failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetHealthSummary returned nil")
	}

	if retrieved.TotalSessions != summary.TotalSessions {
		t.Errorf("TotalSessions mismatch: got %v, want %v", retrieved.TotalSessions, summary.TotalSessions)
	}
	if retrieved.Healthy != summary.Healthy {
		t.Errorf("Healthy mismatch: got %v, want %v", retrieved.Healthy, summary.Healthy)
	}
	if retrieved.Stale != summary.Stale {
		t.Errorf("Stale mismatch: got %v, want %v", retrieved.Stale, summary.Stale)
	}
}

func TestGetHealthSummary_NotFound(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	summary, err := client.GetHealthSummary(ctx)
	if err != nil {
		t.Fatalf("GetHealthSummary returned error: %v", err)
	}
	if summary != nil {
		t.Errorf("Expected nil summary when none exists, got %v", summary)
	}
}

func TestSessionStateOperations(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	state := &types.SessionState{
		SessionID:        "session-1",
		SessionName:      "test-session",
		Tool:             "claude",
		Task:             "Write tests",
		Cwd:              "/home/user/project",
		Model:            "claude-3-opus",
		UseOllama:        false,
		HeartbeatEnabled: true,
		RestartOnCrash:   true,
		RestartCount:     0,
		MaxRestarts:      3,
		CreatedAt:        time.Now().UnixMilli(),
	}

	// Set session state
	err := client.SetSessionState(ctx, state)
	if err != nil {
		t.Fatalf("SetSessionState failed: %v", err)
	}

	// Get session state
	retrieved, err := client.GetSessionState(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("GetSessionState failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetSessionState returned nil")
	}

	if retrieved.SessionID != state.SessionID {
		t.Errorf("SessionID mismatch: got %v, want %v", retrieved.SessionID, state.SessionID)
	}
	if retrieved.Tool != state.Tool {
		t.Errorf("Tool mismatch: got %v, want %v", retrieved.Tool, state.Tool)
	}
	if retrieved.RestartOnCrash != state.RestartOnCrash {
		t.Errorf("RestartOnCrash mismatch: got %v, want %v", retrieved.RestartOnCrash, state.RestartOnCrash)
	}

	// Delete session state
	err = client.DeleteSessionState(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("DeleteSessionState failed: %v", err)
	}

	// Verify deletion
	retrieved, err = client.GetSessionState(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("GetSessionState after delete failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Session state should be nil after deletion")
	}
}

func TestGetSessionState_NotFound(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	state, err := client.GetSessionState(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSessionState returned error: %v", err)
	}
	if state != nil {
		t.Errorf("Expected nil state for nonexistent session, got %v", state)
	}
}

func TestCrashEventOperations(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	sessionID := "session-1"

	// Add multiple crash events
	events := []*types.CrashEvent{
		{
			SessionID:   sessionID,
			Timestamp:   time.Now().UnixMilli(),
			Reason:      "Out of memory",
			WillRestart: true,
		},
		{
			SessionID:   sessionID,
			Timestamp:   time.Now().Add(1 * time.Minute).UnixMilli(),
			Reason:      "Connection timeout",
			WillRestart: true,
		},
		{
			SessionID:   sessionID,
			Timestamp:   time.Now().Add(2 * time.Minute).UnixMilli(),
			Reason:      "Max restarts reached",
			WillRestart: false,
		},
	}

	for _, event := range events {
		err := client.RecordCrashEvent(ctx, event)
		if err != nil {
			t.Fatalf("RecordCrashEvent failed: %v", err)
		}
	}

	// Get crash events
	retrieved, err := client.GetCrashEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCrashEvents failed: %v", err)
	}

	if len(retrieved) != len(events) {
		t.Errorf("Expected %d crash events, got %d", len(events), len(retrieved))
	}

	// Verify events (should be in reverse order due to LPUSH)
	for i, event := range retrieved {
		expectedIdx := len(events) - 1 - i
		if event.Reason != events[expectedIdx].Reason {
			t.Errorf("Event %d reason mismatch: got %v, want %v", i, event.Reason, events[expectedIdx].Reason)
		}
	}
}

func TestCrashEvents_MaxLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	sessionID := "session-1"

	// Add 15 crash events (should keep only last 10)
	for i := 0; i < 15; i++ {
		event := &types.CrashEvent{
			SessionID:   sessionID,
			Timestamp:   time.Now().Add(time.Duration(i) * time.Minute).UnixMilli(),
			Reason:      "Test crash",
			WillRestart: true,
		}
		err := client.RecordCrashEvent(ctx, event)
		if err != nil {
			t.Fatalf("RecordCrashEvent failed: %v", err)
		}
	}

	// Get crash events
	retrieved, err := client.GetCrashEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCrashEvents failed: %v", err)
	}

	if len(retrieved) != 10 {
		t.Errorf("Expected 10 crash events (limit), got %d", len(retrieved))
	}
}

func TestGetCrashEvents_NoEvents(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	events, err := client.GetCrashEvents(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetCrashEvents failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events for nonexistent session, got %d", len(events))
	}
}

func TestSetJSON(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
		ttl   time.Duration
	}{
		{
			name: "simple struct",
			key:  "test:key1",
			value: map[string]string{
				"name":  "test",
				"value": "data",
			},
			ttl: 5 * time.Minute,
		},
		{
			name: "no TTL",
			key:  "test:key2",
			value: map[string]int{
				"count": 42,
			},
			ttl: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetJSON(tt.key, tt.value, tt.ttl)
			if err != nil {
				t.Fatalf("SetJSON failed: %v", err)
			}

			// Verify data was stored
			data, err := client.rdb.Get(ctx, tt.key).Result()
			if err != nil {
				t.Fatalf("Failed to get key: %v", err)
			}

			// Verify it's valid JSON
			var retrieved interface{}
			err = json.Unmarshal([]byte(data), &retrieved)
			if err != nil {
				t.Fatalf("Stored data is not valid JSON: %v", err)
			}
		})
	}
}

func TestGetRaw(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	key := "test:raw"
	value := "test value"

	// Set a raw value
	err := client.rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		t.Fatalf("Failed to set raw value: %v", err)
	}

	// Get raw value
	retrieved, err := client.GetRaw(ctx, key)
	if err != nil {
		t.Fatalf("GetRaw failed: %v", err)
	}

	if retrieved != value {
		t.Errorf("GetRaw() = %v, want %v", retrieved, value)
	}
}

func TestMGetRaw(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set multiple keys
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range testData {
		err := client.rdb.Set(ctx, k, v, 0).Err()
		if err != nil {
			t.Fatalf("Failed to set key %s: %v", k, err)
		}
	}

	// Get multiple keys
	keys := []string{"key1", "key2", "key3"}
	values, err := client.MGetRaw(ctx, keys)
	if err != nil {
		t.Fatalf("MGetRaw failed: %v", err)
	}

	if len(values) != len(keys) {
		t.Fatalf("Expected %d values, got %d", len(keys), len(values))
	}

	for i, key := range keys {
		if values[i] != testData[key] {
			t.Errorf("Value mismatch for %s: got %v, want %v", key, values[i], testData[key])
		}
	}
}

func TestMGetRaw_MissingKeys(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set only some keys
	client.rdb.Set(ctx, "key1", "value1", 0)

	keys := []string{"key1", "nonexistent", "key3"}
	values, err := client.MGetRaw(ctx, keys)
	if err != nil {
		t.Fatalf("MGetRaw failed: %v", err)
	}

	if len(values) != len(keys) {
		t.Fatalf("Expected %d values, got %d", len(keys), len(values))
	}

	if values[0] != "value1" {
		t.Errorf("Expected 'value1', got %v", values[0])
	}
	if values[1] != "" {
		t.Errorf("Expected empty string for missing key, got %v", values[1])
	}
}

func TestScanKeys(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set multiple keys with different prefixes
	keys := []string{
		"coders:promise:session1",
		"coders:promise:session2",
		"coders:pane:session1",
		"coders:health:session1",
		"other:key",
	}

	for _, key := range keys {
		err := client.rdb.Set(ctx, key, "value", 0).Err()
		if err != nil {
			t.Fatalf("Failed to set key %s: %v", key, err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "all promise keys",
			pattern:  "coders:promise:*",
			expected: 2,
		},
		{
			name:     "all coders keys",
			pattern:  "coders:*",
			expected: 4,
		},
		{
			name:     "specific session",
			pattern:  "*:session1",
			expected: 3,
		},
		{
			name:     "no matches",
			pattern:  "nonexistent:*",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := client.ScanKeys(ctx, tt.pattern)
			if err != nil {
				t.Fatalf("ScanKeys failed: %v", err)
			}

			if len(found) != tt.expected {
				t.Errorf("Expected %d keys, got %d", tt.expected, len(found))
			}
		})
	}
}

func TestClose(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	err := client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify connection is closed (operations should fail)
	ctx := context.Background()
	err = client.rdb.Ping(ctx).Err()
	if err == nil {
		t.Error("Expected error after closing connection, got nil")
	}
}

func TestHeartbeatExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	hb := &types.HeartbeatData{
		PaneID:    "pane-1",
		SessionID: "session-1",
		Timestamp: time.Now().UnixMilli(),
		Status:    "running",
	}

	// Set heartbeat
	err := client.SetHeartbeat(ctx, hb)
	if err != nil {
		t.Fatalf("SetHeartbeat failed: %v", err)
	}

	// Verify TTL is set (should be 10 minutes = 600 seconds)
	key := PaneKeyPrefix + hb.SessionID
	ttl := mr.TTL(key)

	// TTL should be close to 10 minutes
	expectedTTL := 10 * time.Minute
	if ttl < expectedTTL-time.Second || ttl > expectedTTL+time.Second {
		t.Errorf("TTL mismatch: got %v, want ~%v", ttl, expectedTTL)
	}
}

func TestHealthCheckExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	hc := &types.HealthCheckResult{
		SessionID: "session-1",
		Timestamp: time.Now().UnixMilli(),
		Status:    types.HealthHealthy,
	}

	// Set health check
	err := client.SetHealthCheck(ctx, hc)
	if err != nil {
		t.Fatalf("SetHealthCheck failed: %v", err)
	}

	// Verify TTL is set (should be 10 minutes)
	key := HealthKeyPrefix + hc.SessionID
	ttl := mr.TTL(key)

	expectedTTL := 10 * time.Minute
	if ttl < expectedTTL-time.Second || ttl > expectedTTL+time.Second {
		t.Errorf("TTL mismatch: got %v, want ~%v", ttl, expectedTTL)
	}
}

func TestHealthSummaryExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	summary := &types.HealthCheckSummary{
		Timestamp:     time.Now().UnixMilli(),
		TotalSessions: 1,
	}

	// Set summary
	err := client.SetHealthSummary(ctx, summary)
	if err != nil {
		t.Fatalf("SetHealthSummary failed: %v", err)
	}

	// Verify TTL is set (should be 5 minutes)
	ttl := mr.TTL(HealthSummaryKey)

	expectedTTL := 5 * time.Minute
	if ttl < expectedTTL-time.Second || ttl > expectedTTL+time.Second {
		t.Errorf("TTL mismatch: got %v, want ~%v", ttl, expectedTTL)
	}
}

func TestSessionStateExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	state := &types.SessionState{
		SessionID:   "session-1",
		SessionName: "test",
		Tool:        "claude",
		Cwd:         "/tmp",
		CreatedAt:   time.Now().UnixMilli(),
	}

	// Set session state
	err := client.SetSessionState(ctx, state)
	if err != nil {
		t.Fatalf("SetSessionState failed: %v", err)
	}

	// Verify TTL is set (should be 24 hours)
	key := SessionStateKeyPrefix + state.SessionID
	ttl := mr.TTL(key)

	expectedTTL := 24 * time.Hour
	if ttl < expectedTTL-time.Second || ttl > expectedTTL+time.Second {
		t.Errorf("TTL mismatch: got %v, want ~%v", ttl, expectedTTL)
	}
}

func TestCrashEventsExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	event := &types.CrashEvent{
		SessionID:   "session-1",
		Timestamp:   time.Now().UnixMilli(),
		Reason:      "Test",
		WillRestart: true,
	}

	// Record crash event
	err := client.RecordCrashEvent(ctx, event)
	if err != nil {
		t.Fatalf("RecordCrashEvent failed: %v", err)
	}

	// Verify TTL is set (should be 24 hours)
	key := CrashEventKeyPrefix + event.SessionID
	ttl := mr.TTL(key)

	expectedTTL := 24 * time.Hour
	if ttl < expectedTTL-time.Second || ttl > expectedTTL+time.Second {
		t.Errorf("TTL mismatch: got %v, want ~%v", ttl, expectedTTL)
	}
}
