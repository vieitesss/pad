package tui

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunWithSpinnerReturnsResult(t *testing.T) {
	result, err := RunWithSpinner(context.Background(), "Testing spinner", func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Fatalf("expected result 'success', got %q", result)
	}
}

func TestRunWithSpinnerReturnsError(t *testing.T) {
	expectedErr := errors.New("test error")
	_, err := RunWithSpinner(context.Background(), "Testing spinner", func(ctx context.Context) (string, error) {
		return "", expectedErr
	})

	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestRunWithSpinnerNoResult(t *testing.T) {
	called := false
	err := RunWithSpinnerNoResult(context.Background(), "Testing spinner", func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected function to be called")
	}
}

func TestRunWithSpinnerRespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := RunWithSpinner(ctx, "Testing spinner", func(ctx context.Context) (string, error) {
		// Simulate long-running work
		select {
		case <-time.After(100 * time.Millisecond):
			return "completed", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}
}
