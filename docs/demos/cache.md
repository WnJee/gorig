# cache 使用指南与场景示例 (Cache Guide & Demos)

`cache` 是 Gorig 框架提供的通用高性能缓存套件，支持内存缓存（In-Memory）、Redis、SQLite 与 JSON 文件缓存，并内置了防击穿的 Singleflight 合并并发请求与分布式锁。

---

## 目录
- [1. 基础缓存操作 (Get / Set / Del)](#1-基础缓存操作-get--set--del)
- [2. 防击穿自动加载缓存 (Remember)](#2-防击穿自动加载缓存-remember)
- [3. 分布式锁与重试 (WithLock / WithLockTimeout)](#3-分布式锁与重试-withlock--withlocktimeout)
- [4. 多级缓存管理 (Tool)](#4-多级缓存管理-tool)
- [5. 独立缓存实例 (New)](#5-独立缓存实例-new)

---

## 1. 基础缓存操作 (Get / Set / Del)

默认使用全局缓存（自动适配 Redis，未配置 Redis 时降级为进程级内存缓存）：

```go
import "github.com/WnJee/gorig/cache"

// 1. 设置缓存（支持泛型强类型与过期时间）
err := cache.Set("user:1001", userProfile, 10*time.Minute)

// 2. 读取缓存
profile, err := cache.Get[UserProfile]("user:1001")
if err != nil {
    if errors.Is(err, cache.ErrCacheMiss) {
        // 缓存未命中
    }
}

// 3. 读取带默认值兜底
cachedName := cache.GetOrDefault("config:site_name", "Default Site")

// 4. 读取或设置初始值
val, err := cache.GetOrSet("counter:init", 5*time.Minute, 100)

// 5. 删除缓存
_ = cache.Del("user:1001")

// 6. 计数器自增
count, _ := cache.Incr("page:views:today")
```

---

## 2. 防击穿自动加载缓存 (Remember)

`Remember` 使用 Singleflight 机制：当高并发下多个协程同时请求不存在的 Key 时，仅有一个协程会调用 `fallback` 回源加载数据并写入缓存，其他协程阻塞共享结果，彻底杜绝**缓存击穿**。

```go
func GetUserProfile(ctx context.Context, userID int64) (*UserProfile, error) {
    cacheKey := fmt.Sprintf("profile:%d", userID)

    return cache.Remember(cacheKey, 30*time.Minute, func() (*UserProfile, error) {
        // 只有当缓存未命中时才会执行数据库查询
        user, err := dx.On[User](ctx).WithID(userID).FirstData()
        if err != nil {
            return nil, err
        }
        return &UserProfile{
            ID:       user.ID,
            Username: user.Username,
            Role:     user.Role,
        }, nil
    })
}
```

---

## 3. 分布式锁与重试 (WithLock / WithLockTimeout)

当配置了 Redis 时，使用基于 Redis SETNX + 安全 Lua 脚本防误删锁；未配置 Redis 时自动降级为进程级内存排他锁。

### 场景 A：无阻塞尝试获取锁 (WithLock)
```go
func DeductStock(ctx context.Context, goodsID int64, count int) error {
    lockKey := fmt.Sprintf("stock:%d", goodsID)

    // 尝试加锁 5 秒，执行扣减库存逻辑
    return cache.WithLock(ctx, lockKey, 5*time.Second, func() error {
        // 临界区代码
        stock := queryStock(goodsID)
        if stock < count {
            return errors.New("insufficient stock")
        }
        return updateStock(goodsID, stock-count)
    })
}
```

### 场景 B：超时重试获取锁 (WithLockTimeout)
```go
func ProcessOrder(ctx context.Context, orderID string) error {
    lockKey := fmt.Sprintf("order:process:%s", orderID)

    // 锁持有 10 秒，若被占用则每 50ms 重试一次，最多等待 3 秒超时
    return cache.WithLockTimeout(ctx, lockKey, 10*time.Second, 3*time.Second, func() error {
        return executePayment(orderID)
    })
}
```

---

## 4. 多级缓存管理 (Tool)

多级缓存依次查询 L1（内存） -> L2（Redis），当 L1 未命中而 L2 命中时自动回填 L1，兼顾极致读取性能与跨节点一致性：

```go
var userCacheTool = cache.NewCacheTool(
    context.Background(),
    []cache.Cache[any]{
        cache.NewGoCache[any](5*time.Minute, 1*time.Minute), // L1 内存
        cache.GetRedisInstance[any](context.Background()),   // L2 Redis
    },
    func(key string) (any, error) {
        // 全量未命中时的 Loader 回源函数
        return db.QueryUser(key)
    },
)

// 读取（自动多级回填与防击穿保护）
data, err := userCacheTool.Get("user:2001", 10*time.Minute)
```

---

## 5. 独立缓存实例 (New)

```go
// 1. 独立内存缓存
memCache := cache.New[string](cache.Memory, 10*time.Minute, 1*time.Minute)

// 2. 独立 SQLite 本地持久化缓存
sqliteCache := cache.New[ReportData](cache.Sqlite, "reports")

// 3. 独立 JSON 文件本地持久化缓存
jsonCache := cache.New[AppConfig](cache.JSON, "app_configs")
```
