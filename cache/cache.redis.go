package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/sys"
	"github.com/rs/xid"
	"github.com/spf13/cast"
	"sync"
	"time"
)

var (
	redisInstance *redis.Client
	initMu        sync.Mutex
	initOnce      = &sync.Once{}
)

func RestRedisInstance() {
	initMu.Lock()
	defer initMu.Unlock()
	if redisInstance != nil {
		redisInstance.Close()
		redisInstance = nil
	}
}

func GetRedisInstance[T any](ctx context.Context) *RedisCache[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	initMu.Lock()
	defer initMu.Unlock()
	if redisInstance == nil {
		redisInstance = initRedisCache()
	}
	if redisInstance == nil {
		return nil
	}
	return &RedisCache[T]{
		Client: redisInstance,
		Ctx:    ctx,
	}
}

func initRedisCache() *redis.Client {
	addr := configure.GetString("redis.addr")
	password := configure.GetString("redis.password")
	db := configure.GetString("redis.db")
	if addr == "" {
		sys.Info("# Redis addr is empty, skipping initialization")
		return nil
	}

	cache, err := newRedisCache(RedisConfig{
		Addr:     addr,
		Password: password,
		DB:       cast.ToInt(db),
		PoolSize: 10000,
	})
	if err != nil {
		sys.Error("# failed to init Redis cache: ", err)
	}
	if cache == nil {
		sys.Error("# Redis cache is nil after initialization")
	}
	sys.Info("# Redis cache initialized")
	redisInstance = cache
	return redisInstance
}

// RedisConfig holds the Redis configuration parameters
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

type RedisCache[T any] struct {
	Client *redis.Client
	Ctx    context.Context
}

type delayedEntry struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

// Redis sorted-set members are unique by value. Wrapping the payload with a
// generated ID prevents identical delayed messages from replacing one another.
var popDueDelayedScript = redis.NewScript(`
local items = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
for _, item in ipairs(items) do

	redis.call('ZREM', KEYS[1], item)
end
return items
`)

func (r *RedisCache[T]) LPop(queue string) (value T, err error) {
	if !r.IsInitialized() {
		return value, fmt.Errorf("redis client is nil")
	}
	result, err := r.Client.LPop(r.Ctx, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return value, ErrCacheMiss
		}
		return value, err
	}
	if err = json.Unmarshal([]byte(result), &value); err != nil {
		return value, err
	}
	return value, nil
}

