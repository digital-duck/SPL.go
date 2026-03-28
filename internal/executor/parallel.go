package executor

import (
	"context"
	"sync"
)

// runParallel runs a batch of independent tasks concurrently.
// Each task is a func(ctx) error.
// Returns the first non-nil error encountered, or nil if all succeed.
// Respects context cancellation: if one task fails, remaining tasks see a cancelled context.
func runParallel(ctx context.Context, tasks []func(context.Context) error) error {
	if len(tasks) == 0 {
		return nil
	}
	if len(tasks) == 1 {
		return tasks[0](ctx)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		task := task // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task(ctx); err != nil {
				cancel() // signal other goroutines to stop
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	// Return the first error if any
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
