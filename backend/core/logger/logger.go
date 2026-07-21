package logger

import (
	"context"
	"os"
	"strings"

	"github.com/yi-nology/zentao-max/backend/core/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger
var sugarLogger *zap.SugaredLogger

type ContextKey string

const (
	TraceIDKey ContextKey = "trace_id"
)

func Init(cfg *config.LogConfig) error {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	level := getZapLevel(cfg.Level)

	core := zapcore.NewCore(
		encoder,
		getOutputWriter(cfg.OutputPath),
		level,
	)

	options := []zap.Option{}
	if cfg.EnableCaller {
		options = append(options, zap.AddCaller())
		options = append(options, zap.AddCallerSkip(1))
	}
	if cfg.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// 用 Hook 把每条日志同时追加到进程内环形缓冲（供日志页消费）
	options = append(options, zap.Hooks(func(ent zapcore.Entry) error {
		entry := &LogEntry{
			Time:    ent.Time.Format("2006-01-02T15:04:05.000Z07:00"),
			Level:   ent.Level.String(),
			Message: ent.Message,
			Caller:  ent.Caller.TrimmedPath(),
		}
		GetRingBuffer().Append(entry)
		return nil
	}))

	globalLogger = zap.New(core, options...)
	sugarLogger = globalLogger.Sugar()

	return nil
}

func getZapLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func getOutputWriter(outputPath string) zapcore.WriteSyncer {
	if outputPath == "" || outputPath == "stdout" {
		return zapcore.AddSync(os.Stdout)
	}

	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return zapcore.AddSync(os.Stdout)
	}

	return zapcore.AddSync(file)
}

func GetLogger() *zap.Logger {
	if globalLogger == nil {
		panic("logger not initialized, please call Init() first")
	}
	return globalLogger
}

func GetSugarLogger() *zap.SugaredLogger {
	if sugarLogger == nil {
		panic("logger not initialized, please call Init() first")
	}
	return sugarLogger
}

func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

func WithTraceID(traceID string) *zap.Logger {
	return GetLogger().With(zap.String("trace_id", traceID))
}

func WithContext(ctx context.Context) *zap.Logger {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	if !ok || traceID == "" {
		return GetLogger()
	}
	return WithTraceID(traceID)
}

func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

func Debugf(template string, args ...interface{}) {
	GetSugarLogger().Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	GetSugarLogger().Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	GetSugarLogger().Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	GetSugarLogger().Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	GetSugarLogger().Fatalf(template, args...)
}

func LogError(msg string, err error, fields ...zap.Field) {
	allFields := append(fields, zap.Error(err))
	Error(msg, allFields...)
}

func LogPanic(msg string, fields ...zap.Field) {
	fields = append(fields, zap.Stack("stack"))
	Error(msg, fields...)
}
