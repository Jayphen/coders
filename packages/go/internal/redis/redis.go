// Package redis provides functions for interacting with Redis for session state.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Jayphen/coders/internal/config"
	"github.com/Jayphen/coders/internal/types"
	"github.com/redis/go-redis/v9"
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
