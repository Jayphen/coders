// Package redis provides functions for interacting with Redis for session state.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/types"
	"github.com/redis/go-redis/v9"
)

var (
	clientOnce     sync.Once
	singletonClient *Client
	clientErr      error
)

const (
	// PromiseKeyPrefix is the Redis key prefix for promises.
	PromiseKeyPrefix = "coders:promise:"
	// PaneKeyPrefix is the Redis key prefix for heartbeats.
	PaneKeyPrefix = "coders:pane:"
	// HealthKeyPrefix is the Redis key prefix for health check results.
	HealthKeyPrefix = "coders:health:"
	// HealthSummaryKey is the Redis key for the health check summary.
	HealthSummaryKey = "coders:health:summary"
	// SessionStateKeyPrefix is the Redis key prefix for session state (for restart-on-crash).
	SessionStateKeyPrefix = "coders:session-state:"
	// CrashEventKeyPrefix is the Redis key prefix for crash events.
	CrashEventKeyPrefix = "coders:crash:"
	// LoopNotificationKeyPrefix is the Redis key prefix for loop completion notifications.
	LoopNotificationKeyPrefix = "coders:loop:notification:"
)

// Client wraps a Redis client with coders-specific operations.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client.
func NewClient() (*Client, error) {
	cfg, err := config.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	rdb := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// GetClient returns a singleton Redis client instance.
func GetClient() (*Client, error) {
	clientOnce.Do(func() {
		singletonClient, clientErr = NewClient()
	})
	return singletonClient, clientErr
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// GetPromises returns all session promises.
func (c *Client) GetPromises(ctx context.Context) (map[string]*types.CoderPromise, error) {
	promises := make(map[string]*types.CoderPromise)

	// Scan for all promise keys
	keys, err := c.scanKeys(ctx, PromiseKeyPrefix+"*")
	if err != nil {
		return promises, err
	}

	if len(keys) == 0 {
		return promises, nil
	}

	// Get all values
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return promises, err
	}

	for _, val := range values {
		if val == nil {
			continue
		}

		str, ok := val.(string)
		if !ok {
			continue
		}

		var promise types.CoderPromise
		if err := json.Unmarshal([]byte(str), &promise); err != nil {
			continue
		}

		promises[promise.SessionID] = &promise
	}

	return promises, nil
}

// GetHeartbeats returns all session heartbeats.
func (c *Client) GetHeartbeats(ctx context.Context) (map[string]*types.HeartbeatData, error) {
	heartbeats := make(map[string]*types.HeartbeatData)

	// Scan for all heartbeat keys
	keys, err := c.scanKeys(ctx, PaneKeyPrefix+"*")
	if err != nil {
		return heartbeats, err
	}

	if len(keys) == 0 {
		return heartbeats, nil
	}

	// Get all values
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return heartbeats, err
	}

	for _, val := range values {
		if val == nil {
			continue
		}

		str, ok := val.(string)
		if !ok {
			continue
		}

		var hb types.HeartbeatData
		if err := json.Unmarshal([]byte(str), &hb); err != nil {
			continue
		}

		if hb.SessionID != "" {
			heartbeats[hb.SessionID] = &hb
		}
	}

	return heartbeats, nil
}

// SetPromise stores a promise for a session.
func (c *Client) SetPromise(ctx context.Context, promise *types.CoderPromise) error {
	data, err := json.Marshal(promise)
	if err != nil {
		return err
	}

	key := PromiseKeyPrefix + promise.SessionID
	return c.rdb.Set(ctx, key, data, 0).Err()
}

// DeletePromise deletes a promise for a session.
func (c *Client) DeletePromise(ctx context.Context, sessionID string) error {
	key := PromiseKeyPrefix + sessionID
	return c.rdb.Del(ctx, key).Err()
}

// GetPromise returns a single promise for a session.
func (c *Client) GetPromise(sessionID string) (*types.CoderPromise, error) {
	ctx := context.Background()
	key := PromiseKeyPrefix + sessionID
	data, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var promise types.CoderPromise
	if err := json.Unmarshal([]byte(data), &promise); err != nil {
		return nil, err
	}

	return &promise, nil
}

