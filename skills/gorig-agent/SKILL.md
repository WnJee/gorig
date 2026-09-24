---
name: gorig-agent
description: Comprehensive development and delivery skill for building AI-native Go backend services using the Gorig framework. Use for scaffolding, routing, generic API binding (apix), fluent ORM (dx/domainx), multi-level caching (cache), HTTP/SSE streaming (httpx/ssex), cron tasks (cronx), JWT auth (mid/tokenx), pubsub messaging (mid/messagex), object storage (storage), and structured utils.
---

# Gorig AI Agent Skill (`gorig-agent`)

This skill provides an authoritative, source-aware delivery standard for AI agents building, refactoring, and maintaining Go backend applications powered by the **Gorig Framework** (`github.com/WnJee/gorig`).

---

## 1. Architectural Principles & Boundaries

All Gorig backend services strictly adhere to the 4-tier layered architecture:

```
[ HTTP Request ]
       │
       ▼
 1. Router        (httpx / gin engine: route definitions, middleware chains, auth)
       │
       ▼
 2. Controller    (apix: BindReq[T], parameter parsing, validation, apix.Ok/Err response)
       │
       ▼
 3. Service       (Business logic orchestration, cache, events, transactions, cron)
       │
       ▼
 4. Model / DX    (domainx/dx fluent ORM, multi-engine MySQL/MongoDB/SQLite, Snowflake ID)
```

### Module File Layout
Each business domain feature must be encapsulated in `domain/<feature>/`:

```text
domain/user/
├── router.go       # Defines RouteGroup and registers endpoints with httpx
├── controller.go   # Unpacks request via apix.BindReq, calls service, returns apix response
├── service.go      # Pure business logic, caching, event dispatch, DX operations
├── dto.go          # Request & Response structs with binding & validation tags
└── model/
    └── user.go     # GORM / Mongo data model definition (embeds dx.Model)
```

---

## 2. Core Package Reference & Implementation Standards

### 2.1 `apix` — Request Binding, Validation & Responses

#### Generic Request Binding
Always prefer `apix.BindReq[T](c)` over manual binding boilerplate. It automatically extracts parameters across JSON body, Query strings, Route parameters, and Form data, and performs struct validation:

```go
type CreateUserReq struct {
    Username string `json:"username" binding:"required,min=3,max=32"`
    Email    string `json:"email" binding:"required,email"`
    Age      int    `json:"age" binding:"gte=0,lte=150"`
    Role     string `json:"role" binding:"omitempty,oneof=admin user"`
}

func (ctrl *UserController) Create(c *gin.Context) {
    req, err := apix.BindReq[CreateUserReq](c)
    if err != nil {
        apix.Err(c, 400, "Invalid parameters: "+err.Error())
        return
    }
    
    res, err := ctrl.svc.CreateUser(c.Request.Context(), req)
    if err != nil {
        apix.Err(c, 500, err.Error())
        return
    }
    apix.Ok(c, res)
}
```

#### Typed Single-Parameter Extraction
```go
// Route parameter: /users/:id
id := apix.Param[int64](c, "id")

// Query parameter with default fallback
keyword := apix.Query[string](c, "keyword", "")
page := apix.Query[int](c, "page", 1)

// Header extraction
traceID := apix.Header[string](c, "X-Trace-ID", "")
```

#### Standardized Response Helpers
```go
// 200 OK standard response: {"code": 200, "msg": "success", "data": ...}
apix.Ok(c, data)

// 200 OK paginated response: {"code": 200, "msg": "success", "data": {"items": [...], "total": 100, "page": 1, "page_size": 20}}
apix.OkPage(c, items, total, page, pageSize)

// Business Error response: {"code": 40001, "msg": "user already exists", "data": null}
apix.Err(c, 40001, "user already exists")
apix.ErrData(c, 40001, "user already exists", extraDetails)
```

---

### 2.2 `domainx` / `dx` — Type-Safe Fluent ORM

Gorig provides the `dx` fluent database query layer across MySQL, SQLite, and MongoDB.

#### Model Definition
```go
type User struct {
    dx.Model        // Automatically embeds ID (Snowflake/uint64), CreatedAt, UpdatedAt, DeletedAt
    Username string `gorm:"column:username;type:varchar(64);uniqueIndex;not null" json:"username"`
    Email    string `gorm:"column:email;type:varchar(128);index" json:"email"`
    Status   int    `gorm:"column:status;default:1" json:"status"`
}

func (User) TableName() string {
    return "users"
}
```

