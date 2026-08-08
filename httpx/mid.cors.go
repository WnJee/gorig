package httpx

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
)

const (
	AllowMethods = "GET, POST, PUT, DELETE, OPTIONS"
	AllowHeaders = "" +
		"Origin, " +
		"Content-Type, " +
		"Content-Length, " +
		"Accept-Encoding, " +
		"X-CSRF-Token, " +
		"X-Request-ID, " +
		"Authorization, "
)

var otherAllowHeaders = []string{}
var allowedOrigins = map[string]struct{}{}
var corsMu sync.RWMutex

func SetOtherAllowHeaders(headers ...string) {
	corsMu.Lock()
	defer corsMu.Unlock()
	otherAllowHeaders = headers
}

// SetAllowedOrigins configures exact origins. Passing "*" allows public,
// non-credentialed cross-origin requests.
func SetAllowedOrigins(origins ...string) {
	corsMu.Lock()
	defer corsMu.Unlock()
	allowedOrigins = make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if origin != "" {
			allowedOrigins[origin] = struct{}{}
		}
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		corsMu.RLock()
		_, exactAllowed := allowedOrigins[origin]
		_, publicAllowed := allowedOrigins["*"]
		extraHeaders := append([]string(nil), otherAllowHeaders...)
		corsMu.RUnlock()
		if origin != "" && exactAllowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin != "" && publicAllowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Methods", AllowMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", AllowHeaders+"cache-control")
		for _, h := range extraHeaders {
			c.Writer.Header().Add("Access-Control-Allow-Headers", h)
		}
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			if origin != "" && !exactAllowed && !publicAllowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
