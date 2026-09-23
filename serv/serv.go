package serv

import (
	"context"
	"fmt"
	_ "github.com/WnJee/gorig/cache"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/WnJee/gorig/utils/sys"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

type Service struct {
	Code     string
	PORT     string
	Startup  func(code, port string) error
	Shutdown func(code string, ctx context.Context) error
}

var (
	gServicesMu sync.RWMutex
	gServices   = make(map[string]Service)
	// gServiceOrder preserves registration order so startup and shutdown are
	// deterministic. Map iteration order in Go is randomized by design.
	gServiceOrder []string
)

func doRegisterService(service Service) *errors.Error {
	gServicesMu.Lock()
	defer gServicesMu.Unlock()
	if _, exists := gServices[service.Code]; exists {
		return errors.Sys(fmt.Sprintf("The same service has been register.[ code=%s ]", service.Code))
	}
	gServices[service.Code] = service
	gServiceOrder = append(gServiceOrder, service.Code)
	return nil
}

func RegisterService(service ...Service) *errors.Error {
	if len(service) == 0 {
		return errors.Sys("no any service")
	}
	for _, s := range service {
		err := doRegisterService(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func StartCode(code string) *errors.Error {
	gServicesMu.RLock()
	service, exists := gServices[code]
	gServicesMu.RUnlock()
	if !exists {
		return errors.Sys(fmt.Sprintf("The service not found.[ code=%s ]", code))
	}
	if service.Startup == nil {
		return errors.Sys(fmt.Sprintf("The service startup function is nil.[ code=%s ]", code))
	}
	err := service.Startup(code, service.PORT)
	if err != nil {
		return errors.Sys(fmt.Sprintf("The service startup error.[ code=%s ]: %v", code, err))
	}
	return nil
}

func Running() {
	gServicesMu.RLock()
	order := append([]string(nil), gServiceOrder...)
	services := make(map[string]Service, len(gServices))
	for code, s := range gServices {
		services[code] = s
	}
	gServicesMu.RUnlock()

	for _, code := range order {
		service := services[code]
		err := service.Startup(code, service.PORT)
		if err != nil {
			logger.Logger.Error("start server failed", zap.String("code", code), zap.Error(err))
			sys.Error("# Start service exception: ", code, " #")
			return
		}
		sys.Success("# Start the service ", code, " [OK] #")
	}

	sys.Info("# ALL Used Configure Items #")
	configure.Dump(func(key string, val any) {
		if strings.Index(strings.ToLower(key), "pass") > -1 || strings.Index(strings.ToLower(key), "secret") > -1 || strings.Index(strings.ToLower(key), "key") > -1 {
			sys.Info("  # ", key, " # ==>> ", "**********")
		} else {
			sys.Info("  # ", key, " # ==>> ", val)
		}

	})

	sys.Success("# System startup successful #")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	sys.Info("# Shutting down the system ...... #")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := len(order) - 1; i >= 0; i-- {
		code := order[i]
		service := services[code]
		sys.Info(" * Start stop service: ", code, " ......")
		err := service.Shutdown(code, ctx)
		if err != nil {
			logger.Logger.Error("shutdown server failed", zap.String("code", code), zap.Error(err))
			sys.Error(" * Stop service ", code, " exception")
		}
		sys.Success(" * Stop service ", code, " [OK]")
	}
	sys.Success("# Shutting down the system [OK] #")
}