#### Querying & CRUD Operations
```go
// 1. Query multiple records with conditions
var users []User
err := dx.On().
    Where("status", 1).
    WhereIn("id", []int64{101, 102, 103}).
    OrderByDesc("created_at").
    Find(&users)

// 2. Query first matching record
var user User
err := dx.On().Where("email", email).First(&user)

// 3. Create record
newUser := User{Username: "alice", Email: "alice@example.com"}
err := dx.On().Create(&newUser)

// 4. Update single column or map
err := dx.On().Where("id", user.ID).Update("status", 2)
err := dx.On().Where("id", user.ID).Updates(map[string]any{
    "username": "alice_updated",
    "status":   1,
})

// 5. Delete
err := dx.On().Where("id", user.ID).Delete(&User{})

// 6. Pagination
var pagedUsers []User
total, err := dx.On().
    Where("status", 1).
    OrderByDesc("id").
    Paginate(&pagedUsers, page, pageSize)

// 7. Atomic Database Transactions
err := dx.On().Transaction(func(tx *dx.Engine) error {
    if err := tx.Where("id", fromID).Update("balance", gorm.Expr("balance - ?", amount)); err != nil {
        return err
    }
    if err := tx.Where("id", toID).Update("balance", gorm.Expr("balance + ?", amount)); err != nil {
        return err
    }
    return nil
})
```

---

### 2.3 `cache` — Multi-Level Caching & Anti-Stampede

Supports `Memory`, `Redis`, `Sqlite`, and `Json` cache drivers with seamless unified APIs.

#### Anti-Stampede Singleflight (`Remember`)
Use `cache.Remember` for cache-aside patterns. It leverages singleflight to prevent cache penetration, avalanche, and breakdown:
```go
func (s *UserService) GetUserCached(ctx context.Context, userID int64) (*UserDTO, error) {
    cacheKey := fmt.Sprintf("user:info:%d", userID)
    
    return cache.Remember[UserDTO](ctx, cacheKey, 30*time.Minute, func() (UserDTO, error) {
        var user model.User
        if err := dx.On().Where("id", userID).First(&user); err != nil {
            return UserDTO{}, err
        }
        return toDTO(user), nil
    })
}
```

#### Distributed Lock (`WithLock` / `WithLockTimeout`)
```go
// Distributed lock with execution callback
err := cache.WithLock(ctx, "lock:order:create:1001", 10*time.Second, func() error {
    // Critical section
    return s.processOrder(ctx, 1001)
})

// Distributed lock with acquisition wait timeout
err := cache.WithLockTimeout(ctx, "lock:inventory:deduct", 10*time.Second, 3*time.Second, func() error {
    return s.deductInventory(ctx, itemID, quantity)
})
```

---

### 2.4 `httpx` & `ssex` — HTTP Engine & Real-Time SSE Streaming

#### Generic HTTP Client
```go
// Typed GET request
res, err := httpx.GetJSON[RemoteApiResponse](ctx, "https://api.example.com/data", httpx.Headers{
    "Authorization": "Bearer " + token,
})

// Typed POST request
res, err := httpx.PostJSON[CreateResult](ctx, "https://api.example.com/items", payload, nil)
```

#### Server-Sent Events (SSE) for AI & Streaming
Use `ssex` for LLM token streaming, notification feeds, or live dashboards:

```go
func (ctrl *ChatController) StreamChat(c *gin.Context) {
    streamer := ssex.NewStreamer(c)
    
    for token := range aiStreamChan {
        if err := streamer.Push(token); err != nil {
            return // Client disconnected
        }
    }
    _ = streamer.PushDone()
}

// SSE with automatic heartbeat keeper
func (ctrl *MonitorController) LiveMetrics(c *gin.Context) {
    ssex.StreamWithHeartbeat(c, 15*time.Second, func(s *ssex.Streamer) error {
        for metric := range metricsChan {
            if err := s.PushJSON(metric); err != nil {
                return err
            }
        }
        return nil
    })
}
```

---

### 2.5 `cronx` — Distributed & Local Scheduler

Supports standard cron expressions, fixed intervals, delay jobs, and Redis-backed distributed leasing:

```go
// 1. Standard Cron Task
cronx.AddCronTask("daily_cleanup", "0 0 3 * * ?", func(ctx context.Context) error {
    return s.cleanupExpiredData(ctx)
})

// 2. Fixed Interval Periodic Task
cronx.AddIntervalTask("sync_rates", 5*time.Minute, func(ctx context.Context) error {
    return s.syncExchangeRates(ctx)
})

// 3. One-Shot Delay Task
cronx.AddDelayTask("cancel_unpaid_order", 15*time.Minute, func(ctx context.Context) error {
    return s.autoCancelOrder(ctx, orderID)
})

// 4. Redis Distributed Task (High-Availability Cluster with Lease Auto-Recovery)
cronx.AddDistributedCronTask("cluster_billing", "0 0 1 * * ?", 10*time.Minute, func(ctx context.Context) error {
    return s.runBillingCycle(ctx)
})
```

