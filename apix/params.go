package apix

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/WnJee/gorig/apix/load"
	"github.com/WnJee/gorig/apix/response"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

const (
	ErrorKey  = "error_g"
	paramsKey = "params"
)

// Forcible indicates whether a missing parameter triggers a validation error response.
type Forcible bool

const (
	Force    Forcible = true
	NotForce Forcible = false
)

type RequestType string

const (
	Get      RequestType = "Get"
	PostForm RequestType = "PostForm"
	PostBody RequestType = "PostBody"
)

// ParamType defines supported primitive constraint types for generic parameter extraction.
type ParamType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string | ~bool | ~uintptr
}

// PutParams caches parsed parameters in Gin context.
func PutParams(ctx *gin.Context, params map[string]interface{}) {
	if ctx == nil || ctx.IsAborted() {
		return
	}
	ctx.Set(paramsKey, params)
}

// GetParams extracts and merges all request parameters (URL query, form, and JSON body).
func GetParams(ctx *gin.Context, requestType ...RequestType) map[string]interface{} {
	if ctx == nil || ctx.IsAborted() {
		return nil
	}
	if v, exists := ctx.Get(paramsKey); exists && v != nil {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}

	req := make(map[string]interface{})

	// 1. Path (URI) Route Parameters
	for _, p := range ctx.Params {
		req[p.Key] = p.Value
	}

	// 2. URL Query Parameters
	if ctx.Request != nil && ctx.Request.URL != nil {
		for k, v := range ctx.Request.URL.Query() {
			if len(v) == 1 {
				req[k] = v[0]
			} else if len(v) > 1 {
				req[k] = v
			}
		}
	}

	if ctx.Request == nil || ctx.Request.ContentLength == 0 {
		if len(req) > 0 {
			ctx.Set(paramsKey, req)
			return req
		}
		return req
	}

	var rt = PostBody
	if len(requestType) > 0 {
		rt = requestType[0]
	}
	if rt == Get {
		if len(req) > 0 {
			ctx.Set(paramsKey, req)
		}
		return req
	}

	contentType := ctx.Request.Header.Get("Content-Type")
	isForm := strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data")

	if rt == PostForm || (ctx.Request.Method == "POST" || ctx.Request.Method == "PUT" || ctx.Request.Method == "PATCH") && isForm {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.Set(ErrorKey, fmt.Sprintf("GetParams parseForm error: %v", err))
			logger.Error(ctx, "GetParams form parse error", zap.Error(err))
			response.ValidatorError(ctx, err)
			return nil
		}
		for k, v := range ctx.Request.PostForm {
			if len(v) == 1 {
				req[k] = v[0]
			} else if len(v) > 1 {
				req[k] = v
			}
		}
		ctx.Set(paramsKey, req)
		return req
	}

	// JSON Body binding
	var bodyMap map[string]interface{}
	if err := ctx.ShouldBind(&bodyMap); err == nil && bodyMap != nil {
		for k, v := range bodyMap {
			req[k] = v
		}
	}

	ctx.Set(paramsKey, req)
	return req
}

// Bind parses and validates request payload into a target struct pointer.
func Bind(ctx *gin.Context, req interface{}) (err *errors.Error) {
	return BindParams(ctx, req, false)
}

