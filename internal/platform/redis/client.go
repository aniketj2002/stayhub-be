package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	timersZSetKey = "stayhub:timers"
)

var unlockScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`)

type Client struct {
	RDB *goredis.Client
}

func NewClient(ctx context.Context, addr, password string, db int) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{RDB: rdb}, nil
}

func (c *Client) Close() error {
	if c == nil || c.RDB == nil {
		return nil
	}
	return c.RDB.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.RDB.Ping(ctx).Err()
}

// TryLock acquires a short-lived distributed lock using SET NX.
func (c *Client) TryLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	ok, err := c.RDB.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Unlock releases the lock only if the stored token matches.
func (c *Client) Unlock(ctx context.Context, key, token string) error {
	_, err := unlockScript.Run(ctx, c.RDB, []string{key}, token).Result()
	return err
}

// ScheduleTimer schedules a delayed job member (e.g., "expire_pending:<reservation_id>") at dueAt.
func (c *Client) ScheduleTimer(ctx context.Context, member string, dueAt time.Time) error {
	return c.RDB.ZAdd(ctx, timersZSetKey, goredis.Z{
		Score:  float64(dueAt.UTC().Unix()),
		Member: member,
	}).Err()
}

// FetchDueTimers returns timer members whose score <= now.Unix().
func (c *Client) FetchDueTimers(ctx context.Context, now time.Time, count int64) ([]string, error) {
	return c.RDB.ZRangeByScore(ctx, timersZSetKey, &goredis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now.UTC().Unix(), 10),
		Count: count,
	}).Result()
}

// RemoveTimer removes a processed or cancelled timer from the sorted set.
func (c *Client) RemoveTimer(ctx context.Context, member string) error {
	return c.RDB.ZRem(ctx, timersZSetKey, member).Err()
}

// SetJSON serializes val as JSON and stores it with the given TTL.
func (c *Client) SetJSON(ctx context.Context, key string, val any, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.RDB.Set(ctx, key, b, ttl).Err()
}

// GetJSON retrieves a JSON value into dest. Returns (false, nil) on cache miss.
func (c *Client) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	b, err := c.RDB.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, dest); err != nil {
		return false, err
	}
	return true, nil
}
