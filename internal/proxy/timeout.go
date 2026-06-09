// Package proxy provides the JSON-RPC stdio proxy.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TimeoutFunc is a function that can be wrapped with a timeout.
type TimeoutFunc[T any] func(ctx context.Context) (T, error)

// WithTimeout executes fn with the given timeout.
func WithTimeout[T any](ctx context.Context, timeout time.Duration, fn TimeoutFunc[T]) (T, error) {
	var zero T
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := fn(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return zero, fmt.Errorf("request timed out after %v: %w", timeout, err)
		}
		return zero, err
	}
	return result, nil
}
