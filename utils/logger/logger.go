package logger

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/WnJee/gorig/global/consts"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/sys"
	"github.com/rs/xid"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"reflect"
	"strings"
)

type Level = string

const (
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
)

func LevelOf(level Level) zapcore.Level {
	zapLevel := zapcore.InfoLevel
	switch level {
	case DebugLevel:
		zapLevel = zapcore.DebugLevel
	case InfoLevel:
		zapLevel = zapcore.InfoLevel
	case WarnLevel:
		zapLevel = zapcore.WarnLevel
	case ErrorLevel:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.ErrorLevel
	}
	return zapLevel
}

func GetLogger(key string) *zap.Logger {
	rootPath := configure.GetString("logger."+key+".root", "./.logs/"+key+"/")
	logLevel := configure.GetString("logger."+key+".level", "debug")
	fileName := rootPath + configure.GetString("logger."+key+".file", key+".jsonl")
	maxSize := configure.GetInt("logger."+key+".size.max", 128)
	maxBackups := configure.GetInt("logger."+key+".backup.max", 60)
	maxAge := configure.GetInt("logger."+key+".age.max", 30)
	compress := configure.GetBool("logger."+key+".compress", false)
	sys.Warn("# Initialize the " + key + " log system ..... #")
	sys.Info(" * PATH: ", strings.ToUpper(rootPath), "      ${ logger."+key+".root }")
	sys.Info(" * LEVEL: ", strings.ToUpper(logLevel), "      ${ logger."+key+".level }")
	sys.Info(" * FILE: ", strings.ToUpper(fileName), "      ${ logger."+key+".file }")
	sys.Info(" * MAX SIZE: ", maxSize, "      ${ logger."+key+".size.max }")
	sys.Info(" * BACKUP MAX: ", maxBackups, "      ${ logger."+key+".backup.max }")
	sys.Info(" * AGE MAX: ", maxAge, "      ${ logger."+key+".age.max }")
	sys.Info(" * COMPRESS: ", compress, "      ${ logger."+key+".compress }")

	zapLogLevel := LevelOf(logLevel)

	hook := lumberjack.Logger{
		Filename:   fileName,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        key,
		CallerKey:      "line",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zapLogLevel)

	writeSyncers := []zapcore.WriteSyncer{
		zapcore.AddSync(&hook),
	}

	if sys.RunMode.IsRd() {
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(writeSyncers...),
		atomicLevel,
	)

	sys.Success("# Initialize the " + key + " log system [OK] #")

	return zap.New(core)
}

func isNilPointer(i any) bool {
	if i == nil {
		return true
	}
	val := reflect.ValueOf(i)
	return val.Kind() == reflect.Ptr && val.IsNil()
}

func GetTraceID(ctx context.Context) string {
	return cast.ToString(getTraceID(ctx))
}

func getTraceID(ctx context.Context) any {
	if ctx == nil {
		return "no traceid"
	}
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		return ginCtx.Value(consts.TraceIDKey)
	}
	if isNilPointer(ctx) {
		return "no-traceid"
	}
	return ctx.Value(consts.TraceIDKey)
}

func getUserID(ctx context.Context) any {
	if ctx == nil {
		return nil
	}
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		return ginCtx.Value(consts.UserIDKey)
	}
	if isNilPointer(ctx) {
		return nil
	}
	return ctx.Value(consts.UserIDKey)
}

func putTraceId(ctx context.Context, fields ...zap.Field) []zap.Field {
	defer func() {
		if r := recover(); r != nil {
			sys.Warn("putTraceId panic:", r)
		}
	}()
	if ctx != nil {
		if userID := getUserID(ctx); userID != nil {
			fields = append([]zap.Field{zap.Any(consts.UserIDKey, userID)}, fields...)
		}
	}
	return append([]zap.Field{zap.Any(consts.TraceIDKey, getTraceID(ctx))}, fields...)
}

