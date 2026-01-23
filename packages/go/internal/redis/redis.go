// Package redis provides functions for interacting with Redis for session state.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Jayphen/coders/internal/types"
	"github.com/redis/go-redis/v9"
)

const (
	// PromiseKeyPrefix is the Redis key prefix for promises.
	PromiseKeyPrefix = "coders:promise:"
	// PaneKeyPrefix is the Redis key prefix for heartbeats.
	PaneKeyPrefix = "coders:pane:"
	// DefaultRedisURL is the default Redis connection URL.
	DefaultRedisURL = "redis://localhost:6379"
)

// Client wraps a Redis client with coders-specific operations.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client.
func NewClient() (*Client, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = DefaultRedisURL
	}

	opts, err := redis.ParseURL(url)
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
