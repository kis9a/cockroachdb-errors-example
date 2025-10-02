package main

import (
	"fmt"
	"sync"
	"time"

	crdberrors "github.com/cockroachdb/errors"
	"github.com/kis9a/cockroachdb-errors-example/logx"
)

// riskyOperation simulates an operation that might panic
func riskyOperation(shouldPanic bool, panicType string) {
	if shouldPanic {
		switch panicType {
		case "nil_pointer":
			var ptr *string
			// This will panic with nil pointer dereference
			_ = *ptr
		case "index_out_of_range":
			arr := []int{1, 2, 3}
			// This will panic with index out of range
			_ = arr[10]
		case "explicit":
			// Explicit panic
			panic("explicit panic triggered")
		default:
			panic(fmt.Sprintf("unknown panic type: %s", panicType))
		}
	}

	fmt.Println("Operation completed successfully")
}

// safeOperationWithManualRecovery demonstrates manual panic recovery
func safeOperationWithManualRecovery(shouldPanic bool, panicType string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Create error from panic with stack trace
			err = crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))

			// Log the panic with full context
			logx.ErrorErr("Manual panic recovery", err,
				"panic_value", r,
				"panic_type", panicType,
			)
		}
	}()

	riskyOperation(shouldPanic, panicType)
	return nil
}

// processTask simulates a task that might panic
func processTask(taskID int, shouldPanic bool) {
	fmt.Printf("Processing task %d...\n", taskID)

	if shouldPanic {
		// Simulate panic in task processing
		panic(fmt.Sprintf("task %d failed unexpectedly", taskID))
	}

	// Simulate some work
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Task %d completed\n", taskID)
}

// taskWorker simulates a worker that processes tasks with SafeGo
func taskWorker() {
	fmt.Println("\n=== Example 1: Using SafeGo for goroutine panic recovery ===")

	// Start multiple goroutines with SafeGo
	// Note: SafeGo uses PanicHandler which re-raises panics after logging
	// In production, you might want to use manual recovery instead

	// Start goroutines
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))
				logx.ErrorErr("[task-worker-1] Panic recovered (no re-raise)", err)
			}
			wg.Done()
		}()
		processTask(1, false) // Normal completion
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))
				logx.ErrorErr("[task-worker-2] Panic recovered (no re-raise)", err)
			}
			wg.Done()
		}()
		processTask(2, true) // This will panic but we recover gracefully
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))
				logx.ErrorErr("[task-worker-3] Panic recovered (no re-raise)", err)
			}
			wg.Done()
		}()
		processTask(3, false) // Normal completion
	}()

	// Wait for goroutines to complete
	wg.Wait()
	fmt.Println("All goroutines completed (task 2 panicked but was recovered)")
}

// demonstrateManualRecovery shows manual panic recovery with defer
func demonstrateManualRecovery() {
	fmt.Println("\n=== Example 2: Manual panic recovery with defer ===")

	// Test different panic types
	panicTypes := []string{"nil_pointer", "index_out_of_range", "explicit"}

	for _, panicType := range panicTypes {
		fmt.Printf("\nTesting panic type: %s\n", panicType)

		err := safeOperationWithManualRecovery(true, panicType)
		if err != nil {
			fmt.Printf("Recovered from panic: %v\n", err)
		}
	}

	// Test successful operation (no panic)
	fmt.Println("\nTesting successful operation:")
	err := safeOperationWithManualRecovery(false, "")
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Println("Operation completed without panic")
	}
}

// demonstratePanicHandler shows PanicHandler usage
func demonstratePanicHandler() {
	fmt.Println("\n=== Example 3: Using PanicHandler ===")

	// PanicHandler will catch panic, log it, and re-raise it
	// This is useful when you want to log panic but still fail properly

	fmt.Println("This will demonstrate panic logging and re-raising")
	fmt.Println("Note: The program will panic after logging")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\nCaught re-raised panic: %v\n", r)
			fmt.Println("In production, this would cause the process to exit")
		}
	}()

	defer logx.PanicHandler("main")

	// This will panic, get logged by PanicHandler, and then re-raised
	panic("critical error in main")
}

// backgroundWorker simulates a long-running background worker
func backgroundWorker(workerID int, taskCount int) {
	var wg sync.WaitGroup
	for i := 1; i <= taskCount; i++ {
		// Each task runs in a goroutine with manual recovery
		taskNum := i
		workerName := fmt.Sprintf("worker-%d-task-%d", workerID, taskNum)

		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					err := crdberrors.WithStack(crdberrors.Errorf("panic recovered: %v", r))
					logx.ErrorErr(fmt.Sprintf("[%s] Task panic recovered", workerName), err)
				}
				wg.Done()
			}()

			// Simulate random panic (20% chance)
			shouldPanic := (taskNum % 5) == 0
			processTask(taskNum, shouldPanic)
		}()

		time.Sleep(50 * time.Millisecond)
	}
	wg.Wait()
}

func main() {
	fmt.Println("Demonstrating panic recovery with cockroachdb/errors")
	fmt.Println("===================================================")
	fmt.Println("\nNote: This example uses manual panic recovery for better control.")
	fmt.Println("For SafeGo usage (which re-raises panics), see the logx package.")

	// Example 1: Manual goroutine panic recovery
	taskWorker()

	// Example 2: Manual panic recovery with different panic types
	demonstrateManualRecovery()

	// Example 3: Background worker with multiple tasks
	fmt.Println("\n=== Example 4: Background worker with panic recovery ===")
	fmt.Println("Starting background worker with 10 tasks (some will panic)")

	backgroundWorker(1, 10)

	// Wait for all tasks to complete (wait inside backgroundWorker)
	fmt.Println("\nAll background tasks completed")

	// Example 4: PanicHandler (this will panic at the end)
	// Uncomment to see PanicHandler in action
	// demonstratePanicHandler()

	fmt.Println("\n=== Summary ===")
	fmt.Println("Key benefits of panic recovery:")
	fmt.Println("1. Manual recovery: Prevents panics from crashing goroutines")
	fmt.Println("2. PanicHandler: Logs panics with full stack trace before re-raising (for critical failures)")
	fmt.Println("3. SafeGo: Convenience wrapper that uses PanicHandler (re-raises after logging)")
	fmt.Println("4. All panics are logged with structured information and stack traces")
	fmt.Println("\nNote: Uncomment demonstratePanicHandler() to see PanicHandler re-raising behavior")
}