func NewCtx(traceIds ...string) context.Context {
	ctx := context.Background()
	var traceId string
	if len(traceIds) > 0 {
		traceId = traceIds[0]
	} else {
		traceId = xid.New().String()
	}
	return context.WithValue(ctx, consts.TraceIDKey, traceId)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Info(msg, putTraceId(ctx, fields...)...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Warn(msg, putTraceId(ctx, fields...)...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Error(msg, putTraceId(ctx, fields...)...)
}

func DPanic(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.DPanic(msg, putTraceId(ctx, fields...)...)
}

func Panic(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Panic(msg, putTraceId(ctx, fields...)...)
}

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Fatal(msg, putTraceId(ctx, fields...)...)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	fields = insertLine(fields...)
	Logger.Debug(msg, putTraceId(ctx, fields...)...)
}

func insertLine(fields ...zap.Field) []zap.Field {
	return fields
}

// Infof writes a formatted info message with trace context.
func Infof(ctx context.Context, format string, args ...any) {
	Info(ctx, fmt.Sprintf(format, args...))
}

// Warnf writes a formatted warn message with trace context.
func Warnf(ctx context.Context, format string, args ...any) {
	Warn(ctx, fmt.Sprintf(format, args...))
}

// Errorf writes a formatted error message with trace context.
func Errorf(ctx context.Context, format string, args ...any) {
	Error(ctx, fmt.Sprintf(format, args...))
}

// Debugf writes a formatted debug message with trace context.
func Debugf(ctx context.Context, format string, args ...any) {
	Debug(ctx, fmt.Sprintf(format, args...))
}

// InfoKV logs an info message with key-value pairs.
func InfoKV(ctx context.Context, msg string, kvs ...any) {
	Info(ctx, msg, toZapFields(kvs...)...)
}

// WarnKV logs a warning message with key-value pairs.
func WarnKV(ctx context.Context, msg string, kvs ...any) {
	Warn(ctx, msg, toZapFields(kvs...)...)
}

// ErrorKV logs an error message with key-value pairs.
func ErrorKV(ctx context.Context, msg string, kvs ...any) {
	Error(ctx, msg, toZapFields(kvs...)...)
}

// DebugKV logs a debug message with key-value pairs.
func DebugKV(ctx context.Context, msg string, kvs ...any) {
	Debug(ctx, msg, toZapFields(kvs...)...)
}

// toZapFields converts key-value pairs into zap fields.
func toZapFields(kvs ...any) []zap.Field {
	if len(kvs) == 0 {
		return nil
	}
	fields := make([]zap.Field, 0, len(kvs)/2)
	for i := 0; i < len(kvs); i += 2 {
		key := cast.ToString(kvs[i])
		if i+1 < len(kvs) {
			fields = append(fields, zap.Any(key, kvs[i+1]))
		} else {
			fields = append(fields, zap.Any(key, nil))
		}
	}
	return fields
}

// WithContext returns a zap.Logger with the context's TraceID and UserID attached.
func WithContext(ctx context.Context) *zap.Logger {
	fields := putTraceId(ctx)
	return Logger.With(fields...)
}

// Sugar returns a sugared logger with context fields attached.
func Sugar(ctx context.Context) *zap.SugaredLogger {
	return WithContext(ctx).Sugar()
}

// With creates a child logger with the given zap fields.
func With(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}

// MaskString masks middle characters of a string, keeping prefix and suffix characters.
func MaskString(s string, prefixLen, suffixLen int) string {
	runes := []rune(s)
	total := len(runes)
	if total == 0 {
		return ""
	}
	if total <= prefixLen+suffixLen {
		if total <= 4 {
			return strings.Repeat("*", total)
		}
		return string(runes[:1]) + strings.Repeat("*", total-2) + string(runes[total-1:])
	}
	maskLen := total - prefixLen - suffixLen
	if maskLen > 6 {
		maskLen = 6
	}
	return string(runes[:prefixLen]) + strings.Repeat("*", maskLen) + string(runes[total-suffixLen:])
}

// MaskToken masks sensitive tokens (e.g., Bearer tokens, API keys, JWTs), keeping header and tail characters.
func MaskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		prefix := token[:7]
		raw := token[7:]
		return prefix + MaskString(raw, 6, 4)
	}
	return MaskString(token, 6, 4)
}

// MaskPhone masks phone numbers (e.g., 138****1234).
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return MaskString(phone, 2, 2)
	}
	return MaskString(phone, 3, 4)
}

// MaskEmail masks email addresses (e.g., a***b@example.com).
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	atIdx := strings.Index(email, "@")
	if atIdx <= 0 {
		return MaskString(email, 2, 2)
	}
	name := email[:atIdx]
	domain := email[atIdx:]
	if len(name) <= 2 {
		return string(name[0]) + "***" + domain
	}
	return string(name[0]) + "***" + string(name[len(name)-1]) + domain
}

var Logger *zap.Logger
var Console *zap.Logger

func init() {
	Logger = GetLogger("commons")
	Console = GetLogger("console")
	if !sys.RunMode.IsRd() {
		sys.ConsoleToLogger(func(msg string) {
			Console.Info(msg)
		})
	}
}
