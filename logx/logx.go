package logx

import (
	"context"
	stdfmt "fmt"
	"log/slog"
	"os"
	"sync/atomic"

	crdberrors "github.com/cockroachdb/errors"
)

var logger atomic.Value // holds *slog.Logger

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger.Store(slog.New(handler))
}

// SetLevel sets the logging level
func SetLevel(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger.Store(slog.New(handler))
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	get().Debug(msg, attrsToAny(argsToAttrs(args...))...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	get().Info(msg, attrsToAny(argsToAttrs(args...))...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	get().Warn(msg, attrsToAny(argsToAttrs(args...))...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	get().Error(msg, attrsToAny(argsToAttrs(args...))...)
}

// ErrorErr logs an error with enhanced details including stack trace, hints, details, and domain
func ErrorErr(msg string, err error, kv ...any) {
	if err == nil {
		Error(msg, kv...)
		return
	}

	// Extract rich error information
	attrs := []slog.Attr{
		slog.String("error", err.Error()),
		slog.String("error_verbose", stdfmt.Sprintf("%+v", err)),
	}

	// Add source location if available
	if file, line, fn, ok := crdberrors.GetOneLineSource(err); ok {
		attrs = append(attrs, slog.String("error_source", stdfmt.Sprintf("%s:%d in %s", file, line, fn)))
	}

	// Add hints if present
	if hints := crdberrors.GetAllHints(err); hints != nil && len(hints) > 0 {
		attrs = append(attrs, slog.Any("error_hints", hints))
	}

	// Add details if present
	if details := crdberrors.GetAllDetails(err); details != nil && len(details) > 0 {
		attrs = append(attrs, slog.Any("error_details", details))
	}

	// Add domain if present
	if domain := crdberrors.GetDomain(err); domain != crdberrors.NoDomain {
		attrs = append(attrs, slog.String("error_domain", stdfmt.Sprintf("%v", domain)))
	}

	// Append any additional key-value pairs safely
	attrs = append(attrs, argsToAttrs(kv...)...)
	get().Error(msg, attrsToAny(attrs)...)
}

// WarnErr logs a warning with error details
func WarnErr(msg string, err error, kv ...any) {
	if err == nil {
		Warn(msg, kv...)
		return
	}

	attrs := []slog.Attr{slog.String("error", err.Error())}

	// Add source location if available
	if file, line, fn, ok := crdberrors.GetOneLineSource(err); ok {
		attrs = append(attrs, slog.String("error_source", stdfmt.Sprintf("%s:%d in %s", file, line, fn)))
	}
	attrs = append(attrs, argsToAttrs(kv...)...)
	get().Warn(msg, attrsToAny(attrs)...)
}

// With returns a logger with additional key-value pairs
func With(args ...any) *slog.Logger {
	return get().With(attrsToAny(argsToAttrs(args...))...)
}

// WithContext creates a logger with context
func WithContext(ctx context.Context) *slog.Logger {
	// 例：context から request-id を拾って紐付ける
	if v := ctx.Value("request_id"); v != nil {
		return get().With(slog.String("request_id", stdfmt.Sprint(v)))
	}
	return get()
}

// Logger type alias for slog.Logger for easier usage
type Logger = slog.Logger

// WithComponent creates a logger with component context
func WithComponent(component string) *slog.Logger {
	return get().With(slog.String("component", component))
}

// PanicHandler is a utility to recover from panics and log them with stack trace
// It re-raises the panic after logging to ensure the process fails properly
func PanicHandler(component string) {
	if r := recover(); r != nil {
		err := crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))
		ErrorErr(stdfmt.Sprintf("[%s] Panic recovered", component), err)
		// Re-raise the panic to ensure proper failure handling
		panic(r)
	}
}

// SafeGo runs a goroutine with panic recovery
func SafeGo(name string, fn func()) {
	go func() {
		defer PanicHandler(name)
		fn()
	}()
}

// internal helpers
func get() *slog.Logger {
	return logger.Load().(*slog.Logger)
}

// argsToAttrs converts variadic keyvals safely to slog.Attr list
func argsToAttrs(kv ...any) []slog.Attr {
	// enforce even length
	if len(kv)%2 != 0 {
		kv = kv[:len(kv)-1]
	}
	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, val := kv[i], kv[i+1]
		// slog は string キー推奨
		k, ok := key.(string)
		if !ok {
			k = stdfmt.Sprint(key)
		}
		attrs = append(attrs, slog.Any(k, val))
	}
	return attrs
}

// attrsToAny converts []slog.Attr to []any for slog methods
func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, attr := range attrs {
		result[i] = attr
	}
	return result
}
