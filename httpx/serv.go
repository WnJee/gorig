package httpx

import (
	"context"
	"fmt"
	"github.com/WnJee/gorig/apix/response"
	_ "github.com/WnJee/gorig/domainx"
	"github.com/WnJee/gorig/global/consts"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/sys"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func IsRegistered() bool {
	serverMu.Lock()
	defer serverMu.Unlock()
	return gHttpServer != nil
}

func Startup(code, port string) error {
	serverMu.Lock()
	defer serverMu.Unlock()
	if gHttpServer != nil {
		sys.Info(" * Rest service already started")
		return nil
		//sys.Exit(errors.Sys("You should not start the rest service twice"))
	}
	server := &http.Server{
		Addr:              port,
		Handler:           gEngine,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	listener, err := net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", port, err)
	}
	gHttpServer = server
	sys.Info(" * Rest service startup on: ", server.Addr)
	go func() {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			sys.Error(" * rest service failed: ", err.Error())
		}
	}()

	return nil
}

func Shutdown(code string, ctx context.Context) error {
	serverMu.Lock()
	server := gHttpServer
	serverMu.Unlock()
	if server == nil {
		return nil
	}
	if err := server.Shutdown(ctx); err != nil {
		sys.Error(" * Rest service shutdown error: ", err.Error())
		return err
	}

	serverMu.Lock()
	if gHttpServer == server {
		gHttpServer = nil
	}
	serverMu.Unlock()
	sys.Info(" * Rest service stopped: ", server.Addr)
	return nil
}

// RegisterRouter 注册路由
func RegisterRouter(reg func(groupRouter *gin.RouterGroup)) {
	reg(&gEngine.RouterGroup)
}

func RegisterRouterMid(group func(groupRouter *gin.RouterGroup, mid ...gin.HandlerFunc) *gin.RouterGroup, mid ...gin.HandlerFunc) *gin.RouterGroup {
	var groupRouter *gin.RouterGroup
	RegisterRouter(func(groupRouter *gin.RouterGroup) {
		groupRouter = group(groupRouter, mid...)
	})
	return groupRouter
}

var gEngine = gin.New()
var gHttpServer *http.Server
var serverMu sync.Mutex

func init() {
	if !sys.RunMode.IsRd() {
		gin.SetMode(gin.ReleaseMode)
	}
	gEngine.Use(Recovery())
	gEngine.Use(Logger())
	allowedOrigins := configureAllowedOrigins()
	SetAllowedOrigins(allowedOrigins...)
	switch {
	case len(allowedOrigins) == 1 && allowedOrigins[0] == "*":
		sys.Warn(" * CORS: allow-all mode (api.cors.allowAll defaults to true): Access-Control-Allow-Origin: * without credentials; set api.cors.origins to restrict origins")
	case len(allowedOrigins) == 0:
		sys.Warn(" * CORS: no allowed origins configured (api.cors.origins); cross-origin browser requests will be rejected")
	}
	gEngine.Use(CORS())
	gEngine.Use(gzip.Gzip(gzip.BestSpeed))
	gEngine.Use(Debounce(200 * time.Millisecond))
	//gEngine.Use(IdemVerify())
	//gEngine.Use(SignVerify())
	RegisterRouter(func(groupRouter *gin.RouterGroup) {
		groupRouter.GET("ping", func(ctx *gin.Context) {
			response.Success(ctx, consts.CurdStatusOkMsg, fmt.Sprintf("timestamp %d", time.Now().UnixMilli()))
		})
	})
}

// configureAllowedOrigins resolves the CORS whitelist with precedence:
//  1. api.cors.origins (YAML list or comma-separated string) wins when non-empty;
//  2. otherwise api.cors.allowAll (default true) allows every origin via "*",
//     mirroring the historical open behavior. "*" is non-credentialed by design;
//     cookie-based frontends must switch to explicit origins instead.
//  3. otherwise the whitelist stays empty and browser cross-origin requests
//     are rejected.
func configureAllowedOrigins() []string {
	allowAll := configure.GetBool("api.cors.allowAll", true)
	return resolveCorsOrigins(allowAll, configure.GetStringSlice("api.cors.origins"))
}

func resolveCorsOrigins(allowAll bool, raw []string) []string {
	if len(raw) == 1 {
		raw = strings.Split(raw[0], ",")
	}
	origins := make([]string, 0, len(raw))
	for _, origin := range raw {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) > 0 {
		return origins
	}
	if allowAll {
		return []string{"*"}
	}
	return nil
}
