package main

import (
	"context"
	"fmt"
	"time"

	crdberrors "github.com/cockroachdb/errors"
	"github.com/kis9a/cockroachdb-errors-example/domain"
	"github.com/kis9a/cockroachdb-errors-example/logx"
)

// ExchangeAPI simulates an exchange API client
type ExchangeAPI struct {
	failureCount int
}

// FetchPrice simulates fetching price from exchange with potential failures
func (api *ExchangeAPI) FetchPrice(symbol string) (float64, error) {
	api.failureCount++

	// Simulate different types of failures
	switch api.failureCount {
	case 1:
		// Temporary network error (retriable)
		return 0, domain.NewExchangeError("NETWORK_ERROR", "connection timeout", true)
	case 2:
		// Rate limiting (retriable)
		return 0, domain.NewExchangeError("RATE_LIMIT", "too many requests", true)
	case 3:
		// Success
		return 50000.0, nil
	default:
		// Invalid symbol (permanent, not retriable)
		return 0, domain.NewExchangeError("INVALID_SYMBOL", "symbol not found", false)
	}
}

// DatabaseService simulates a database service
type DatabaseService struct{}

// SavePrice simulates saving price to database
func (db *DatabaseService) SavePrice(symbol string, price float64) error {
	// Simulate temporary database connection issue
	err := crdberrors.New("connection pool exhausted")
	err = domain.MarkTemporary(err)
	err = crdberrors.WithDomain(err, domain.DomainAdapters)
	err = crdberrors.WithHint(err, "Database connection pool is full, retry after a short delay")

	return domain.WrapWithStack(err, "failed to save price to database")
}

// PriceService orchestrates fetching and saving price data
type PriceService struct {
	api *ExchangeAPI
	db  *DatabaseService
}

// UpdatePrice fetches price from exchange and saves it to database
func (svc *PriceService) UpdatePrice(symbol string) error {
	// Fetch price from exchange
	price, err := svc.api.FetchPrice(symbol)
	if err != nil {
		// Wrap with usecase domain context
		return domain.WrapWithDomain(err, "failed to update price", domain.DomainUsecase)
	}

	// Save to database
	err = svc.db.SavePrice(symbol, price)
	if err != nil {
		// Wrap with usecase domain context
		return domain.WrapWithDomain(err, "failed to persist price", domain.DomainUsecase)
	}

	logx.Info("Price updated successfully",
		"symbol", symbol,
		"price", price,
	)

	return nil
}

// RetryWithBackoff retries an operation with exponential backoff
func RetryWithBackoff(
	operation func(context.Context) error,
	maxRetries int,
	initialDelay time.Duration,
) error {
	var lastErr error
	delay := initialDelay

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := operation(ctx)

		if err == nil {
			// Success
			if attempt > 1 {
				logx.Info("Operation succeeded after retry",
					"attempt", attempt,
					"max_retries", maxRetries,
				)
			}
			return nil
		}

		lastErr = err

		// Check if error is temporary and retriable
		if !domain.IsTemporary(err) {
			// Permanent error, don't retry
			logx.ErrorErr("Operation failed with permanent error", err,
				"attempt", attempt,
				"retry", false,
			)
			return err
		}

		// Temporary error, retry if we haven't exceeded max retries
		if attempt < maxRetries {
			logx.WarnErr("Operation failed with temporary error, retrying", err,
				"attempt", attempt,
				"max_retries", maxRetries,
				"retry_delay", delay,
			)

			// Exponential backoff with jitter, max 5s
			d := delay + time.Duration((int64(delay) / 5)) // ~20% ジッタ
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
			delay *= 2
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
		} else {
			// Max retries exceeded
			logx.ErrorErr("Operation failed after max retries", err,
				"attempt", attempt,
				"max_retries", maxRetries,
			)
		}
	}

	// All retries exhausted
	return crdberrors.Wrapf(lastErr, "operation failed after %d attempts", maxRetries)
}

func main() {
	fmt.Println("Demonstrating domain classification and retry control")
	fmt.Println("====================================================")

	api := &ExchangeAPI{}
	db := &DatabaseService{}
	svc := &PriceService{api: api, db: db}

	// Example 1: Automatic retry with temporary errors
	fmt.Println("\n=== Example 1: Retrying temporary errors ===")

	err := RetryWithBackoff(
		func(ctx context.Context) error {
			return svc.UpdatePrice("BTC/USD")
		},
		5,                    // max 5 retries
		500*time.Millisecond, // initial delay
	)

	if err != nil {
		logx.ErrorErr("Final result: failed to update price", err)
	} else {
		fmt.Println("Final result: price updated successfully")
	}

	// Example 2: No retry for permanent errors
	fmt.Println("\n=== Example 2: Permanent error (no retry) ===")

	err = RetryWithBackoff(
		func(ctx context.Context) error {
			return svc.UpdatePrice("INVALID")
		},
		5,
		500*time.Millisecond,
	)

	if err != nil {
		logx.ErrorErr("Final result: failed with permanent error", err)

		// Check error domain
		errorDomain := crdberrors.GetDomain(err)
		fmt.Printf("Error domain: %v\n", errorDomain)

		// Check if error is permanent
		if domain.IsPermanent(err) {
			fmt.Println("This error is permanent and should not be retried")
		}
	}

	// Example 3: Domain-based error classification
	fmt.Println("\n=== Example 3: Domain-based error classification ===")

	// Create errors from different domains
	usecaseErr := crdberrors.New("business logic validation failed")
	usecaseErr = crdberrors.WithDomain(usecaseErr, domain.DomainUsecase)

	adapterErr := crdberrors.New("database query failed")
	adapterErr = crdberrors.WithDomain(adapterErr, domain.DomainAdapters)

	exchangeErr := domain.NewExchangeError("API_ERROR", "exchange API failed", true)

	// Check domains
	fmt.Printf("Usecase error domain: %v\n", crdberrors.GetDomain(usecaseErr))
	fmt.Printf("Adapter error domain: %v\n", crdberrors.GetDomain(adapterErr))
	fmt.Printf("Exchange error domain: %v\n", crdberrors.GetDomain(exchangeErr))

	fmt.Println("\n=== Summary ===")
	fmt.Println("Key benefits of domain classification:")
	fmt.Println("1. Automatic retry for temporary errors")
	fmt.Println("2. Skip retry for permanent errors")
	fmt.Println("3. Domain-based error categorization")
	fmt.Println("4. Clear error context and troubleshooting hints")
}
