package domain

import (
	"fmt"

	crdberrors "github.com/cockroachdb/errors"
)

// Error domains for categorization
var (
	DomainUsecase  = crdberrors.NamedDomain("usecase")
	DomainAdapters = crdberrors.NamedDomain("adapters")
	DomainExchange = crdberrors.NamedDomain("exchange")
)

// Sentinel errors for common conditions
var (
	// ErrTemporary indicates a temporary error that can be retried
	ErrTemporary = crdberrors.New("temporary error")

	// ErrPermanent indicates a permanent error that should not be retried
	ErrPermanent = crdberrors.New("permanent error")

	// ErrNotFound indicates a resource was not found
	ErrNotFound = crdberrors.New("not found")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = crdberrors.New("timeout")

	// ErrRateLimited indicates rate limiting
	ErrRateLimited = crdberrors.New("rate limited")
)

// MarkTemporary marks an error as temporary/retriable
func MarkTemporary(err error) error {
	return crdberrors.Mark(err, ErrTemporary)
}

// IsTemporary checks if an error is temporary
func IsTemporary(err error) bool {
	return crdberrors.Is(err, ErrTemporary)
}

// MarkPermanent marks an error as permanent
func MarkPermanent(err error) error {
	return crdberrors.Mark(err, ErrPermanent)
}

// IsPermanent checks if an error is permanent
func IsPermanent(err error) bool {
	return crdberrors.Is(err, ErrPermanent)
}

// ExchangeError represents errors from exchange operations
type ExchangeError struct {
	Code    string
	Message string
	Retry   bool
}

func (e *ExchangeError) Error() string {
	return fmt.Sprintf("exchange error [%s]: %s", e.Code, e.Message)
}

// NewExchangeError creates a new ExchangeError with proper categorization
func NewExchangeError(code, message string, retry bool) error {
	base := &ExchangeError{
		Code:    code,
		Message: message,
		Retry:   retry,
	}

	// Create one boundary with stack + domain
	wrapped := crdberrors.WithDomain(crdberrors.WithStack(base), DomainExchange)

	// Add details
	wrapped = crdberrors.WithDetailf(wrapped, "code=%s retry=%v", code, retry)

	// Mark as temporary if retriable
	if retry {
		wrapped = MarkTemporary(wrapped)
		wrapped = crdberrors.WithHint(wrapped, "This error is temporary and can be retried")
	} else {
		wrapped = MarkPermanent(wrapped)
		wrapped = crdberrors.WithHint(wrapped, "This error is permanent and should not be retried")
	}

	// Add telemetry key for metrics
	wrapped = crdberrors.WithTelemetry(wrapped, "exchange.error."+code)

	return wrapped
}

// WrapWithDomain wraps an error with a specific domain
func WrapWithDomain(err error, msg string, domain crdberrors.Domain) error {
	if err == nil {
		return nil
	}
	// Wrap（境界）で十分。WithStackはここでは付けない（二重化回避）
	return crdberrors.WithDomain(crdberrors.Wrap(err, msg), domain)
}

// WrapWithStack wraps an error with message and stack trace (for error boundaries)
func WrapWithStack(err error, msg string) error {
	if err == nil {
		return nil
	}
	// 「境界」になっている箇所のみ使用
	return crdberrors.WithStack(crdberrors.Wrap(err, msg))
}

// IsExchangeCode reports whether err is an ExchangeError with the given code.
func IsExchangeCode(err error, code string) bool {
	var ex *ExchangeError
	if crdberrors.As(err, &ex) {
		return ex.Code == code
	}
	return false
}
