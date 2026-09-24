package apix

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/WnJee/gorig/apix/load"
	"github.com/WnJee/gorig/apix/response"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/WnJee/gorig/utils/notify/dingding"
	"github.com/gin-gonic/gin"
)

// PanicHandler is a callback hook invoked when a panic occurs.
type PanicHandler func(ctx *gin.Context, err any, stack string)

// ErrorHandler is a callback hook invoked when a system error occurs.
type ErrorHandler func(ctx *gin.Context, err error, stack string)

var (
	panicHooks []PanicHandler
	errorHooks []ErrorHandler
	hookMu     sync.RWMutex
)

// RegisterPanicHook adds a custom panic notification hook.
func RegisterPanicHook(h PanicHandler) {
	if h == nil {
		return
	}
	hookMu.Lock()
	defer hookMu.Unlock()
	panicHooks = append(panicHooks, h)
}

// RegisterErrorHook adds a custom system error notification hook.
func RegisterErrorHook(h ErrorHandler) {
	if h == nil {
		return
	}
	hookMu.Lock()
	defer hookMu.Unlock()
	errorHooks = append(errorHooks, h)
}

// HandlePanic recovers from panics, logs stack traces, and renders 500 JSON response.
func HandlePanic(ctx *gin.Context) {
	if r := recover(); r != nil {
		PanicNotify(ctx, r)
		traceID := GetTraceID(ctx)
		response.ErrorSystem(ctx, traceID, traceID)
	}
}

// PanicNotify logs and sends panic alert notifications.
func PanicNotify(ctx *gin.Context, err interface{}) {
	if err == nil {
		return
	}
	traceID := GetTraceID(ctx)
	stack := string(debug.Stack())
	logMsg := fmt.Sprintf("TraceID: %s,\nPanic: %v,\nRequest: %v,\nStack: %s", traceID, err, ctx.Request, stack)

	logger.DPanic(ctx, logMsg)
	go dingding.PanicNotifyDefault(logMsg)

	hookMu.RLock()
	hooks := make([]PanicHandler, len(panicHooks))
	copy(hooks, panicHooks)
	hookMu.RUnlock()

	for _, hook := range hooks {
		func(h PanicHandler) {
			defer func() { _ = recover() }()
			h(ctx, err, stack)
		}(hook)
	}
}

// toAppError normalizes any error into *errors.Error.
func toAppError(err error) *errors.Error {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*errors.Error); ok {
		return appErr
	}
	return errors.Verify(err.Error(), err)
}

// HandleError handles errors, distinguishes between System & Application errors, and renders the JSON response.
func HandleError(ctx *gin.Context, code int, data any, err error) {
	if err == nil {
		return
	}

	appErr := toAppError(err)
	traceID := GetTraceID(ctx)

	if appErr.Type == errors.System {
		logger.Warn(ctx, appErr.Error())
		response.ErrorSystem(ctx, traceID, traceID)

		stack := string(debug.Stack())
		logMsg := fmt.Sprintf("TraceID: %s,\nError: %v,\nRequest: %v,\nStack: %s", traceID, appErr, ctx.Request, stack)
		go dingding.ErrNotifyDefault(logMsg)

		hookMu.RLock()
		hooks := make([]ErrorHandler, len(errorHooks))
		copy(hooks, errorHooks)
		hookMu.RUnlock()

		for _, hook := range hooks {
			func(h ErrorHandler) {
				defer func() { _ = recover() }()
				h(ctx, appErr, stack)
			}(hook)
		}
		return
	}

	// Application error
	logger.Error(ctx, appErr.Error())
	response.Fail(ctx, code, appErr.Message, data)
}

// Handle renders success (S) when err is nil, or handles the error when non-nil.
func Handle(ctx *gin.Context, code int, err error) {
	if err != nil {
		HandleError(ctx, code, nil, err)
	} else {
		response.S(ctx)
	}
}

// HandleOk renders standard success response.
func HandleOk(ctx *gin.Context) {
	response.S(ctx)
}

// HandleData renders success with data when err is nil, or handles the error when non-nil.
func HandleData(ctx *gin.Context, code int, data interface{}, err error) {
	if err != nil {
		if appErr, ok := err.(*errors.Error); ok && appErr.Code != "" && appErr.CodeInt() != 0 {
			code = appErr.CodeInt()
		}
		HandleError(ctx, code, data, err)
	} else {
		response.Success(ctx, "", data)
	}
}

// HandleResult is a generic helper for standard handler returns.
func HandleResult[T any](ctx *gin.Context, code int, data T, err error) {
	HandleData(ctx, code, data, err)
}

// HandlePage renders paginated response or handles error.
func HandlePage[T any](ctx *gin.Context, code int, pageResp *load.PageRespT[T], err error) {
	if err != nil {
		HandleData(ctx, code, nil, err)
		return
	}
	if pageResp == nil {
		pageResp = load.EmptyPageRespT[T]()
	}
	response.Success(ctx, "", pageResp)
}

// SendError logs error and dispatches notifications.
func SendError(ctx *gin.Context, code int, message string, err ...error) {
	var targetErr *errors.Error
	if len(err) > 0 && err[0] != nil {
		targetErr = toAppError(err[0])
	} else {
		targetErr = errors.Verify(message)
	}

	traceID := GetTraceID(ctx)
	stack := string(debug.Stack())
	logMsg := fmt.Sprintf("TraceID: %s,\nError: %v,\nRequest: %v,\nStack: %s", traceID, targetErr, ctx.Request, stack)

	logger.Error(ctx, targetErr.Error())
	go dingding.ErrNotifyDefault(logMsg)
}
