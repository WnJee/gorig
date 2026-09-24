# httpx 使用指南与场景示例 (HTTP & SSE Guide & Demos)

`httpx` 是 Gorig 框架的 HTTP 服务核心套件，集成了 Gin 引擎管理、安全中间件（Panic 恢复、链路日志、防重复提交、CORS、签名认证）、泛型 HTTP Client 工具以及 SSE（Server-Sent Events）实时流式推送。

---

## 目录
- [1. 路由注册与服务启动 (RegisterRouter / Startup)](#1-路由注册与服务启动-registerrouter--startup)
- [2. 中间件生态 (Middleware)](#2-中间件生态-middleware)
- [3. 泛型 HTTP 客户端 (GetJSON / Post)](#3-泛型-http-客户端-getjson--post)
- [4. SSE 流式传输与心跳 (ssex.Streamer)](#4-sse-流式传输与心跳-ssexstreamer)

---

## 1. 路由注册与服务启动 (RegisterRouter / Startup)

```go
package main

import (
    "github.com/WnJee/gorig/httpx"
    "github.com/gin-gonic/gin"
)

func init() {
    // 注册全局业务路由
    httpx.RegisterRouter(func(r *gin.RouterGroup) {
        v1 := r.Group("/api/v1")
        {
            v1.GET("/users", ListUsers)
            v1.POST("/users", CreateUser)
        }
    })
}
```

框架已内置标准探针接口：
- `GET /ping`：返回服务时间戳；
- `GET /healthz`：返回服务健康状态、运行环境及启动模式。

---

## 2. 中间件生态 (Middleware)

`httpx` 预置了工业级中间件：
- **`Recovery()`**：自动捕获全局 Panic 并发送结构化告警；
- **`Logger()`**：记录全链路请求方法、路由、耗时、状态码与客户端 IP；
- **`CORS()`**：跨域安全支持（通过 `api.cors.origins` 或 `api.cors.allowAll` 配置）；
- **`Debounce(duration)`**：防高频重复提交（默认防抖 200ms）；
- **`SignVerify()`**：API 访问签名及 Token 校验。

```go
// 针对特定路由组应用签名认证中间件
httpx.RegisterRouter(func(r *gin.RouterGroup) {
    admin := r.Group("/admin", httpx.SignVerify())
    {
        admin.GET("/dashboard", GetDashboard)
    }
})
```

---

## 3. 泛型 HTTP 客户端 (GetJSON / Post)

内置连接池与重试支持，直接将三方 API 响应反序列化为泛型结构体：

```go
type WeatherResp struct {
    City string  `json:"city"`
    Temp float64 `json:"temp"`
}

// 1. 发送 GET 请求并解析 JSON
weather, err := httpx.GetJSON[WeatherResp](
    "https://api.weather.com/v1",
    map[string]string{"city": "Beijing", "key": "secret_key"},
)

// 2. 发送 POST JSON 请求
type OrderNotifyReq struct {
    OrderID string `json:"order_id"`
    Status  string `json:"status"`
}
type ThirdPartyResult struct {
    Success bool `json:"success"`
}

result, err := httpx.Post[ThirdPartyResult](
    "https://partner.com/notify",
    OrderNotifyReq{OrderID: "ORD100", Status: "PAID"},
    map[string]string{"Authorization": "Bearer token"},
)
```

---

## 4. SSE 流式传输与心跳 (ssex.Streamer)

用于 AI 对话大模型流式输出（如 ChatGPT / DeepSeek 逐字流式返回）或实时事件推送：

```go
import "github.com/WnJee/gorig/httpx/ssex"

func StreamAIResponse(ctx *gin.Context) {
    // 1. 使用 SSE 中间件建立流式长连接
    ssex.Mid()(ctx)

    stream := ssex.NewStreamer(ctx)

    // 2. 开启后台定时心跳（每 15 秒 ping 一次，防止客户端或网关超时断开）
    stopHeartbeat := stream.Heartbeat(15 * time.Second)
    defer stopHeartbeat()

    // 3. 模拟大模型逐字推送
    tokens := []string{"Hello", ", ", "welcome", " to ", "Gorig", " AI", " system!"}
    for _, token := range tokens {
        select {
        case <-ctx.Request.Context().Done():
            // 客户端主动断开连接
            return
        default:
            _ = stream.Send("message", gin.H{"delta": token})
            time.Sleep(100 * time.Millisecond)
        }
    }

    // 4. 推送完成事件
    _ = stream.SendOK("done", gin.H{"finished": true})
}
```