func (r *RedisCache[T]) AddDelayed(queue string, value T, score float64) error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	member, err := json.Marshal(delayedEntry{ID: xid.New().String(), Value: b})
	if err != nil {
		return err
	}
	return r.Client.ZAdd(r.Ctx, queue, &redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

func (r *RedisCache[T]) PopDueDelayed(queue string, now float64, limit int) ([]T, error) {
	if !r.IsInitialized() {
		return nil, fmt.Errorf("redis client is nil")
	}
	if limit <= 0 {
		limit = 100
	}
	items, err := popDueDelayedScript.Run(r.Ctx, r.Client, []string{queue}, now, limit).StringSlice()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	results := make([]T, 0, len(items))
	for _, item := range items {
		var entry delayedEntry
		if err := json.Unmarshal([]byte(item), &entry); err == nil && entry.ID != "" && len(entry.Value) > 0 {
			var v T
			if err := json.Unmarshal(entry.Value, &v); err != nil {
				return nil, err
			}
			results = append(results, v)
			continue
		}

		// Accept members written by older versions that stored the payload
		// directly, so a rolling deployment does not discard pending messages.
		var v T
		if err := json.Unmarshal([]byte(item), &v); err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, nil
}

func (r *RedisCache[T]) GetCtx() context.Context {
	return r.Ctx
}

func newRedisCache(cfg RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return client, nil
}

func (r *RedisCache[T]) IsInitialized() bool {
	return r != nil && r.Client != nil
}

func (r *RedisCache[T]) Keys() ([]string, error) {
	if !r.IsInitialized() {
		return nil, fmt.Errorf("redis client is nil")
	}
	var keys []string
	var cursor uint64
	for {
		batch, next, err := r.Client.Scan(r.Ctx, cursor, "*", 500).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

func (r *RedisCache[T]) Items() map[string]T {
	result := make(map[string]T)
	if !r.IsInitialized() {
		return result
	}
	keys, err := r.Keys()
	if err != nil {
		return result
	}
	for _, key := range keys {
		val, err := r.Get(key)
		if err == nil {
			result[key] = val
		}
	}
	return result
}

func (r *RedisCache[T]) Get(key string) (T, error) {
	var zero T
	if !r.IsInitialized() {
		return zero, fmt.Errorf("redis client is nil")
	}
	val, err := r.Client.Get(r.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return zero, ErrCacheMiss
	} else if err != nil {
		return zero, err
	}
	var data T
	if err = json.Unmarshal([]byte(val), &data); err != nil {
		return zero, err
	}
	return data, nil
}

func (r *RedisCache[T]) Set(key string, value T, expiration time.Duration) error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Client.Set(r.Ctx, key, jsonValue, expiration).Err()
}

func (r *RedisCache[T]) Del(key string) error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.Del(r.Ctx, key).Err()
}

func (r *RedisCache[T]) Exists(key string) (bool, error) {
	if !r.IsInitialized() {
		return false, fmt.Errorf("redis client is nil")
	}
	result, err := r.Client.Exists(r.Ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (r *RedisCache[T]) RPush(queue string, value T) error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Client.RPush(r.Ctx, queue, b).Err()
}

// BLPopCtx consumes the oldest item from a Redis list. It is kept separate
// from BRPopCtx because RPUSH + BRPOP has LIFO semantics, while cache queues
// and the message broker promise FIFO delivery.
func (r *RedisCache[T]) BLPopCtx(ctx context.Context, timeout time.Duration, queue string) (value T, err error) {
	if !r.IsInitialized() {
		return value, fmt.Errorf("redis client is nil")
	}
	result, err := r.Client.BLPop(ctx, timeout, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return value, ErrCacheMiss
		}
		return value, err
	}
	if len(result) != 2 {
		return value, fmt.Errorf("invalid result length from BLPop for queue %s", queue)
	}
	if err = json.Unmarshal([]byte(result[1]), &value); err != nil {
		return value, err
	}
	return value, nil
}

func (r *RedisCache[T]) BLPop(timeout time.Duration, queue string) (value T, err error) {
	return r.BLPopCtx(r.Ctx, timeout, queue)
}

func (r *RedisCache[T]) BRPopCtx(ctx context.Context, timeout time.Duration, queue string) (value T, err error) {
	if !r.IsInitialized() {
		return value, fmt.Errorf("redis client is nil")
	}
	result, err := r.Client.BRPop(ctx, timeout, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return value, ErrCacheMiss
		}
		return value, err
	}

	if len(result) != 2 {
		return value, fmt.Errorf("invalid result length from BRPop for queue %s", queue)
	}

	if err = json.Unmarshal([]byte(result[1]), &value); err != nil {
		return value, err
	}
	return
}

func (r *RedisCache[T]) BRPop(timeout time.Duration, queue string) (value T, err error) {
	return r.BRPopCtx(r.Ctx, timeout, queue)
}

func (r *RedisCache[T]) Incr(key string) (int64, error) {
	if !r.IsInitialized() {
		return 0, fmt.Errorf("redis client is nil")
	}
	return r.Client.Incr(r.Ctx, key).Result()
}

func (r *RedisCache[T]) Expire(key string, expiration time.Duration) error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.Expire(r.Ctx, key, expiration).Err()
}

func (r *RedisCache[T]) Flush() error {
	if !r.IsInitialized() {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.FlushDB(r.Ctx).Err()
}
