package cache

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WnJee/gorig/utils/logger"
	"golang.org/x/sync/singleflight"
)

// Cache is a generic cache interface that defines basic cache operations
type Cache[T any] interface {
	IsInitialized() bool
	Keys() ([]string, error)
	Items() map[string]T
	Get(key string) (T, error)
	Set(key string, value T, expiration time.Duration) error
	Del(key string) error
	Exists(key string) (bool, error)
	RPush(key string, value T) error
	BRPop(timeout time.Duration, key string) (value T, err error)
	Incr(key string) (int64, error)
	Expire(key string, expiration time.Duration) error
	Flush() error
}

type Type string

const (
	Memory Type = "memory"
	Redis  Type = "redis"
	JSON   Type = "json"
	Sqlite Type = "sqlite"
)

func sanitizeCacheName(name string) string {
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	return name
}

func New[T any](t Type, args ...any) Cache[T] {
	switch t {
	case Memory:
		var defaultExpiration, cleanupInterval = time.Minute, time.Minute
		if len(args) >= 1 {
			if value, ok := args[0].(time.Duration); ok {
				defaultExpiration = value
			}
		}
		if len(args) >= 2 {
			if value, ok := args[1].(time.Duration); ok {
				cleanupInterval = value
			}
		}
		return NewGoCache[T](defaultExpiration, cleanupInterval)
	case Redis:
		return GetRedisInstance[T](context.Background())
	case JSON:
		name := ""
		if len(args) >= 1 {
			if str, ok := args[0].(string); ok && str != "" {
				name = str
			}
		}
		if name == "" {
			name = sanitizeCacheName(filepath.Base(fmt.Sprintf("%T", new(T))))
		}
		cache, err := NewJSONCache[T](name)
		if err != nil {
			logger.Error(nil, fmt.Sprintf("Failed to create JSON cache: %v", err))
		}
		return cache
	case Sqlite:
		name := ""
		if len(args) >= 1 {
			if str, ok := args[0].(string); ok && str != "" {
				name = str
			}
		}
		if name == "" {
			name = sanitizeCacheName(filepath.Base(fmt.Sprintf("%T", new(T))))
		}
		cache, err := NewSQLiteCache[T](name)
		if err != nil {
			logger.Error(nil, fmt.Sprintf("Failed to create SQLite cache: %v", err))
		}
		return cache
	default:
		logger.Error(nil, fmt.Sprintf("Unsupported cache type: %s, using memory cache", t))
		return NewGoCache[T](time.Minute, time.Minute)
	}
}

// ErrCacheMiss indicates a cache miss error
var ErrCacheMiss = errors.New("cache miss")

var (
	defaultGroup singleflight.Group
)

func getGlobalGoCache[T any]() *GoCache[T] {
	return NewSharedGoCache[T]()
}

// GetDefaultCache returns the default cache instance for type T.
// If Redis is configured and initialized, it returns the Redis cache.
// Otherwise, it returns the shared in-memory cache.
func GetDefaultCache[T any](ctx context.Context) Cache[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	if redisIns := GetRedisInstance[T](ctx); redisIns != nil && redisIns.IsInitialized() {
		return redisIns
	}
	return getGlobalGoCache[T]()
}

// Get retrieves a value from the default cache.
func Get[T any](key string) (T, error) {
	return GetCtx[T](context.Background(), key)
}

// GetCtx retrieves a value from the default cache with context.
func GetCtx[T any](ctx context.Context, key string) (T, error) {
	c := GetDefaultCache[T](ctx)
	if c == nil {
		var zero T
		return zero, errors.New("default cache unavailable")
	}
	return c.Get(key)
}

// Set stores a value in the default cache.
func Set[T any](key string, value T, expiration time.Duration) error {
	return SetCtx[T](context.Background(), key, value, expiration)
}

// SetCtx stores a value in the default cache with context.
func SetCtx[T any](ctx context.Context, key string, value T, expiration time.Duration) error {
	c := GetDefaultCache[T](ctx)
	if c == nil {
		return errors.New("default cache unavailable")
	}
	return c.Set(key, value, expiration)
}

