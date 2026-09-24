package response

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/WnJee/gorig/global/consts"
	"github.com/WnJee/gorig/global/errc"
	"github.com/WnJee/gorig/utils/logger"
)

const Tocamel = "toCamel"

// Response represents a standard JSON API response structure.
type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func snakeToCamel(s string) string {
	if !strings.Contains(s, "_") {
		return s
	}
	var builder strings.Builder
	builder.Grow(len(s))
	toUpper := false
	for i, r := range s {
		if r == '_' {
			toUpper = true
			continue
		}
		if toUpper {
			builder.WriteRune(unicode.ToUpper(r))
			toUpper = false
		} else if i == 0 {
			builder.WriteRune(unicode.ToLower(r))
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func convertKeysToCamel(val any) any {
	switch v := val.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, item := range v {
			result[snakeToCamel(k)] = convertKeysToCamel(item)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = convertKeysToCamel(item)
		}
		return result
	default:
		return val
	}
}

func toCamel(g interface{}) any {
	if g == nil {
		return nil
	}
	b, err := json.Marshal(g)
	if err != nil {
		logger.Error(nil, "toCamel marshal failed: "+err.Error())
		return g
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return g
	}
	return convertKeysToCamel(raw)
}

// SetToCamel marks context to convert response data keys to camelCase.
func SetToCamel(c *gin.Context) {
	c.Set(Tocamel, true)
}

// GetToCamel returns whether context has toCamel enabled.
func GetToCamel(c *gin.Context) bool {
	if v, exists := c.Get(Tocamel); exists {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// ReturnJson renders a standard JSON response and aborts the Gin context.
func ReturnJson(c *gin.Context, httpCode int, dataCode int, msg string, data interface{}) {
	if c.Writer.Written() {
		return
	}
	result := gin.H{
		"code": dataCode,
		"msg":  msg,
	}
	if GetToCamel(c) {
		result["data"] = toCamel(data)
	} else {
		result["data"] = data
	}
	c.JSON(httpCode, result)
	c.Abort()
}

// ReturnJsonFromString outputs raw JSON string with specified HTTP status.
func ReturnJsonFromString(c *gin.Context, httpCode int, jsonStr string) {
	if c.Writer.Written() {
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.String(httpCode, jsonStr)
	c.Abort()
}

// S returns standard success response with code 200 and default success message.
func S(c *gin.Context) {
	ReturnJson(c, http.StatusOK, consts.CurdStatusOkCode, consts.CurdStatusOkMsg, nil)
}

// Ok is an alias for S.
func Ok(c *gin.Context) {
	S(c)
}

// OkData returns success response with data.
func OkData(c *gin.Context, data any) {
	Success(c, consts.CurdStatusOkMsg, data)
}

// OkMsg returns success response with custom message.
func OkMsg(c *gin.Context, msg string) {
	Success(c, msg, nil)
}

// OkDataMsg returns success response with data and custom message.
func OkDataMsg(c *gin.Context, data any, msg string) {
	Success(c, msg, data)
}

// Success returns success response with custom message and data.
func Success(c *gin.Context, msg string, data interface{}) {
	if msg == "" {
		msg = consts.CurdStatusOkMsg
	}
	ReturnJson(c, http.StatusOK, consts.CurdStatusOkCode, msg, data)
}

// Fail returns failure response with status 400, custom business code, message, and optional data.
func Fail(c *gin.Context, dataCode int, msg string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	ReturnJson(c, http.StatusBadRequest, dataCode, msg, d)
}

// FailMsg returns failure response with default error code and given message.
func FailMsg(c *gin.Context, msg string) {
	Fail(c, consts.CurdSelectFailCode, msg, nil)
}

// FailCode returns failure response with custom code and message.
func FailCode(c *gin.Context, code int, msg string) {
	Fail(c, code, msg, nil)
}

// FailErr returns failure response from a standard error.
func FailErr(c *gin.Context, err error) {
	if err == nil {
		S(c)
		return
	}
	Fail(c, consts.CurdSelectFailCode, err.Error(), nil)
}

// Unauthorized returns 401 Unauthorized response.
func Unauthorized(c *gin.Context, msg ...string) {
	message := errc.ErrorsNoAuthorization
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	ReturnJson(c, http.StatusUnauthorized, http.StatusUnauthorized, message, nil)
}

// Forbidden returns 403 Forbidden response.
func Forbidden(c *gin.Context, msg ...string) {
	message := errc.ErrorsTokenPermissionDenied
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	ReturnJson(c, http.StatusForbidden, http.StatusForbidden, message, nil)
}

// NotFound returns 404 Not Found response.
func NotFound(c *gin.Context, msg ...string) {
	message := "Resource not found"
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	ReturnJson(c, http.StatusNotFound, http.StatusNotFound, message, nil)
}

// ServerError returns 500 Internal Server Error response.
func ServerError(c *gin.Context, msg ...string) {
	message := consts.ServerOccurredErrorMsg
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	ReturnJson(c, http.StatusInternalServerError, consts.ServerOccurredErrorCode, message, nil)
}

// TooManyRequests returns 429 Too Many Requests response.
func TooManyRequests(c *gin.Context, msg ...string) {
	message := http.StatusText(http.StatusTooManyRequests)
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	ReturnJson(c, http.StatusTooManyRequests, http.StatusTooManyRequests, message, nil)
}

// ErrorTokenBaseInfo token format 400
func ErrorTokenBaseInfo(c *gin.Context) {
	ReturnJson(c, http.StatusBadRequest, http.StatusBadRequest, errc.ErrorsTokenBaseInfo, nil)
}

// ErrorTokenAuthFail token auth 401
func ErrorTokenAuthFail(c *gin.Context) {
	Unauthorized(c)
}

// ErrorForbidden token 403
func ErrorForbidden(c *gin.Context) {
	Forbidden(c)
}

// ErrorServiceForbidden service 403
func ErrorServiceForbidden(c *gin.Context) {
	ReturnJson(c, http.StatusForbidden, http.StatusForbidden, errc.ErrorsServicePermissionDenied, nil)
}

// ErrorTokenRefreshFail token refresh 401
func ErrorTokenRefreshFail(c *gin.Context) {
	ReturnJson(c, http.StatusUnauthorized, http.StatusUnauthorized, errc.ErrorsRefreshTokenFail, nil)
}

// TokenErrorParam params 403
func TokenErrorParam(c *gin.Context, msg string) {
	ReturnJson(c, http.StatusForbidden, consts.ValidatorParamsCheckFailCode, msg, nil)
}

// ErrorCasbinAuthFail casbin 405
func ErrorCasbinAuthFail(c *gin.Context, msg interface{}) {
	ReturnJson(c, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, errc.ErrorsCasbinNoAuthorization, msg)
}

// ErrorParam parameter validation fail 400
func ErrorParam(c *gin.Context, wrongParam interface{}) {
	ReturnJson(c, http.StatusBadRequest, consts.ValidatorParamsCheckFailCode, consts.ValidatorParamsCheckFailMsg, wrongParam)
}

// ErrorSystem 500 system error
func ErrorSystem(c *gin.Context, msg string, data interface{}) {
	errMsg := consts.ServerOccurredErrorMsg
	if msg != "" {
		errMsg = consts.ServerOccurredErrorMsg + " " + msg
	}
	ReturnJson(c, http.StatusInternalServerError, consts.ServerOccurredErrorCode, errMsg, data)
}

// ErrorTooManyRequests 429
func ErrorTooManyRequests(c *gin.Context) {
	TooManyRequests(c)
}

// ValidatorError 400 validation error
func ValidatorError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	logger.Error(c, "ValidatorError: "+err.Error())
	ReturnJson(c, http.StatusBadRequest, consts.ValidatorParamsCheckFailCode, err.Error(), nil)
}