// BindParams binds and validates request data into req struct pointer.
func BindParams(ctx *gin.Context, req interface{}, noPrint ...bool) (err *errors.Error) {
	if ctx == nil || ctx.IsAborted() {
		return errors.Verify("BindParams: ctx is aborted or nil")
	}

	if v, exists := ctx.Get(paramsKey); exists && v != nil {
		params := cast.ToStringMap(v)
		jsonStr, err := json.Marshal(params)
		if err != nil {
			ctx.Set(ErrorKey, fmt.Sprintf("BindParams marshal error: %v", err))
			logger.Error(ctx, "BindParams json marshal error", zap.Error(err))
			response.ValidatorError(ctx, err)
			return errors.Verify(fmt.Sprintf("BindParams: %v", err))
		}
		if err := json.Unmarshal(jsonStr, req); err != nil {
			ctx.Set(ErrorKey, fmt.Sprintf("BindParams unmarshal error: %v", err))
			logger.Error(ctx, "BindParams json unmarshal error", zap.Error(err))
			response.ValidatorError(ctx, err)
			return errors.Verify(fmt.Sprintf("BindParams: %v", err))
		}
	} else {
		if err := ctx.ShouldBind(req); err != nil {
			ctx.Set(ErrorKey, fmt.Sprintf("BindParams bind error: %v", err))
			logger.Error(ctx, "BindParams ShouldBind error", zap.Error(err))
			response.ValidatorError(ctx, err)
			return errors.Verify(fmt.Sprintf("BindParams: %v", err))
		}
	}

	if vErr := ValidateStruct(req); vErr != nil {
		ctx.Set(ErrorKey, vErr.Error())
		logger.Error(ctx, "BindParams validation", zap.Any("err", vErr))
		response.ValidatorError(ctx, vErr)
		return vErr
	}

	if len(noPrint) == 0 || !noPrint[0] {
		logger.Info(ctx, "BindParams success", zap.Any("req", req))
	}
	return nil
}

// BindReq is a generic constructor and binder: allocates a new struct T, binds, validates, and returns *T.
func BindReq[T any](ctx *gin.Context, noPrint ...bool) (*T, *errors.Error) {
	req := new(T)
	if err := BindParams(ctx, req, noPrint...); err != nil {
		return nil, err
	}
	return req, nil
}

// BindQuery binds URL query parameters into a new struct *T.
func BindQuery[T any](ctx *gin.Context) (*T, *errors.Error) {
	if ctx == nil || ctx.IsAborted() {
		return nil, errors.Verify("BindQuery: ctx is nil or aborted")
	}
	req := new(T)
	if err := ctx.ShouldBindQuery(req); err != nil {
		ctx.Set(ErrorKey, err.Error())
		response.ValidatorError(ctx, err)
		return nil, errors.Verify(err.Error())
	}
	if vErr := ValidateStruct(req); vErr != nil {
		ctx.Set(ErrorKey, vErr.Error())
		response.ValidatorError(ctx, vErr)
		return nil, vErr
	}
	return req, nil
}

// BindJSON binds JSON body into a new struct *T.
func BindJSON[T any](ctx *gin.Context) (*T, *errors.Error) {
	if ctx == nil || ctx.IsAborted() {
		return nil, errors.Verify("BindJSON: ctx is nil or aborted")
	}
	req := new(T)
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.Set(ErrorKey, err.Error())
		response.ValidatorError(ctx, err)
		return nil, errors.Verify(err.Error())
	}
	if vErr := ValidateStruct(req); vErr != nil {
		ctx.Set(ErrorKey, vErr.Error())
		response.ValidatorError(ctx, vErr)
		return nil, vErr
	}
	return req, nil
}

// BindURI binds URI route parameters into a new struct *T.
func BindURI[T any](ctx *gin.Context) (*T, *errors.Error) {
	if ctx == nil || ctx.IsAborted() {
		return nil, errors.Verify("BindURI: ctx is nil or aborted")
	}
	req := new(T)
	if err := ctx.ShouldBindUri(req); err != nil {
		ctx.Set(ErrorKey, err.Error())
		response.ValidatorError(ctx, err)
		return nil, errors.Verify(err.Error())
	}
	if vErr := ValidateStruct(req); vErr != nil {
		ctx.Set(ErrorKey, vErr.Error())
		response.ValidatorError(ctx, vErr)
		return nil, vErr
	}
	return req, nil
}

