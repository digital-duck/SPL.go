package executor

import (
	"context"
	"errors"
	"testing"
)

func TestRunParallelMultiError(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	tasks := []func(context.Context) error{
		func(ctx context.Context) error { return err1 },
		func(ctx context.Context) error { return err2 },
		func(ctx context.Context) error { return nil },
	}

	err := runParallel(context.Background(), tasks)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Verify both errors are present (errors.Join wraps them)
	if !errors.Is(err, err1) {
		t.Errorf("Expected error to contain err1")
	}
	if !errors.Is(err, err2) {
		t.Errorf("Expected error to contain err2")
	}
}

func TestRunParallelContextCancel(t *testing.T) {
	err1 := errors.New("error 1")
	
	task1Started := make(chan struct{})
	task2Cancelled := make(chan struct{})

	tasks := []func(context.Context) error{
		func(ctx context.Context) error {
			close(task1Started)
			return err1
		},
		func(ctx context.Context) error {
			<-task1Started
			select {
			case <-ctx.Done():
				close(task2Cancelled)
				return ctx.Err()
			}
		},
	}

	err := runParallel(context.Background(), tasks)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	<-task2Cancelled // Wait for task 2 to be cancelled

	if !errors.Is(err, err1) {
		t.Errorf("Expected error to contain err1")
	}
}
