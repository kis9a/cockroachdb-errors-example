package main

import (
	"fmt"

	crdberrors "github.com/cockroachdb/errors"
	"github.com/kis9a/cockroachdb-errors-example/logx"
)

// demonstrateStandardErrors shows basic error handling with standard errors package
func demonstrateStandardErrors() error {
	fmt.Println("\n=== Standard errors package ===")

	// Standard error creation - no stack trace
	err := fmt.Errorf("database connection failed: %w", fmt.Errorf("connection timeout"))

	// Standard error only shows the message
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Error with %%+v: %+v\n", err)

	return err
}

// demonstrateCockroachDBErrors shows enhanced error handling with cockroachdb/errors
func demonstrateCockroachDBErrors() error {
	fmt.Println("\n=== cockroachdb/errors package ===")

	// New / Wrap はスタック情報を保持するので WithStack は不要
	err := crdberrors.Wrap(crdberrors.New("connection timeout"), "database connection failed")

	// cockroachdb/errors provides rich error information
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Error with %%+v (includes stack trace): %+v\n", err)

	return err
}

// performDatabaseOperation simulates a database operation that might fail
func performDatabaseOperation(shouldFail bool) error {
	if shouldFail {
		// Create error with additional context
		err := crdberrors.New("query execution failed")
		err = crdberrors.WithHint(err, "Check if the database is accessible")
		err = crdberrors.WithDetailf(err, "query=%s timeout=%dms", "SELECT * FROM users", 5000)

		return crdberrors.Wrap(err, "failed to fetch user data")
	}
	return nil
}

func main() {
	fmt.Println("Demonstrating cockroachdb/errors basic usage")
	fmt.Println("=============================================")

	// 1. Standard errors vs cockroachdb/errors comparison
	demonstrateStandardErrors()
	demonstrateCockroachDBErrors()

	// 2. Using logx.ErrorErr for structured logging with stack traces
	fmt.Println("\n=== Using logx.ErrorErr for structured logging ===")

	err := performDatabaseOperation(true)
	if err != nil {
		// logx.ErrorErr extracts rich error information including:
		// - error message
		// - stack trace
		// - hints
		// - details
		// - source location
		logx.ErrorErr("Database operation failed", err,
			"user_id", 12345,
			"operation", "fetch_user_data",
		)
	}

	// 3. Creating errors with context
	fmt.Println("\n=== Creating errors with context ===")

	// Create base error
	baseErr := crdberrors.New("insufficient balance")

	// New 由来のスタックで十分（ここでは WithStack を外す）
	enrichedErr := baseErr

	// Add hint for troubleshooting
	enrichedErr = crdberrors.WithHint(enrichedErr, "User needs to deposit more funds")

	// Add structured details
	enrichedErr = crdberrors.WithDetailf(enrichedErr, "balance=%d required=%d", 100, 500)

	// Wrap with higher-level context
	finalErr := crdberrors.Wrap(enrichedErr, "payment processing failed")

	// Log with rich context
	logx.ErrorErr("Payment failed", finalErr,
		"payment_id", "pay_123",
		"amount", 500,
	)

	fmt.Println("\n=== Summary ===")
	fmt.Println("Key benefits of cockroachdb/errors:")
	fmt.Println("1. Automatic stack trace capture")
	fmt.Println("2. Hints for troubleshooting")
	fmt.Println("3. Structured details")
	fmt.Println("4. Source location tracking")
	fmt.Println("5. Error wrapping with context preservation")
}