// convertValue converts any raw input to target type T.
func convertValue[T any](raw any) (T, error) {
	var zero T
	if raw == nil {
		return zero, fmt.Errorf("value is nil")
	}

	switch any(zero).(type) {
	case string:
		return any(cast.ToString(raw)).(T), nil
	case int:
		v, err := cast.ToIntE(raw)
		return any(v).(T), err
	case int64:
		v, err := cast.ToInt64E(raw)
		return any(v).(T), err
	case int32:
		v, err := cast.ToInt32E(raw)
		return any(v).(T), err
	case int16:
		v, err := cast.ToInt16E(raw)
		return any(v).(T), err
	case int8:
		v, err := cast.ToInt8E(raw)
		return any(v).(T), err
	case uint:
		v, err := cast.ToUintE(raw)
		return any(v).(T), err
	case uint64:
		v, err := cast.ToUint64E(raw)
		return any(v).(T), err
	case uint32:
		v, err := cast.ToUint32E(raw)
		return any(v).(T), err
	case uint16:
		v, err := cast.ToUint16E(raw)
		return any(v).(T), err
	case uint8:
		v, err := cast.ToUint8E(raw)
		return any(v).(T), err
	case float64:
		v, err := cast.ToFloat64E(raw)
		return any(v).(T), err
	case float32:
		v, err := cast.ToFloat32E(raw)
		return any(v).(T), err
	case bool:
		v, err := cast.ToBoolE(raw)
		return any(v).(T), err
	case time.Duration:
		v, err := cast.ToDurationE(raw)
		return any(v).(T), err
	case time.Time:
		v, err := cast.ToTimeE(raw)
		return any(v).(T), err
	default:
		if val, ok := raw.(T); ok {
			return val, nil
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return zero, err
		}
		var target T
		if err := json.Unmarshal(b, &target); err != nil {
			return zero, err
		}
		return target, nil
	}
}

