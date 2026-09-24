# tokenx 使用指南与场景示例 (Authentication & JWT Guide & Demos)

`tokenx` 是 Gorig 框架的身份令牌管理套件，支持 JWT 签发与校验、黑名单管理、Token 刷新以及内存/Redis 双重存储后端。

---

## 目录
- [1. 签发 Token (GenerateToken)](#1-签发-token-generatetoken)
- [2. 解析与验证 Token (ParseToken)](#2-解析与验证-token-parsetoken)
- [3. 销毁与注销 Token (DestroyToken)](#3-销毁与注销-token-destroytoken)
- [4. 在 Gin 路由中保护接口](#4-在-gin-路由中保护接口)

---

## 1. 签发 Token (GenerateToken)

用户登录成功后，签发带有自定义用户信息的 JWT Token：

```go
import "github.com/WnJee/gorig/mid/tokenx"

func Login(ctx *gin.Context) {
    defer apix.HandlePanic(ctx)

    // 校验账号密码成功后...
    userInfo := map[string]interface{}{
        "role":     "admin",
        "username": "john_doe",
    }

    // 签发 7 天有效期的 Token
    tokenStr, err := tokenx.GenerateToken("user_10001", userInfo, 7*24*time.Hour)
    if err != nil {
        response.FailMsg(ctx, "Failed to issue token")
        return
    }

    response.OkData(ctx, gin.H{
        "token":  tokenStr,
        "expire": 7 * 24 * 3600,
    })
}
```

---

## 2. 解析与验证 Token (ParseToken)

```go
claims, err := tokenx.ParseToken(tokenStr)
if err != nil {
    // Token 已过期或签名不合法
}

fmt.Println("UserID:", claims.UserId)
fmt.Println("Role:", claims.UserInfo["role"])
```

---

## 3. 销毁与注销 Token (DestroyToken)

用户主动注销登录（Logout）时，将 Token 加入黑名单使其立即失效：

```go
func Logout(ctx *gin.Context) {
    token := apix.GetBearerToken(ctx)
    if token != "" {
        tokenx.DestroyToken(token)
    }
    response.OkMsg(ctx, "Logged out successfully")
}
```

---

## 4. 在 Gin 路由中保护接口

结合 `httpx.SignVerify()` 中间件，未登录或 Token 无效的请求会自动返回 401 Unauthorized：

```go
httpx.RegisterRouter(func(r *gin.RouterGroup) {
    // 开放接口
    auth := r.Group("/auth")
    {
        auth.POST("/login", Login)
    }

    // 鉴权保护接口
    protected := r.Group("/api", httpx.SignVerify())
    {
        protected.GET("/profile", func(c *gin.Context) {
            userID := apix.GetUserID(c)
            response.OkData(c, gin.H{"userID": userID})
        })
    }
})
```