---

### 2.6 `mid/tokenx` — JWT Authentication & Token Lifecycle

```go
// 1. Issue JWT Token
claims := map[string]any{"role": "admin", "tenant_id": 42}
tokenStr, err := tokenx.GenerateToken("user_1001", claims, 24*time.Hour)

// 2. Mount Auth Middleware in router
authGroup := router.Group("/api/v1/admin")
authGroup.Use(tokenx.AuthMiddleware())
{
    authGroup.GET("/profile", ctrl.GetProfile)
}

// 3. Extract Context in Controller
func (ctrl *AdminController) GetProfile(c *gin.Context) {
    userID := tokenx.GetUserID(c)
    claims, err := tokenx.GetClaims[map[string]any](c)
    // ...
}

// 4. Logout / Revoke Token (Blacklist)
func (ctrl *AuthController) Logout(c *gin.Context) {
    token := tokenx.ExtractToken(c)
    _ = tokenx.RevokeToken(token)
    apix.Ok(c, "Logged out successfully")
}
```

---

### 2.7 `mid/messagex` — Pub/Sub Event Bus

Supports in-memory and Redis distributed event brokers with typed payload serialization:

```go
type OrderPaidEvent struct {
    OrderID int64     `json:"order_id"`
    Amount  float64   `json:"amount"`
    PaidAt  time.Time `json:"paid_at"`
}

// Publish Event
err := messagex.PublishEvent(ctx, "order.paid", OrderPaidEvent{
    OrderID: 10023,
    Amount:  99.5,
    PaidAt:  time.Now(),
})

// Subscribe Event (Typed Listener)
messagex.SubscribeEvent[OrderPaidEvent](ctx, "order.paid", func(ctx context.Context, e OrderPaidEvent) error {
    logger.Info(ctx, "Received order.paid event", "order_id", e.OrderID, "amount", e.Amount)
    return s.processOrderFulfillment(ctx, e.OrderID)
})
```

---

### 2.8 `storage` — Unified Object Storage

Provides unified abstraction over Local Disk, AWS S3, Aliyun OSS, and MinIO:

```go
// Direct String/Bytes/File upload
url, err := storage.SaveString(ctx, "articles/2026/intro.md", markdownContent)
url, err := storage.SaveBytes(ctx, "avatars/u1001.png", pngBytes)
url, err := storage.SaveFile(ctx, "uploads/attachment.pdf", multipartFileHeader)

// Read / Delete
reader, err := storage.Get(ctx, "articles/2026/intro.md")
err := storage.Delete(ctx, "articles/2026/intro.md")

// Generate Presigned Upload/Download URL
signedURL, err := storage.GetPresignedURL(ctx, "backups/db.sql", 15*time.Minute)
```

---

### 2.9 `utils` — Logging, Config, Crypto, Errors & Alerts

```go
// Structured Logging with Context
logger.Info(ctx, "Order processed", "order_id", 1001, "latency_ms", 45)
logger.Error(ctx, "Failed to connect payment gateway", "err", err)

// Strongly Typed Configuration
port := cofigure.Get[int]("server.port", 8080)
dbName := cofigure.Get[string]("database.mysql.name", "main")

// Password Hashing & Encryption
hash, err := encrypt.BcryptHash("user_password")
valid := encrypt.BcryptVerify("user_password", hash)
encryptedText, err := encrypt.AESEncrypt("sensitive_token", aesKey)

// Business Error Definition
return errors.NewBizError(40001, "Account balance insufficient")

// Multi-Channel Instant Alerting (DingTalk, Feishu, WeChat Work)
alert.Send(ctx, "Database Connection Spike", "Active connections exceeded 90% threshold.")
```

---

## 3. AI Agent Quality Standards & Rules

1. **Clean Code & Modern Go Idioms**:
   - Always use generics (`apix.BindReq[T]`, `cache.Remember[T]`, `dx.Paginate[T]`, `messagex.SubscribeEvent[T]`).
   - Do not write legacy shims, fallback compatibility branches, or redundant type assertions.
2. **No Test File Persistence Rule**:
   - Do NOT commit or leave temporary `*_test.go` files unless explicitly requested by the user.
   - Verify code using `go fmt ./...`, `go vet ./...`, and `go build ./...`.
3. **Layer Separation Guarantee**:
   - Controllers only handle HTTP translation (`apix.BindReq`, calling service, returning `apix.Ok`/`apix.Err`).
   - Business logic, caching, transactions, and event emissions stay strictly in Services.
   - Database queries stay in Services or Model repositories using `dx.On()`.
