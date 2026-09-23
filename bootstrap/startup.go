package bootstrap

import (
	"github.com/gin-gonic/gin"
	_ "github.com/WnJee/gorig/cache"
	_ "github.com/WnJee/gorig/domainx"
	_ "github.com/WnJee/gorig/global/variable"
	"github.com/WnJee/gorig/httpx"
	"github.com/WnJee/gorig/serv"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/sys"
)

func regWebService() {
	err := serv.RegisterService(
		serv.Service{
			Code:     "HTTP",
			PORT:     configure.GetString("api.rest.addr", ":9617"),
			Startup:  httpx.Startup,
			Shutdown: httpx.Shutdown,
		},
	)
	if err != nil {
		sys.Exit(err)
	}
}

func StartUp() {
	regWebService()
	sys.Warn("# All registered API information ...... #")
	httpx.DumpRouters(func(info gin.RouteInfo) {
		sys.Info(" * ", info.Method, ": ", info.Path)
	})

	sys.Success("# All registered API information [OK] #")
	serv.Running()
}