// convertSlice converts a raw parameter (slice, comma-separated string, or JSON array) to []T.
func convertSlice[T any](raw any) ([]T, error) {
	if raw == nil {
		return []T{}, nil
	}

	if typed, ok := raw.([]T); ok {
		return typed, nil
	}

	var elements []any
	switch v := raw.(type) {
	case string:
		str := strings.TrimSpace(v)
		if str == "" || str == "[]" {
			return []T{}, nil
		}
		if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
			var unmarshaled []any
			if err := json.Unmarshal([]byte(str), &unmarshaled); err == nil {
				elements = unmarshaled
				break
			}
		}
		// Split by comma
		for _, part := range strings.Split(str, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				elements = append(elements, p)
			}
		}
	case []string:
		for _, s := range v {
			elements = append(elements, s)
		}
	case []int:
		for _, s := range v {
			elements = append(elements, s)
		}
	case []int64:
		for _, s := range v {
			elements = append(elements, s)
		}
	case []any:
		elements = v
	default:
		val := reflect.ValueOf(raw)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			for i := 0; i < val.Len(); i++ {
				elements = append(elements, val.Index(i).Interface())
			}
		} else {
			elements = append(elements, raw)
		}
	}

	result := make([]T, 0, len(elements))
	for _, item := range elements {
		converted, err := convertValue[T](item)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

// Param retrieves a parameter by key converted to type T with optional default value.
func Param[T any](ctx *gin.Context, key string, defValue ...T) (T, *errors.Error) {
	return extractParam[T](ctx, key, NotForce, defValue...)
}

// ParamReq retrieves a required parameter by key converted to type T, triggering validation error if missing.
func ParamReq[T any](ctx *gin.Context, key string) (T, *errors.Error) {
	return extractParam[T](ctx, key, Force)
}

// ParamSlice retrieves a slice parameter by key with optional default.
func ParamSlice[T any](ctx *gin.Context, key string, defValue ...[]T) ([]T, *errors.Error) {
	return extractParamSlice[T](ctx, key, NotForce, defValue...)
}

// ParamSliceReq retrieves a required slice parameter by key, triggering validation error if missing.
func ParamSliceReq[T any](ctx *gin.Context, key string) ([]T, *errors.Error) {
	return extractParamSlice[T](ctx, key, Force)
}

func extractParam[T any](ctx *gin.Context, key string, force Forcible, defValue ...T) (T, *errors.Error) {
	var zero T
	if len(defValue) > 0 {
		zero = defValue[0]
	}

	if ctx == nil {
		if force {
			return zero, errors.Verify("Param: ctx is nil")
		}
		return zero, nil
	}

	if ctx.IsAborted() && ctx.GetString(ErrorKey) != "" {
		return zero, errors.Verify(ctx.GetString(ErrorKey))
	}

	// 1. Check Path Route Param first
	if val := ctx.Param(key); val != "" {
		converted, err := convertValue[T](val)
		if err == nil {
			return converted, nil
		}
		errText := fmt.Sprintf("param '%s' type error: %v", key, err)
		if force {
			ctx.Set(ErrorKey, errText)
			response.ValidatorError(ctx, errors.Verify(errText))
			return zero, errors.Verify(errText)
		}
		return zero, nil
	}

	// 2. Check merged params map
	params := GetParams(ctx)
	if raw, exists := params[key]; exists && raw != nil {
		if s, ok := raw.(string); ok && (s == "undefined" || s == "null") {
			return zero, nil
		}
		converted, err := convertValue[T](raw)
		if err == nil {
			return converted, nil
		}
		errText := fmt.Sprintf("param '%s' type error: %v", key, err)
		if force {
			ctx.Set(ErrorKey, errText)
			response.ValidatorError(ctx, errors.Verify(errText))
			return zero, errors.Verify(errText)
		}
		return zero, nil
	}

	if force {
		errText := fmt.Sprintf("param '%s' is required", key)
		ctx.Set(ErrorKey, errText)
		response.ValidatorError(ctx, errors.Verify(errText))
		return zero, errors.Verify(errText)
	}

	return zero, nil
}

func extractParamSlice[T any](ctx *gin.Context, key string, force Forcible, defValue ...[]T) ([]T, *errors.Error) {
	var zero []T
	if len(defValue) > 0 {
		zero = defValue[0]
	}

	if ctx == nil {
		if force {
			return zero, errors.Verify("ParamSlice: ctx is nil")
		}
		return zero, nil
	}

	if ctx.IsAborted() && ctx.GetString(ErrorKey) != "" {
		return zero, errors.Verify(ctx.GetString(ErrorKey))
	}

	params := GetParams(ctx)
	if raw, exists := params[key]; exists && raw != nil {
		converted, err := convertSlice[T](raw)
		if err == nil && len(converted) > 0 {
			return converted, nil
		}
		if err != nil {
			errText := fmt.Sprintf("param '%s' slice conversion error: %v", key, err)
			if force {
				ctx.Set(ErrorKey, errText)
				response.ValidatorError(ctx, errors.Verify(errText))
				return zero, errors.Verify(errText)
			}
			return zero, nil
		}
	}

	if force {
		errText := fmt.Sprintf("param '%s' is required", key)
		ctx.Set(ErrorKey, errText)
		response.ValidatorError(ctx, errors.Verify(errText))
		return zero, errors.Verify(errText)
	}

	return zero, nil
}

// GetParamType retrieves a parameter conforming to ParamType.
func GetParamType[t ParamType](ctx *gin.Context, key string, force Forcible, defValue ...t) (value t, err *errors.Error) {
	return extractParam[t](ctx, key, force, defValue...)
}

// GetParamArray retrieves an array parameter.
func GetParamArray[t any](ctx *gin.Context, key string, force Forcible, defValue ...[]t) (value []t, err *errors.Error) {
	return extractParamSlice[t](ctx, key, force, defValue...)
}

// GetParamInt retrieves an int parameter.
func GetParamInt(ctx *gin.Context, key string, force Forcible, defValue ...int) (value int, err *errors.Error) {
	return extractParam[int](ctx, key, force, defValue...)
}

// GetParamInt64 retrieves an int64 parameter.
func GetParamInt64(ctx *gin.Context, key string, force Forcible, defValue ...int64) (value int64, err *errors.Error) {
	return extractParam[int64](ctx, key, force, defValue...)
}

// GetParamBool retrieves a bool parameter.
func GetParamBool(ctx *gin.Context, key string, force Forcible, defValue ...bool) (value bool, err *errors.Error) {
	return extractParam[bool](ctx, key, force, defValue...)
}

// GetParamFloat64 retrieves a float64 parameter.
func GetParamFloat64(ctx *gin.Context, key string, force Forcible, defValue ...float64) (value float64, err *errors.Error) {
	return extractParam[float64](ctx, key, force, defValue...)
}

// GetParamInt64Slice retrieves an []int64 parameter (supporting comma separated strings).
func GetParamInt64Slice(ctx *gin.Context, key string, force Forcible, defValue ...[]int64) (value []int64, err *errors.Error) {
	return extractParamSlice[int64](ctx, key, force, defValue...)
}

// GetParamStr retrieves a string parameter.
func GetParamStr(ctx *gin.Context, key string, defValue ...string) (value string, err *errors.Error) {
	var def string
	if len(defValue) > 0 {
		def = defValue[0]
	}
	return extractParam[string](ctx, key, NotForce, def)
}

// GetParamForce retrieves a required string parameter.
func GetParamForce(ctx *gin.Context, key string) (value string, err *errors.Error) {
	return extractParam[string](ctx, key, Force)
}

// GetParam retrieves a raw parameter.
func GetParam(ctx *gin.Context, key string, defValue ...string) (value interface{}, err *errors.Error) {
	if ctx == nil {
		return nil, nil
	}
	if ctx.IsAborted() && ctx.GetString(ErrorKey) != "" {
		return "", errors.Verify(ctx.GetString(ErrorKey))
	}
	if val := ctx.Param(key); val != "" {
		return val, nil
	}
	params := GetParams(ctx)
	if params != nil {
		if v, ok := params[key]; ok {
			return v, nil
		}
	}
	if len(defValue) > 0 {
		return defValue[0], nil
	}
	return nil, nil
}

// GetPageReq parses standard pagination parameters (page, size, lastID) from request.
func GetPageReq(ctx *gin.Context) (pageReq *load.Page, err *errors.Error) {
	page, err := GetParamInt64(ctx, "page", NotForce, load.DefaultPageNum)
	if err != nil {
		return nil, err
	}
	size, err := GetParamInt64(ctx, "size", NotForce, load.DefaultPageSize)
	if err != nil {
		return nil, err
	}
	lastID, err := GetParamInt64(ctx, "lastID", NotForce, 0)
	if err != nil {
		return nil, err
	}
	return load.NewPage(page, size, lastID), nil
}

// ValueExtractor provides a fluent interface for extracting single parameter values.
type ValueExtractor struct {
	ctx *gin.Context
	key string
	val any
}

// Value creates a fluent ValueExtractor for a parameter key.
func Value(ctx *gin.Context, key string) ValueExtractor {
	v, _ := GetParam(ctx, key)
	return ValueExtractor{
		ctx: ctx,
		key: key,
		val: v,
	}
}

// String returns parameter as string or fallback default.
func (e ValueExtractor) String(def ...string) string {
	if e.val == nil {
		if len(def) > 0 {
			return def[0]
		}
		return ""
	}
	return cast.ToString(e.val)
}

// Int returns parameter as int or fallback default.
func (e ValueExtractor) Int(def ...int) int {
	if e.val == nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return cast.ToInt(e.val)
}

// Int64 returns parameter as int64 or fallback default.
func (e ValueExtractor) Int64(def ...int64) int64 {
	if e.val == nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return cast.ToInt64(e.val)
}

// Bool returns parameter as bool or fallback default.
func (e ValueExtractor) Bool(def ...bool) bool {
	if e.val == nil {
		if len(def) > 0 {
			return def[0]
		}
		return false
	}
	return cast.ToBool(e.val)
}

// Float64 returns parameter as float64 or fallback default.
func (e ValueExtractor) Float64(def ...float64) float64 {
	if e.val == nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return cast.ToFloat64(e.val)
}
