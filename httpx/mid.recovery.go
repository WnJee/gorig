package httpx

import (
	"github.com/WnJee/gorig/apix"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err interface{}) {
		logger.Error(c, "gin recovery", zap.Any("err", err))
		apix.PanicNotify(c, err)
	})
}
