package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	hertzcontribslog "github.com/hertz-contrib/logger/slog"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/tracing/traceid"

	"github.com/yi-nology/zentao-max/backend/core/config"
)

// 日志后端：oekit/logx（slog 结构化）。
// - 格式化输出（console/json → stdout/文件）由 slog.TextHandler/JSONHandler 承担；
// - 日志页消费的进程内环形缓冲由 ringHandler 承担，与格式化输出经 fanoutHandler 双写；
// - trace_id 通过 logx.SetTraceExtractor 注入，Ctx* 方法与 WithCtx 自动携带。

var globalLogger logx.Logger

// Init 初始化全局 logger。
func Init(cfg *config.LogConfig) error {
	var out io.Writer = os.Stdout
	if cfg.OutputPath != "" && cfg.OutputPath != "stdout" {
		if f, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			out = f
		}
	}

	opts := &slog.HandlerOptions{Level: slogLevel(cfg.Level)}
	var fmtHandler slog.Handler
	if cfg.Format == "json" {
		fmtHandler = slog.NewJSONHandler(out, opts)
	} else {
		fmtHandler = slog.NewTextHandler(out, opts)
	}

	// caller 展示跟随配置（logx 侧在 emitToHandler 时捕获真实调用方）
	_ = cfg.EnableCaller

	globalLogger = logx.NewSlogLogger("zentao-max",
		logx.WithLevel(logx.ParseLogLevelOr(cfg.Level, logx.LevelInfo)),
		logx.WithHandler(&fanoutHandler{hs: []slog.Handler{fmtHandler, ringHandler{}}}),
	)

	// 向 logx 注册 trace_id 提取器：WithCtx/Ctx* 路径自动携带 trace_id。
	// trace_id 由 oekit hmw.Middleware 经 traceid.ContextWith 注入标准 context
	logx.SetTraceExtractor(traceid.FromContext)

	// async.GoSafe 的 panic 统一进 logx（业务 goroutine panic 不再走 std log）
	async.SetPanicHandler(func(msg string, stack []byte) {
		GetLogger().Error("goroutine panic recovered", "panic", msg, "stack", string(stack))
	})

	// hertz 内部日志（路由注册 [Debug] HERTZ、hlog.SystemLogger）桥接到 slog，
	// 统一走结构化输出，不再以 hlog 默认文本格式旁路
	hlog.SetLogger(hertzcontribslog.NewLogger(
		hertzcontribslog.WithOutput(os.Stdout),
		hertzcontribslog.WithHandlerOptions(&slog.HandlerOptions{Level: slogLevel(cfg.Level)}),
	))

	return nil
}

func slogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fanoutHandler 一条日志同时交给多个 slog.Handler（格式化输出 + 环形缓冲）。
type fanoutHandler struct{ hs []slog.Handler }

func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, sub := range h.hs {
		if sub.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, sub := range h.hs {
		if err := sub.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	subs := make([]slog.Handler, 0, len(h.hs))
	for _, sub := range h.hs {
		subs = append(subs, sub.WithAttrs(attrs))
	}
	return &fanoutHandler{hs: subs}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	subs := make([]slog.Handler, 0, len(h.hs))
	for _, sub := range h.hs {
		subs = append(subs, sub.WithGroup(name))
	}
	return &fanoutHandler{hs: subs}
}

// ringHandler 把 slog.Record 转成 LogEntry 追加到环形缓冲（供日志页 Query）。
type ringHandler struct{}

func (ringHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (ringHandler) Handle(_ context.Context, r slog.Record) error {
	entry := &LogEntry{
		Time:    r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		Level:   strings.ToLower(r.Level.String()),
		Message: r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "caller" {
			entry.Caller = fmt.Sprint(a.Value.Resolve())
			return false
		}
		return true
	})
	GetRingBuffer().Append(entry)
	return nil
}

func (ringHandler) WithAttrs(_ []slog.Attr) slog.Handler { return ringHandler{} }

func (ringHandler) WithGroup(_ string) slog.Handler { return ringHandler{} }

// GetLogger 返回全局 logx.Logger，未初始化时 panic（与旧语义一致）。
func GetLogger() logx.Logger {
	if globalLogger == nil {
		panic("logger not initialized, please call Init() first")
	}
	return globalLogger
}

var syncOnce sync.Once

// Sync 兼容旧调用点（defer logger.Sync()）。slog handler 直写无缓冲，
// 文件句柄交给进程退出统一回收；保留幂等空实现以免调用点变更。
func Sync() error {
	syncOnce.Do(func() {})
	return nil
}

func Debug(msg string, kv ...any) { GetLogger().Debug(msg, kv...) }
func Info(msg string, kv ...any)  { GetLogger().Info(msg, kv...) }
func Warn(msg string, kv ...any)  { GetLogger().Warn(msg, kv...) }
func Error(msg string, kv ...any) { GetLogger().Error(msg, kv...) }

// Fatal 记录 error 级日志后退出进程（logx 无 Fatal，保持原 zap.Fatal 语义）。
func Fatal(msg string, kv ...any) {
	GetLogger().Error(msg, kv...)
	os.Exit(1)
}

func Debugf(format string, args ...any) { GetLogger().Debugf(format, args...) }
func Infof(format string, args ...any)  { GetLogger().Infof(format, args...) }
func Warnf(format string, args ...any)  { GetLogger().Warnf(format, args...) }
func Errorf(format string, args ...any) { GetLogger().Errorf(format, args...) }

func Fatalf(format string, args ...any) {
	GetLogger().Errorf(format, args...)
	os.Exit(1)
}

// LogError 记录 error 字段的错误日志。
func LogError(msg string, err error, kv ...any) {
	GetLogger().Error(msg, append(kv, "error", err)...)
}

// LogPanic 记录 panic 场景的错误日志（附调用栈）。
func LogPanic(msg string, kv ...any) {
	GetLogger().Error(msg, append(kv, "stack", string(debug.Stack()))...)
}
