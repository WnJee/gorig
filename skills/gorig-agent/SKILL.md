---
name: gorig-agent
description: Comprehensive development and delivery skill for building AI-native Go backend services using the Gorig framework. Use for project scaffolding, module generation (gorig_gen_cli), HTTP routing & generic parameter binding (apix), fluent ORM (dx/domainx), multi-level caching (cache), HTTP/SSE streaming (httpx/ssex), cron tasks (cronx), JWT auth (mid/tokenx), pubsub messaging (mid/messagex), object storage (storage), and structured utils.
---

# Gorig AI Agent Skill (`gorig-agent`)

This skill provides an authoritative, source-aware delivery standard for AI agents building, refactoring, and maintaining Go backend applications powered by the **Gorig Framework** (`github.com/WnJee/gorig`) and the official **`gorig_gen_cli`** tool.

---

## 1. Project Scaffolding & Architecture Rules

### 1.1 `gorig_gen_cli` Priority Rule (脚手架与模块生成优先原则)

> **CRITICAL RULE**: When initializing a new project or creating a new business domain module, the AI Agent **MUST prioritize directly executing `gorig_gen_cli` CLI commands** instead of manually creating directories, boilerplate files, or stitching `init.go` import statements from scratch.

#### Commands to Use:
1. **Initialize New Project**:
   ```sh
   npx gorig_gen_cli@latest init <project-name>
   # or with global install:
   gorig_gen_cli init <project-name>
   ```
   Automatically generates complete standard project layouts:
   - `_bin/` (`dev.yaml`, `local.yaml`, `prod.yaml`)
   - `_cmd/main.go` (program entry & bootstrap)
   - `api/init.go` (HTTP router auto-registration)
   - `domain/init.go` (DDD domain model auto-migration)
   - `cron/cron.go` (scheduled task configuration)
   - `global/config.go` (global configurations)
   - `go.mod` (Go 1.23 with `github.com/WnJee/gorig@latest`)

2. **Create New Business Domain Module**:
   ```sh
   npx gorig_gen_cli@latest create <module-name>
   # or with global install:
   gorig_gen_cli create <module-name>
   ```
   Automatically creates the DDD 4-tier module files and injects blank imports into `api/init.go` and `domain/init.go`:
   - `api/<module>/controller.go` — HTTP controller with generic `apix.BindReq`
   - `api/<module>/router.go` — Route group definition & registration
   - `domain/<module>/dto.go` — Request, Response, and Filter DTO structs
   - `domain/<module>/model.go` — Entity model with `DConfig()` & `AutoMigrate()`
   - `domain/<module>/service.go` — Domain business logic powered by `dx` fluent ORM

3. **Generate OpenAPI Docs & Interactive ReDoc Preview**:
   ```sh
   npx gorig_gen_cli@latest doc
   # or for a specific module:
   npx gorig_gen_cli@latest doc <module-name>
   ```

---

### 1.2 4-Tier Layered Architecture

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

### Standard Module Directory Structure
```text
my-app/
├── _bin/                         # Multi-environment YAML configs
│   ├── dev.yaml
│   ├── local.yaml
│   └── prod.yaml
├── _cmd/
│   └── main.go                   # Main entry point
├── api/                          # 【HTTP Interface Layer】
│   ├── init.go                   # HTTP service router registry (auto-imports api modules)
│   └── user/
│       ├── controller.go         # Controller (apix.BindReq generic parsing & responses)
│       └── router.go             # Gin route group definition
├── cron/
│   └── cron.go                   # Cron job registry
├── domain/                       # 【DDD Domain Layer】
│   ├── init.go                   # Domain model registry (auto-imports domain modules)
│   └── user/
│       ├── dto.go                # Req / Resp / Filter DTOs
│       ├── model.go              # Database model entity & DConfig & AutoMigrate
│       └── service.go            # Pure business logic & dx ORM operations
├── global/
│   └── config.go                 # App-level config variables
└── go.mod
```

---

## 2. Core Package Reference & Implementation Standards

### 2.1 `apix` — Request Binding, Validation & Responses

#### Generic Request Binding (`apix.BindReq[T]`)
Always prefer `apix.BindReq[T](c)` over manual parameter parsing. It automatically extracts parameters across JSON body, Query strings, Route parameters, and Form data, and validates binding tags:

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

// 200 OK paginated response
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
    ID       int64   `gorm:"column:id;primaryKey" json:"id"`
    Username string  `gorm:"column:username;type:varchar(64);uniqueIndex;not null" json:"username"`
    Email    string  `gorm:"column:email;type:varchar(128);index" json:"email"`
    Status   int     `gorm:"column:status;default:1" json:"status"`
}

