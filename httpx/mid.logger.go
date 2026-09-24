package httpx

import (
	"github.com/WnJee/gorig/apix"
	"github.com/WnJee/gorig/global/consts"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strings"
)

var restLogger = logger.GetLogger("rest")

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer restLogger.Sync()
		apix.SetTraceID(c)
		restLogger.Info("IN", doGetArrForIn(c)...)

		c.Next()

		restLogger.Info("OUT", doGetArrForOut(c)...)
	}
}

func doGetArrForIn(c *gin.Context) []zap.Field {
	return []zap.Field{
		zap.String(consts.TraceIDKey, apix.GetTraceID(c)),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("remoteAddr", c.Request.RemoteAddr),
		zap.Any("header", sanitizedHeaders(c.Request.Header)),
		zap.Any("query", sanitizedQuery(c.Request.URL.Query())),
	}
}

func sanitizedHeaders(input http.Header) http.Header {
	result := input.Clone()
	for _, key := range []string{"Authorization", "X-Api-Key", "Token", "X-Token", "Access-Token"} {
		if val := result.Get(key); val != "" {
			result.Set(key, logger.MaskToken(val))
		}
	}
	for _, key := range []string{"Cookie", "Set-Cookie"} {
		if val := result.Get(key); val != "" {
			result.Set(key, "[REDACTED]")
		}
	}
	return result
}

func sanitizedQuery(input map[string][]string) map[string][]string {
	result := make(map[string][]string, len(input))
	for key, values := range input {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "ticket") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") {
			masked := make([]string, len(values))
			for i, v := range values {
				masked[i] = logger.MaskToken(v)
			}
			result[key] = masked
			continue
		}
		if strings.Contains(lower, "password") || strings.Contains(lower, "pwd") {
			result[key] = []string{"******"}
			continue
		}
		result[key] = append([]string(nil), values...)
	}
	return result
}

func doGetArrForOut(c *gin.Context) []zap.Field {
	return []zap.Field{
		zap.String(consts.TraceIDKey, apix.GetTraceID(c)),
		zap.Int("status", c.Writer.Status()),
	}
}
