package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/xid"
)

var (
	unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

	memLocks   sync.Map // map[string]*memLockItem
	memLockMut sync.Mutex
)

type memLockItem struct {
	token     string
	expiresAt time.Time
	mu        sync.Mutex
}

// ErrLockAcquireFailed indicates failure to acquire lock
var ErrLockAcquireFailed = errors.New("failed to acquire distributed lock")

// WithLock executes fn while holding a distributed lock on key.
// If Redis is configured, it uses Redis SETNX with expiration and safe Lua unlock.
// If Redis is not configured, it uses in-memory mutex lock with TTL.
func WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	unlock, ok, err := TryLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLockAcquireFailed
	}
	defer unlock()
	return fn()
}

// TryLock attempts to acquire a lock on key immediately without blocking.
// Returns an unlock function, a boolean indicating if lock was acquired, and any error encountered.
func TryLock(ctx context.Context, key string, ttl time.Duration) (unlock func(), ok bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	lockKey := "lock:" + key
	token := xid.New().String()

	redisClient := GetRedisInstance[any](ctx)
	if redisClient != nil && redisClient.IsInitialized() {
		// Redis lock
		success, err := redisClient.Client.SetNX(ctx, lockKey, token, ttl).Result()
		if err != nil {
			return nil, false, err
		}
		if !success {
			return nil, false, nil
		}
		unlocked := false
		var unlMu sync.Mutex
		unlock = func() {
			unlMu.Lock()
			defer unlMu.Unlock()
			if unlocked {
				return
			}
			unlocked = true
			_ = unlockScript.Run(context.Background(), redisClient.Client, []string{lockKey}, token).Err()
		}
		return unlock, true, nil
	}

	// In-memory fallback
	memLockMut.Lock()
	val, _ := memLocks.LoadOrStore(lockKey, &memLockItem{})
	item := val.(*memLockItem)
	memLockMut.Unlock()

	item.mu.Lock()
	now := time.Now()
	if item.token != "" && now.Before(item.expiresAt) {
		item.mu.Unlock()
		return nil, false, nil
	}
	item.token = token
	item.expiresAt = now.Add(ttl)
	item.mu.Unlock()

	unlocked := false
	var unlMu sync.Mutex
	unlock = func() {
		unlMu.Lock()
		defer unlMu.Unlock()
		if unlocked {
			return
		}
		unlocked = true
		item.mu.Lock()
		if item.token == token {
			item.token = ""
			item.expiresAt = time.Time{}
		}
		item.mu.Unlock()
	}
	return unlock, true, nil
}

// Lock blocks until lock is acquired or timeout is reached.
func Lock(ctx context.Context, key string, ttl time.Duration, retryInterval, timeout time.Duration) (unlock func(), err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if retryInterval <= 0 {
		retryInterval = 50 * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		unl, ok, err := TryLock(ctx, key, ttl)
		if err != nil {
			return nil, err
		}
		if ok {
			return unl, nil
		}

		if time.Now().Add(retryInterval).After(deadline) {
			return nil, fmt.Errorf("%w: timeout after %v for key %s", ErrLockAcquireFailed, timeout, key)
		}
		time.Sleep(retryInterval)
	}
}
