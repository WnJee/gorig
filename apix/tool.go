package apix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/WnJee/gorig/global/consts"
	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

// NewCtx creates an isolated gin.Context for unit testing or offline execution.
func NewCtx() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	SetTraceID(ctx)
	return ctx
}

// SetTraceID sets a new or given Trace ID into both Gin Context and standard Request Context.
func SetTraceID(ctx *gin.Context, customTraceID ...string) string {
	if ctx == nil {
		return ""
	}
	var traceID string
	if len(customTraceID) > 0 && customTraceID[0] != "" {
		traceID = customTraceID[0]
	} else {
		traceID = xid.New().String()
	}
	ctx.Set(consts.TraceIDKey, traceID)
	if ctx.Request != nil {
		newCtx := context.WithValue(ctx.Request.Context(), consts.TraceIDKey, traceID)
		ctx.Request = ctx.Request.WithContext(newCtx)
	}
	return traceID
}

// GetTraceID extracts the Trace ID from Gin Context, Request Context, or HTTP headers.
func GetTraceID(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	if val := ctx.GetString(consts.TraceIDKey); val != "" {
		return val
	}
	if ctx.Request != nil {
		if val, ok := ctx.Request.Context().Value(consts.TraceIDKey).(string); ok && val != "" {
			return val
		}
		if val := ctx.GetHeader("X-Trace-ID"); val != "" {
			return val
		}
		if val := ctx.GetHeader("X-Request-ID"); val != "" {
			return val
		}
		if val := ctx.GetHeader("Traceparent"); val != "" {
			return val
		}
	}
	return ""
}

// EnsureTraceID retrieves the trace ID or generates and sets a new one if missing.
func EnsureTraceID(ctx *gin.Context) string {
	tid := GetTraceID(ctx)
	if tid == "" {
		tid = SetTraceID(ctx)
	}
	return tid
}

// GetUserID returns the User ID string from context.
func GetUserID(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	if uid := ctx.GetString(consts.UserID); uid != "" {
		return uid
	}
	if ctx.Request != nil {
		if uid, ok := ctx.Request.Context().Value(consts.UserIDKey).(string); ok {
			return uid
		}
	}
	return ""
}

// GetUserIDInt returns the User ID as int.
func GetUserIDInt(ctx *gin.Context) int {
	userID := GetUserID(ctx)
	if userID == "" {
		return 0
	}
	id, _ := strconv.Atoi(userID)
	return id
}

// GetUserIDInt64 returns the User ID as int64.
func GetUserIDInt64(ctx *gin.Context) int64 {
	userID := GetUserID(ctx)
	if userID == "" {
		return 0
	}
	id, _ := strconv.ParseInt(userID, 10, 64)
	return id
}

// SetUserID sets the User ID into both Gin Context and Request Context.
func SetUserID(ctx *gin.Context, userID string) {
	if ctx == nil {
		return
	}
	ctx.Set(consts.UserID, userID)
	if ctx.Request != nil {
		newCtx := context.WithValue(ctx.Request.Context(), consts.UserIDKey, userID)
		ctx.Request = ctx.Request.WithContext(newCtx)
	}
}

// GetUserInfo returns the UserInfo map from context.
func GetUserInfo(ctx *gin.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	value, exists := ctx.Get(consts.UserInfo)
	if !exists {
		return nil
	}
	userInfo, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return userInfo
}

// GetUserInfoAs converts user info into a typed struct pointer.
func GetUserInfoAs[T any](ctx *gin.Context) (*T, error) {
	if ctx == nil {
		return nil, nil
	}
	val, exists := ctx.Get(consts.UserInfo)
	if !exists || val == nil {
		return nil, nil
	}
	if typed, ok := val.(*T); ok {
		return typed, nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	result := new(T)
	if err := json.Unmarshal(b, result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetUserInfo sets the UserInfo map into context.
func SetUserInfo(ctx *gin.Context, userInfo map[string]interface{}) {
	if ctx != nil {
		ctx.Set(consts.UserInfo, userInfo)
	}
}

// GetUserInfoValue gets a specific field from UserInfo map.
func GetUserInfoValue(ctx *gin.Context, key string) any {
	userInfo := GetUserInfo(ctx)
	if userInfo == nil {
		return nil
	}
	return userInfo[key]
}

// GetClientIP gets client IP address checking headers with fallback.
func GetClientIP(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return ""
	}
	clientIP := ctx.GetHeader("X-Real-IP")
	if clientIP == "" {
		clientIP = ctx.GetHeader("X-Forwarded-For")
		if clientIP != "" {
			parts := strings.Split(clientIP, ",")
			if len(parts) > 0 {
				clientIP = strings.TrimSpace(parts[0])
			}
		}
	}
	if clientIP == "" {
		clientIP = ctx.ClientIP()
	}
	return clientIP
}

// GetHeader safely retrieves a header value with optional default.
func GetHeader(ctx *gin.Context, key string, defValue ...string) string {
	if ctx == nil || ctx.Request == nil {
		if len(defValue) > 0 {
			return defValue[0]
		}
		return ""
	}
	val := ctx.GetHeader(key)
	if val == "" && len(defValue) > 0 {
		return defValue[0]
	}
	return val
}

// GetBearerToken extracts Bearer token from Authorization header or 'token' query/header parameter.
func GetBearerToken(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return ""
	}
	auth := ctx.GetHeader("Authorization")
	if auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return strings.TrimSpace(auth)
	}
	if token := ctx.GetHeader(consts.TokenKey); token != "" {
		return token
	}
	return ctx.Query(consts.TokenKey)
}

// GetContextVal safely retrieves a typed value from Gin Context.
func GetContextVal[T any](ctx *gin.Context, key string) (val T, exists bool) {
	if ctx == nil {
		return val, false
	}
	v, exists := ctx.Get(key)
	if !exists || v == nil {
		return val, false
	}
	if typed, ok := v.(T); ok {
		return typed, true
	}
	return val, false
}

// SetContextVal sets a typed value into Gin Context.
func SetContextVal[T any](ctx *gin.Context, key string, val T) {
	if ctx != nil {
		ctx.Set(key, val)
	}
}

// GetHost returns the request host.
func GetHost(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return ""
	}
	if host := ctx.GetHeader("X-Forwarded-Host"); host != "" {
		return host
	}
	return ctx.Request.Host
}

// GetScheme returns http or https based on connection/proxy headers.
func GetScheme(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return "http"
	}
	if scheme := ctx.GetHeader("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	if ctx.Request.TLS != nil {
		return "https"
	}
	return "http"
}

// GetFullURL returns the full request URL string including scheme and host.
func GetFullURL(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return ""
	}
	return GetScheme(ctx) + "://" + GetHost(ctx) + ctx.Request.RequestURI
}
