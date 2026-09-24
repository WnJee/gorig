# utils 常用工具库使用指南 (Utilities Guide & Demos)

`utils` 包含 Gorig 框架底层常用的配置读取、结构化日志、加密解密、业务错误模型与报警通知工具。

---

## 目录
- [1. 错误模型 (errors.Error)](#1-错误模型-errorserror)
- [2. 结构化日志 (logger)](#2-结构化日志-logger)
- [3. 配置读取 (cofigure)](#3-配置读取-cofigure)
- [4. 加密与解密 (encrypt)](#4-加密与解密-encrypt)
- [5. 多渠道告警通知 (notify)](#5-多渠道告警通知-notify)

---

## 1. 错误模型 (errors.Error)

支持业务校验错误 (`Verify`)、系统内部错误 (`Sys`) 与断言错误 (`Assert`)，可携带原生底包错误并附带链路 TraceID：

```go
import "github.com/WnJee/gorig/utils/errors"

// 1. 业务校验/应用层错误 (Application Error, 对应 HTTP 400 系列)
if len(password) < 6 {
    return errors.Verify("Password length must be at least 6 characters")
}

// 2. 携带业务状态码
if !user.IsActive {
    return errors.VerifyCode(consts.CurdLoginFailCode, "Account is disabled")
}

// 3. 系统底层错误 (System Error, 对应 HTTP 500 系列，自动触发报警)
dbConn, err := sql.Open(...)
if err != nil {
    return errors.Sys("Failed to connect database", err)
}
```

---

## 2. 结构化日志 (logger)

自动在日志中携带 Context 链路 TraceID，支持分级日志与轮转：

```go
import "github.com/WnJee/gorig/utils/logger"
import "go.uber.org/zap"

// 记录带 TraceID 与结构化字段的日志
logger.Info(ctx, "User login success",
    zap.String("user_id", "u1001"),
    zap.String("client_ip", clientIP),
)

logger.Warn(ctx, "Rate limit warning", zap.Int("current_qps", 500))
logger.Error(ctx, "Failed to call payment API", zap.Error(err))
```

---

## 3. 配置读取 (cofigure)

支持分层读取 YAML 配置文件及环境变量覆盖：

```go
import configure "github.com/WnJee/gorig/utils/cofigure"

// 读取带默认值的字符串、布尔、数字与切片
port := configure.GetString("api.port", ":8080")
enableDebug := configure.GetBool("app.debug", false)
maxRetry := configure.GetInt("queue.max_retry", 3)
origins := configure.GetStringSlice("api.cors.origins")
```

---

## 4. 加密与解密 (encrypt)

提供 MD5、AES、RSA、Base64 与密码哈希工具：

```go
import "github.com/WnJee/gorig/utils/encrypt"

// 1. MD5 哈希
hash := encrypt.MD5("secret_string")

// 2. 密码哈希与比对 (Bcrypt)
hashedPassword, _ := encrypt.BcryptHash("user_password")
isValid := encrypt.BcryptCompare("user_password", hashedPassword)

// 3. AES-GCM 安全对称加解密
encrypted, err := encrypt.AesGcmEncrypt([]byte("sensitive_data"), []byte("32_bytes_aes_key..."))
plain, err := encrypt.AesGcmDecrypt(encrypted, []byte("32_bytes_aes_key..."))
```

---

## 5. 多渠道告警通知 (notify)

```go
import "github.com/WnJee/gorig/utils/notify/dingding"
import "github.com/WnJee/gorig/utils/notify/feishu"
import "github.com/WnJee/gorig/utils/notify/wecom"

// 钉钉机器人告警
go dingding.ErrNotifyDefault("Service memory usage exceeds 85%")

// 飞书机器人告警
go feishu.SendText("https://open.feishu.cn/open-apis/bot/v2/hook/...", "Deployment finished")

// 企业微信机器人告警
go wecom.SendText("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...", "Order surge detected")
```
