package benchmark

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	crdberrors "github.com/cockroachdb/errors"
	"github.com/kis9a/cockroachdb-errors-example/domain"
)

// Global variables to prevent compiler optimizations
var (
	result    error
	logOutput string
)

// createLogger creates a logger that writes to a buffer for benchmarking
func createLogger() (*slog.Logger, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(buf, opts)
	return slog.New(handler), buf
}

// createNullLogger creates a logger that discards output for fair comparison
func createNullLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(io.Discard, opts)
	return slog.New(handler)
}

// BenchmarkStdErrors benchmarks standard errors package
// Creates error, wraps it, and logs it
func BenchmarkStdErrors(b *testing.B) {
	logger := createNullLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create base error
		baseErr := errors.New("connection timeout")

		// Wrap error with context
		wrappedErr := fmt.Errorf("database connection failed: %w", baseErr)

		// Wrap again with more context
		finalErr := fmt.Errorf("operation failed: %w", wrappedErr)

		// Log the error
		logger.Error("error occurred", "error", finalErr.Error())

		result = finalErr
	}
}

// BenchmarkCrdberrorsBasic benchmarks cockroachdb/errors with minimal features
// Uses basic New and Wrap without stack traces or additional metadata
func BenchmarkCrdberrorsBasic(b *testing.B) {
	logger := createNullLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create base error (no stack trace)
		baseErr := crdberrors.NewWithDepth(1, "connection timeout")

		// Wrap error with context
		wrappedErr := crdberrors.Wrap(baseErr, "database connection failed")

		// Wrap again with more context
		finalErr := crdberrors.Wrap(wrappedErr, "operation failed")

		// Log the error
		logger.Error("error occurred", "error", finalErr.Error())

		result = finalErr
	}
}

// BenchmarkCrdberrorsWithStack benchmarks cockroachdb/errors with stack traces
// This is the recommended approach for production error handling
func BenchmarkCrdberrorsWithStack(b *testing.B) {
	logger := createNullLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create base error with stack trace
		baseErr := crdberrors.New("connection timeout")

		// Wrap error with context (preserves stack)
		wrappedErr := crdberrors.Wrap(baseErr, "database connection failed")

		// Wrap again with more context
		finalErr := crdberrors.Wrap(wrappedErr, "operation failed")

		// Log with verbose format (includes stack trace)
		logger.Error("error occurred",
			"error", finalErr.Error(),
			"error_verbose", fmt.Sprintf("%+v", finalErr),
		)

		result = finalErr
	}
}

// BenchmarkCrdberrorsWithHints benchmarks cockroachdb/errors with full features
// Includes stack traces, hints, details, and domain classification
func BenchmarkCrdberrorsWithHints(b *testing.B) {
	logger := createNullLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create base error with stack trace
		baseErr := crdberrors.New("connection timeout")

		// Add hint
		enrichedErr := crdberrors.WithHint(baseErr, "Check if the database is accessible")

		// Add structured details
		enrichedErr = crdberrors.WithDetailf(enrichedErr, "timeout=%dms retry=%d", 5000, 3)

		// Add domain classification
		enrichedErr = crdberrors.WithDomain(enrichedErr, domain.DomainAdapters)

		// Mark as temporary for retry logic
		enrichedErr = domain.MarkTemporary(enrichedErr)

		// Wrap with context
		wrappedErr := crdberrors.Wrap(enrichedErr, "database connection failed")

		// Wrap again with more context
		finalErr := crdberrors.Wrap(wrappedErr, "operation failed")

		// Log with full context extraction
		var hints []string
		if h := crdberrors.GetAllHints(finalErr); h != nil && len(h) > 0 {
			hints = h
		}
		var details []string
		if d := crdberrors.GetAllDetails(finalErr); d != nil && len(d) > 0 {
			details = d
		}
		domainStr := fmt.Sprintf("%v", crdberrors.GetDomain(finalErr))

		logger.Error("error occurred",
			"error", finalErr.Error(),
			"error_verbose", fmt.Sprintf("%+v", finalErr),
			"hints", hints,
			"details", details,
			"domain", domainStr,
			"is_temporary", domain.IsTemporary(finalErr),
		)

		result = finalErr
	}
}

