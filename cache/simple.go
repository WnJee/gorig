package cache

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

var defaultCaches []Cache[any]

// SimpleTool returns a multi-level cache tool (L1 In-Memory + L2 Redis if available).
var SimpleTool = func(ctx *gin.Context) *Tool[any] {
	return NewCacheTool[any](ctx, defaultCaches, nil)
}

func init() {
	l1Cache := NewGoCache[any](10*time.Minute, 5*time.Minute)
	defaultCaches = []Cache[any]{l1Cache}

	l2Cache := GetRedisInstance[any](context.Background())
	if l2Cache != nil {
		defaultCaches = append(defaultCaches, l2Cache)
	}
}