// Del deletes a key from the default cache.
func Del(key string) error {
	return DelCtx(context.Background(), key)
}

// DelCtx deletes a key from the default cache with context.
func DelCtx(ctx context.Context, key string) error {
	c := GetDefaultCache[any](ctx)
	if c == nil {
		return errors.New("default cache unavailable")
	}
	return c.Del(key)
}

// Exists checks if a key exists in the default cache.
func Exists(key string) (bool, error) {
	return ExistsCtx(context.Background(), key)
}

// ExistsCtx checks if a key exists in the default cache with context.
func ExistsCtx(ctx context.Context, key string) (bool, error) {
	c := GetDefaultCache[any](ctx)
	if c == nil {
		return false, errors.New("default cache unavailable")
	}
	return c.Exists(key)
}

// Incr increments a counter in the default cache.
func Incr(key string) (int64, error) {
	return IncrCtx(context.Background(), key)
}

// IncrCtx increments a counter in the default cache with context.
func IncrCtx(ctx context.Context, key string) (int64, error) {
	c := GetDefaultCache[any](ctx)
	if c == nil {
		return 0, errors.New("default cache unavailable")
	}
	return c.Incr(key)
}

// Expire sets expiration for a key in the default cache.
func Expire(key string, expiration time.Duration) error {
	return ExpireCtx(context.Background(), key, expiration)
}

// ExpireCtx sets expiration for a key in the default cache with context.
func ExpireCtx(ctx context.Context, key string, expiration time.Duration) error {
	c := GetDefaultCache[any](ctx)
	if c == nil {
		return errors.New("default cache unavailable")
	}
	return c.Expire(key, expiration)
}

// GetOrDefault retrieves a value from the default cache or returns defValue on error/miss.
func GetOrDefault[T any](key string, defValue T) T {
	val, err := Get[T](key)
	if err != nil {
		return defValue
	}
	return val
}

// GetOrSet retrieves a value or sets the default value if key misses.
func GetOrSet[T any](key string, expiration time.Duration, defaultVal T) (T, error) {
	return Remember[T](key, expiration, func() (T, error) {
		return defaultVal, nil
	})
}

// Remember retrieves data from the default cache. If cache misses, calls fallback,
// caches the result with Singleflight protection to prevent cache stampedes, and returns it.
func Remember[T any](key string, expiration time.Duration, fallback func() (T, error)) (T, error) {
	return RememberCtx[T](context.Background(), key, expiration, fallback)
}

// RememberCtx retrieves data from the default cache with context and Singleflight protection.
func RememberCtx[T any](ctx context.Context, key string, expiration time.Duration, fallback func() (T, error)) (T, error) {
	c := GetDefaultCache[T](ctx)
	return RememberWithCache[T](c, key, expiration, fallback)
}

// RememberWithCache retrieves data from the specified cache with Singleflight protection.
func RememberWithCache[T any](c Cache[T], key string, expiration time.Duration, fallback func() (T, error)) (T, error) {
	var zero T
	if c == nil || !c.IsInitialized() {
		if fallback != nil {
			return fallback()
		}
		return zero, errors.New("cache not initialized")
	}

	// 1. Fast path: check cache directly
	val, err := c.Get(key)
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		logger.Warn(nil, fmt.Sprintf("cache get error for key %s: %v", key, err))
	}

	if fallback == nil {
		return zero, ErrCacheMiss
	}

	// 2. Slow path: singleflight protection against cache stampede
	v, err, _ := defaultGroup.Do(key, func() (any, error) {
		// Double check within lock
		if val, err := c.Get(key); err == nil {
			return val, nil
		}

		data, err := fallback()
		if err != nil {
			return zero, err
		}

		if setErr := c.Set(key, data, expiration); setErr != nil {
			logger.Warn(nil, fmt.Sprintf("failed to write cache for key %s: %v", key, setErr))
		}
		return data, nil
	})

	if err != nil {
		return zero, err
	}
	return v.(T), nil
}