// BenchmarkStdErrorsDeep benchmarks standard errors with deeper call stack
func BenchmarkStdErrorsDeep(b *testing.B) {
	logger := createNullLogger()

	deepError := func() error {
		return errors.New("connection timeout")
	}

	middleLayer := func() error {
		err := deepError()
		return fmt.Errorf("database connection failed: %w", err)
	}

	topLayer := func() error {
		err := middleLayer()
		return fmt.Errorf("operation failed: %w", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finalErr := topLayer()
		logger.Error("error occurred", "error", finalErr.Error())
		result = finalErr
	}
}

// BenchmarkCrdberrorsDeep benchmarks cockroachdb/errors with deeper call stack
func BenchmarkCrdberrorsDeep(b *testing.B) {
	logger := createNullLogger()

	deepError := func() error {
		return crdberrors.New("connection timeout")
	}

	middleLayer := func() error {
		err := deepError()
		return crdberrors.Wrap(err, "database connection failed")
	}

	topLayer := func() error {
		err := middleLayer()
		return crdberrors.Wrap(err, "operation failed")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finalErr := topLayer()
		logger.Error("error occurred",
			"error", finalErr.Error(),
			"error_verbose", fmt.Sprintf("%+v", finalErr),
		)
		result = finalErr
	}
}

// BenchmarkExchangeError benchmarks the domain-specific ExchangeError
// This simulates real-world usage with full error enrichment
func BenchmarkExchangeError(b *testing.B) {
	logger := createNullLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create exchange error with all enrichments
		err := domain.NewExchangeError("INSUFFICIENT_BALANCE", "not enough funds", true)

		// Wrap in higher-level context
		wrappedErr := domain.WrapWithDomain(err, "failed to place order", domain.DomainUsecase)

		// Extract and log all metadata
		var hints []string
		if h := crdberrors.GetAllHints(wrappedErr); h != nil && len(h) > 0 {
			hints = h
		}
		var details []string
		if d := crdberrors.GetAllDetails(wrappedErr); d != nil && len(d) > 0 {
			details = d
		}
		domainStr := fmt.Sprintf("%v", crdberrors.GetDomain(wrappedErr))

		logger.Error("exchange operation failed",
			"error", wrappedErr.Error(),
			"error_verbose", fmt.Sprintf("%+v", wrappedErr),
			"hints", hints,
			"details", details,
			"domain", domainStr,
			"is_temporary", domain.IsTemporary(wrappedErr),
		)

		result = wrappedErr
	}
}

// BenchmarkErrorFormatting benchmarks different error formatting approaches
func BenchmarkErrorFormatting(b *testing.B) {
	err := crdberrors.New("test error")
	err = crdberrors.WithHint(err, "this is a hint")
	err = crdberrors.WithDetailf(err, "detail=%s", "value")

	b.Run("SimpleString", func(b *testing.B) {
		var s string
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s = err.Error()
		}
		logOutput = s
	})

	b.Run("VerboseFormat", func(b *testing.B) {
		var s string
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s = fmt.Sprintf("%+v", err)
		}
		logOutput = s
	})

	b.Run("MetadataExtraction", func(b *testing.B) {
		var hints []string
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hints = crdberrors.GetAllHints(err)
			_ = crdberrors.GetAllDetails(err)
			_ = crdberrors.GetDomain(err)
		}
		if len(hints) > 0 {
			logOutput = hints[0]
		}
	})
}

// BenchmarkErrorChecking benchmarks error type checking and marking
func BenchmarkErrorChecking(b *testing.B) {
	err := crdberrors.New("test error")
	err = domain.MarkTemporary(err)

	b.Run("IsCheck", func(b *testing.B) {
		var isTemp bool
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			isTemp = domain.IsTemporary(err)
		}
		_ = isTemp
	})

	b.Run("StdErrorIs", func(b *testing.B) {
		stdErr := errors.New("test error")
		sentinel := errors.New("sentinel")
		var is bool
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			is = errors.Is(stdErr, sentinel)
		}
		_ = is
	})

	b.Run("CrdberrorsIs", func(b *testing.B) {
		var is bool
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			is = crdberrors.Is(err, domain.ErrTemporary)
		}
		_ = is
	})
}
