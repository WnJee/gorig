# apix 使用指南与场景示例 (API Extensions Guide & Demos)

`apix` 是 Gorig 框架中用于 Web API 请求解析、参数校验、统一响应、分页封装与异常处理的核心开发套件。

---

## 目录
- [1. 泛型参数绑定与校验 (BindReq)](#1-泛型参数绑定与校验-bindreq)
- [2. 灵活参数提取 (Param / ParamReq / ParamSlice)](#2-灵活参数提取-param--paramreq--paramslice)
- [3. 流式参数读取 (Value)](#3-流式参数读取-value)
- [4. 统一 HTTP 响应封装 (response)](#4-统一-http-响应封装-response)
- [5. 统一错误与 Panic 处理 (handle)](#5-统一错误与-panic-处理-handle)
- [6. 分页请求与响应 (load.Page / PageRespT)](#6-分页请求与响应-loadpage--pagerespt)
- [7. 请求上下文与链路追踪工具 (tool)](#7-请求上下文与链路追踪工具-tool)
- [8. 自定义与内置校验器 (validate)](#8-自定义与内置校验器-validate)

---

## 1. 泛型参数绑定与校验 (BindReq)

在 Controller 中使用 `apix.BindReq[T](ctx)` 可实现一步完成内存分配、参数绑定（支持 JSON / Form / Query）和结构体验证：

```go
type CreateUserReq struct {
    Username string `json:"username" binding:"required,min=3,max=32"`
    Email    string `json:"email" binding:"required,email"`
    Phone    string `json:"phone" binding:"omitempty,phone"` // 内置手机号校验
    Age      int    `json:"age" binding:"min=1,max=150"`
}

func CreateUser(ctx *gin.Context) {
    defer apix.HandlePanic(ctx)

    // 一行代码完成结构体实例化、解析与校验；失败自动输出 400 校验错误
    req, err := apix.BindReq[CreateUserReq](ctx)
    if err != nil {
        return
    }

    // 执行业务逻辑
    user, createErr := userService.Create(ctx, req)
    apix.HandleData(ctx, consts.CurdCreatFailCode, user, createErr)
}
```

---

## 2. 灵活参数提取 (Param / ParamReq / ParamSlice)

无需定义结构体，即可从 Route 参数 (`/:id`)、Query 参数、Form 参数或 JSON Body 中提取强类型值：

```go
func GetUserDetail(ctx *gin.Context) {
    defer apix.HandlePanic(ctx)

    // 1. 必填参数提取（缺失或类型不匹配时自动返回 400 参数错误）
    id, err := apix.ParamReq[int64](ctx, "id")
    if err != nil {
        return
    }

    // 2. 选填参数带默认值
    viewMode, _ := apix.Param[string](ctx, "mode", "simple")

    // 3. 数组/切片提取（自动支持逗号分隔如 ?tags=go,k8s,gin 或 JSON 数组）
    tags, _ := apix.ParamSlice[string](ctx, "tags")

    result, fetchErr := userService.Find(ctx, id, viewMode, tags)
    apix.HandleData(ctx, consts.CurdSelectFailCode, result, fetchErr)
}
```

---

## 3. 流式参数读取 (Value)

对于快速简易参数提取，可使用 `apix.Value` 链式提取器：

```go
func QueryStats(ctx *gin.Context) {
    defer apix.HandlePanic(ctx)

    page := apix.Value(ctx, "page").Int64(1)
    size := apix.Value(ctx, "size").Int64(20)
    keyword := apix.Value(ctx, "keyword").String()
    isExport := apix.Value(ctx, "export").Bool(false)

    // ...
}
```

---

## 4. 统一 HTTP 响应封装 (response)

`response` 包提供标准规范的 JSON API 响应方法，并自动处理 Context 生命周期 (`c.Abort()`)：

```go
import "github.com/WnJee/gorig/apix/response"

// 成功响应
response.S(ctx)                                   // { "code": 200, "msg": "Success", "data": null }
response.OkData(ctx, data)                       // { "code": 200, "msg": "Success", "data": {...} }
response.OkMsg(ctx, "Operation succeeded")        // { "code": 200, "msg": "Operation succeeded", "data": null }
response.OkDataMsg(ctx, data, "Custom message")  // { "code": 200, "msg": "Custom message", "data": {...} }

// 错误与语义状态响应
response.FailMsg(ctx, "Invalid username or password")
response.FailCode(ctx, -400101, "Token expired")
response.Unauthorized(ctx, "Please login first") // HTTP 401
response.Forbidden(ctx, "Permission denied")     // HTTP 403
response.NotFound(ctx, "User not found")         // HTTP 404
response.ServerError(ctx, "Internal DB error")   // HTTP 500
response.TooManyRequests(ctx)                    // HTTP 429
```

---

## 5. 统一错误与 Panic 处理 (handle)

在 Controller 顶部声明 `defer apix.HandlePanic(ctx)`，可自动捕获未处理的 Panic、打印调用栈、输出 500 响应并触发报警通知：

```go
func UpdateArticle(ctx *gin.Context) {
    defer apix.HandlePanic(ctx) // 自动捕获 Panic 与全链路报警

    req, err := apix.BindReq[UpdateArticleReq](ctx)
    if err != nil {
        return
    }

    // HandleResult: err 为 nil 时返回 200 Success(data)，非 nil 时智能分类处理
    article, svcErr := articleService.Update(ctx, req)
    apix.HandleResult(ctx, consts.CurdUpdateFailCode, article, svcErr)
}
```

### 注册自定义告警钩子 (Alarm Hooks)
```go
apix.RegisterPanicHook(func(ctx *gin.Context, err any, stack string) {
    // 发送告警至飞书/企业微信/Sentry
    sentry.CaptureException(...)
})
```

---

## 6. 分页请求与响应 (load.Page / PageRespT)

```go
// 1. 从请求中自动读取 page, size, lastID 参数
pageReq, err := apix.GetPageReq(ctx)
if err != nil {
    return
}

// 2. 执行分页查询并返回
resp, err := dx.On[User](ctx).Eq("status", 1).PageData(pageReq.Page, pageReq.Size)
apix.HandlePage(ctx, consts.CurdSelectFailCode, resp, err)
```

---

## 7. 请求上下文与链路追踪工具 (tool)

```go
// 链路追踪 ID（自动提取 X-Trace-ID / X-Request-ID / 生成新 ID）
traceID := apix.GetTraceID(ctx)

// 用户登录身份获取
userID := apix.GetUserID(ctx)
userIDInt64 := apix.GetUserIDInt64(ctx)

// 用户登录信息反序列化为业务强类型
type CustomUser struct {
    Role   string   `json:"role"`
    Perms  []string `json:"perms"`
}
userMeta, err := apix.GetUserInfoAs[CustomUser](ctx)

// 获取客户端真实 IP
clientIP := apix.GetClientIP(ctx)

// 获取 Bearer Token
token := apix.GetBearerToken(ctx)
```

---

## 8. 自定义与内置校验器 (validate)

内置常用业务校验规则（支持 `binding` 和 `validate` 标签）：
- `phone` / `mobile`: 国内 11 位手机号；
- `idcard`: 18 位身份证号码；
- `json_str`: 合法 JSON 字符串；
- `semver`: 语义化版本号格式（如 `v1.2.3`）；
- `datetime` / `date` / `time`: 日期时间格式。

```go
// 单值快速验证
if err := apix.ValidateVar("13800138000", "required,phone"); err != nil {
    // 校验失败
}

// 全局注册自定义校验规则
apix.RegisterValidation("custom_code", func(fl validator.FieldLevel) bool {
    return strings.HasPrefix(fl.Field().String(), "CODE_")
})
```