// SetJSON stores a JSON-serializable value with a TTL.
func (c *Client) SetJSON(key string, value interface{}, ttl time.Duration) error {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// GetRaw retrieves a raw string value from Redis.
func (c *Client) GetRaw(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// MGetRaw retrieves multiple raw string values from Redis.
func (c *Client) MGetRaw(ctx context.Context, keys []string) ([]string, error) {
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make([]string, len(values))
	for i, val := range values {
		if val != nil {
			if str, ok := val.(string); ok {
				result[i] = str
			}
		}
	}
	return result, nil
}

// ScanKeys scans for all keys matching a pattern (exported version).
func (c *Client) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	return c.scanKeys(ctx, pattern)
}

// SetHeartbeat stores a heartbeat for a session.
func (c *Client) SetHeartbeat(ctx context.Context, hb *types.HeartbeatData) error {
	data, err := json.Marshal(hb)
	if err != nil {
		return err
	}

	key := PaneKeyPrefix + hb.SessionID
	// Heartbeats expire after 10 minutes
	return c.rdb.Set(ctx, key, data, 10*time.Minute).Err()
}

// scanKeys scans for all keys matching a pattern.
func (c *Client) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64

	for {
		var batch []string
		var err error
		batch, cursor, err = c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return keys, err
		}

		keys = append(keys, batch...)

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// IsAvailable checks if Redis is available.
func IsAvailable() bool {
	client, err := NewClient()
	if err != nil {
		return false
	}
	defer client.Close()
	return true
}

// DetermineHeartbeatStatus determines the status based on heartbeat age.
func DetermineHeartbeatStatus(hb *types.HeartbeatData) types.HeartbeatStatus {
	if hb == nil {
		return types.HeartbeatDead
	}

	age := time.Since(time.UnixMilli(hb.Timestamp))

	if age < time.Minute {
		return types.HeartbeatHealthy
	} else if age < 5*time.Minute {
		return types.HeartbeatStale
	}
	return types.HeartbeatDead
}

// GetHealthChecks returns all session health check results.
func (c *Client) GetHealthChecks(ctx context.Context) (map[string]*types.HealthCheckResult, error) {
	healthChecks := make(map[string]*types.HealthCheckResult)

	// Scan for all health check keys (excluding summary)
	keys, err := c.scanKeys(ctx, HealthKeyPrefix+"*")
	if err != nil {
		return healthChecks, err
	}

	// Filter out the summary key
	var sessionKeys []string
	for _, k := range keys {
		if k != HealthSummaryKey {
			sessionKeys = append(sessionKeys, k)
		}
	}

	if len(sessionKeys) == 0 {
		return healthChecks, nil
	}

	// Get all values
	values, err := c.rdb.MGet(ctx, sessionKeys...).Result()
	if err != nil {
		return healthChecks, err
	}

	for _, val := range values {
		if val == nil {
			continue
		}

		str, ok := val.(string)
		if !ok {
			continue
		}

		var hc types.HealthCheckResult
		if err := json.Unmarshal([]byte(str), &hc); err != nil {
			continue
		}

		if hc.SessionID != "" {
			healthChecks[hc.SessionID] = &hc
		}
	}

	return healthChecks, nil
}

// SetHealthCheck stores a health check result for a session.
func (c *Client) SetHealthCheck(ctx context.Context, hc *types.HealthCheckResult) error {
	data, err := json.Marshal(hc)
	if err != nil {
		return err
	}

	key := HealthKeyPrefix + hc.SessionID
	// Health checks expire after 10 minutes (same as heartbeats)
	return c.rdb.Set(ctx, key, data, 10*time.Minute).Err()
}

// GetHealthSummary returns the latest health check summary.
func (c *Client) GetHealthSummary(ctx context.Context) (*types.HealthCheckSummary, error) {
	data, err := c.rdb.Get(ctx, HealthSummaryKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var summary types.HealthCheckSummary
	if err := json.Unmarshal([]byte(data), &summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

// SetHealthSummary stores a health check summary.
func (c *Client) SetHealthSummary(ctx context.Context, summary *types.HealthCheckSummary) error {
	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}

	// Summary expires after 5 minutes
	return c.rdb.Set(ctx, HealthSummaryKey, data, 5*time.Minute).Err()
}

// DeleteHealthCheck deletes a health check result for a session.
func (c *Client) DeleteHealthCheck(ctx context.Context, sessionID string) error {
	key := HealthKeyPrefix + sessionID
	return c.rdb.Del(ctx, key).Err()
}

// SetSessionState stores session state for restart-on-crash functionality.
func (c *Client) SetSessionState(ctx context.Context, state *types.SessionState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	key := SessionStateKeyPrefix + state.SessionID
	// Session state expires after 24 hours (sessions shouldn't run longer than this)
	return c.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}

// GetSessionState retrieves session state for a given session ID.
func (c *Client) GetSessionState(ctx context.Context, sessionID string) (*types.SessionState, error) {
	key := SessionStateKeyPrefix + sessionID
	data, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var state types.SessionState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// DeleteSessionState removes session state for a given session ID.
func (c *Client) DeleteSessionState(ctx context.Context, sessionID string) error {
	key := SessionStateKeyPrefix + sessionID
	return c.rdb.Del(ctx, key).Err()
}

// RecordCrashEvent stores a crash event for a session.
func (c *Client) RecordCrashEvent(ctx context.Context, event *types.CrashEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Use a list to store crash history, keep last 10 events per session
	key := CrashEventKeyPrefix + event.SessionID
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 9)
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err = pipe.Exec(ctx)
	return err
}

// GetCrashEvents retrieves crash events for a session.
func (c *Client) GetCrashEvents(ctx context.Context, sessionID string) ([]types.CrashEvent, error) {
	key := CrashEventKeyPrefix + sessionID
	data, err := c.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var events []types.CrashEvent
	for _, item := range data {
		var event types.CrashEvent
		if err := json.Unmarshal([]byte(item), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// SetLoopNotification stores a loop completion notification.
func (c *Client) SetLoopNotification(ctx context.Context, notification *types.LoopNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	key := LoopNotificationKeyPrefix + notification.LoopID
	// Notifications expire after 24 hours
	return c.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}
