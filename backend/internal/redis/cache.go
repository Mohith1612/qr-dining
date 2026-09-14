package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"
)

// Cache provides a simple JSON get/set cache with TTL.
// Used to avoid repeated DB lookups for hot read paths (e.g., session validation on WS auth).
// A cache miss always falls back to PostgreSQL — Redis is never the source of truth.
type Cache struct {
	client *goredis.Client
	hits   prometheus.Counter // optional; nil if metrics not provided
	misses prometheus.Counter
}

func NewCache(client *goredis.Client, hits, misses prometheus.Counter) *Cache {
	return &Cache{client: client, hits: hits, misses: misses}
}

// Set serializes v as JSON and stores it under key with the given TTL.
func (c *Cache) Set(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// Get deserializes the cached JSON into dst. Returns (false, nil) on cache miss.
func (c *Cache) Get(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		if c.misses != nil {
			c.misses.Inc()
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("cache unmarshal: %w", err)
	}
	if c.hits != nil {
		c.hits.Inc()
	}
	return true, nil
}

// Invalidate deletes a cache key.
func (c *Cache) Invalidate(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// SAdd adds a member to a Redis Set with an expiry. Used for token set tracking.
func (c *Cache) SAdd(ctx context.Context, key, member string, ttl time.Duration) error {
	pipe := c.client.Pipeline()
	pipe.SAdd(ctx, key, member)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SRem removes a single member from a Redis Set, leaving the rest of the set
// intact. Used to untrack one staff token on logout without disturbing that
// staff member's other sessions.
func (c *Cache) SRem(ctx context.Context, key, member string) error {
	return c.client.SRem(ctx, key, member).Err()
}

// SMembers returns all members of a Redis Set. Returns nil slice on missing key.
func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.SMembers(ctx, key).Result()
}

// DeleteMany deletes multiple keys in a single round-trip.
func (c *Cache) DeleteMany(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// DeleteByPattern deletes every key matching a glob pattern (e.g. "featgate:*").
// It scans in batches rather than using KEYS so it never blocks Redis. Intended
// for rare operator-triggered invalidations, not hot paths.
func (c *Cache) DeleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return fmt.Errorf("cache scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache del: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