// DConfig binds storage engine type, database name ("main"), and table name
func (u *User) DConfig() (domainx.ConType, string, string) {
    return domainx.Mysql, "main", "users"
}

func init() {
    domainx.AutoMigrate(func() domainx.ConTable {
        return dx.On[User](context.Background()).Complex()
    })
}
```

#### Querying & CRUD Operations
```go
// 1. Query multiple records with conditions
users, err := dx.On[User](ctx).
    Eq("status", 1).
    In("id", []int64{101, 102, 103}).
    Sort("id", false). // false = DESC, true = ASC
    FindData()

// 2. Query first matching record
user, err := dx.On[User](ctx).Eq("email", email).FirstData()

// 3. Create record (automatic Snowflake ID generation)
newUser := User{Username: "alice", Email: "alice@example.com"}
id, err := dx.On(ctx, &newUser).Save()

// 4. Update single column or map
err := dx.On[User](ctx).WithID(id).Update("status", 2)
err := dx.On[User](ctx).WithID(id).Updates(map[string]any{
    "username": "alice_updated",
    "status":   1,
})

// 5. Delete
err := dx.On[User](ctx).WithID(id).Delete()

// 6. Pagination
pagedResp, err := dx.On[User](ctx).
    Eq("status", 1).
    Sort("id", false).
    PageData(page, pageSize)

// 7. Atomic Database Transactions
err := domainx.Transaction(ctx, func(txCtx context.Context) error {
    if err := dx.On[Account](txCtx).WithID(fromID).Update("balance", fromBalance - amount); err != nil {
        return err
    }
    if err := dx.On[Account](txCtx).WithID(toID).Update("balance", toBalance + amount); err != nil {
        return err
    }
    return nil
})
```

---

### 2.3 `cache` — Multi-Level Caching & Anti-Stampede

Supports `Memory`, `Redis`, `Sqlite`, and `Json` cache drivers with unified APIs.

#### Anti-Stampede Singleflight (`Remember`)
Use `cache.Remember` for cache-aside patterns. It leverages singleflight to prevent cache penetration, avalanche, and breakdown:
```go
func (s *UserService) GetUserCached(ctx context.Context, userID int64) (*UserDTO, error) {
    cacheKey := fmt.Sprintf("user:info:%d", userID)
    
    return cache.Remember[UserDTO](ctx, cacheKey, 30*time.Minute, func() (UserDTO, error) {
        user, err := dx.On[model.User](ctx).WithID(userID).FirstData()
        if err != nil {
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
```

---

### 2.5 `cronx` — Distributed & Local Scheduler

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

// Layered Configuration (snake_case in YAML)
port := configure.GetInt("api.rest.addr")
sysName := configure.GetString("sys.name", "APP")

// Password Hashing & Encryption
hash, err := encrypt.BcryptHash("user_password")
valid := encrypt.BcryptVerify("user_password", hash)
encryptedText, err := encrypt.AESEncrypt("sensitive_token", aesKey)

// Business Error Definition
return errors.Verify("Account balance insufficient")
```

---

## 3. AI Agent Quality Standards & Execution Rules

1. **Scaffolding & Module Creation Priority (`gorig_gen_cli` 优先原则)**:
   - When initializing a project or creating a new module, **always prioritize running `gorig_gen_cli` commands** (`npx gorig_gen_cli@latest init <project>` / `npx gorig_gen_cli@latest create <module>`) over manual boilerplate assembly.
2. **Clean Code & Modern Go Idioms**:
   - Always use generics (`apix.BindReq[T]`, `cache.Remember[T]`, `dx.On[T]`, `messagex.SubscribeEvent[T]`).
   - Do not write legacy shims, fallback compatibility branches, or redundant dirty data logic.
3. **Configuration Standard**:
   - YAML configuration definitions must follow **lowercase snake_case** (`mysql.main.write.host`, `mysql.main.gorm_init`, `mysql.main.slow_threshold`, `mysql.main.write.max_idle_conns`, `mysql.main.write.max_open_conns`, `mysql.main.write.conn_max_lifetime`).
   - `dbname` must use the unexported constant `defaultDBName = "main"`, avoiding hardcoded exported constants.
4. **No Test File Persistence Rule (严禁持久保留测试文件)**:
   - Do NOT commit or leave temporary `*_test.go` files unless explicitly requested by the user.
   - Verify code using `go fmt ./...`, `go vet ./...`, and `go build ./...`.
5. **Layer Separation Guarantee**:
   - Controllers only handle HTTP translation (`apix.BindReq`, calling service, returning `apix.Ok`/`apix.Err`).
   - Business logic, caching, transactions, and event emissions stay strictly in Services.
   - Database queries stay in Services using `dx.On[T](ctx)`.