// LoaderFunc is a function type for loading data from an external source
type LoaderFunc[T any] func(key string) (T, error)

// Tool is a management tool for multi-level cache
type Tool[T any] struct {
	Ctx    context.Context
	caches []Cache[T]
	loader LoaderFunc[T]
	group  singleflight.Group
	mu     sync.Mutex
}

// NewCacheTool creates a new Tool instance
func NewCacheTool[T any](ctx context.Context, caches []Cache[T], loader LoaderFunc[T]) *Tool[T] {
	return &Tool[T]{
		Ctx:    ctx,
		caches: caches,
		loader: loader,
	}
}

// Remember retrieves data from multi-level cache or falls back to the loader function with singleflight protection
func (c *Tool[T]) Remember(key string, expiration time.Duration, fallback func() (T, error)) (T, error) {
	var zero T
	for i, cacheLayer := range c.caches {
		if cacheLayer == nil || !cacheLayer.IsInitialized() {
			continue
		}
		val, err := cacheLayer.Get(key)
		if err == nil {
			for j := 0; j < i; j++ {
				_ = c.caches[j].Set(key, val, expiration)
			}
			return val, nil
		}
	}

	if fallback == nil {
		if c.loader != nil {
			return c.Get(key, expiration)
		}
		return zero, ErrCacheMiss
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		for _, cacheLayer := range c.caches {
			if cacheLayer == nil || !cacheLayer.IsInitialized() {
				continue
			}
			if val, err := cacheLayer.Get(key); err == nil {
				return val, nil
			}
		}

		val, err := fallback()
		if err != nil {
			return zero, err
		}

		for _, cacheLayer := range c.caches {
			if cacheLayer == nil || !cacheLayer.IsInitialized() {
				continue
			}
			_ = cacheLayer.Set(key, val, expiration)
		}
		return val, nil
	})

	if err != nil {
		return zero, err
	}
	return v.(T), nil
}

// Get retrieves data from the cache, searching each level in order, and loads from the loader if all levels miss
func (c *Tool[T]) Get(key string, expiration time.Duration) (T, error) {
	var zero T
	// Use singleflight to prevent cache stampede
	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		var value T

		// Search each cache level in order
		for i, cacheLayer := range c.caches {
			if cacheLayer == nil || !cacheLayer.IsInitialized() {
				continue
			}
			val, err := cacheLayer.Get(key)
			if err == nil {
				logger.Info(c.Ctx, fmt.Sprintf("Cache hit in layer %d", i+1))
				value = val
				// Sync data to higher-level caches
				for j := 0; j < i; j++ {
					err = c.caches[j].Set(key, value, expiration)
					if err != nil {
						return nil, err
					}
				}
				return value, nil
			}
			if !errors.Is(err, ErrCacheMiss) {
				return zero, err
			}
		}

		// If no loader, return cacheMiss directly
		if c.loader == nil {
			return zero, ErrCacheMiss
		}

		// If all cache levels miss, load data using loader
		logger.Info(c.Ctx, "Cache miss in all layers, loading from external source")
		val, err := c.loader(key)
		if err != nil {
			return zero, err
		}
		value = val

		// Store data in all cache levels
		for _, cacheLayer := range c.caches {
			if cacheLayer == nil || !cacheLayer.IsInitialized() {
				continue
			}
			if err := cacheLayer.Set(key, value, expiration); err != nil {
				return zero, err
			}
		}

		return value, nil
	})

	if err != nil {
		return zero, err
	}

	return v.(T), nil
}

// Set stores data in all cache levels
func (c *Tool[T]) Set(key string, value T, expiration time.Duration) error {
	for _, cacheLayer := range c.caches {
		if err := cacheLayer.Set(key, value, expiration); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes data from all cache levels
func (c *Tool[T]) Delete(key string) error {
	for _, cacheLayer := range c.caches {
		if err := cacheLayer.Del(key); err != nil {
			return err
		}
	}
	return nil
}
