package lock

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("distributed lock: failed to acquire")

// RedisLock implements a simple distributed lock using Redis SET NX EX.
// Each lock instance holds a unique token to ensure only the holder can release it.
type RedisLock struct {
	rdb   *redis.Client
	key   string
	token string
	ttl   time.Duration
}

// NewRedisLock creates a lock handle. Call Acquire to actually obtain it.
func NewRedisLock(rdb *redis.Client, key string, ttl time.Duration) *RedisLock {
	return &RedisLock{
		rdb:   rdb,
		key:   "dlock:" + key,
		token: uuid.New().String(),
		ttl:   ttl,
	}
}

// Acquire tries to obtain the lock. Returns ErrLockNotAcquired if the lock
// is already held by another process.
func (l *RedisLock) Acquire(ctx context.Context) error {
	ok, err := l.rdb.SetNX(ctx, l.key, l.token, l.ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrLockNotAcquired
	}
	return nil
}

// AcquireWithRetry retries acquiring the lock at the given interval
// until timeout. Useful for short-lived contention.
func (l *RedisLock) AcquireWithRetry(ctx context.Context, timeout, interval time.Duration) error {
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return ErrLockNotAcquired
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := l.Acquire(ctx); err == nil {
				return nil
			}
			time.Sleep(interval)
		}
	}
}

// Release deletes the lock only if the token matches (Lua atomic check-and-delete).
func (l *RedisLock) Release(ctx context.Context) error {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		end
		return 0
	`)
	_, err := script.Run(ctx, l.rdb, []string{l.key}, l.token).Result()
	return err
}
