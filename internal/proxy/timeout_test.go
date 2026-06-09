package proxy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		timeout     time.Duration
		fn          TimeoutFunc[[]byte]
		wantResult  []byte
		wantErr     bool
		wantTimeout bool
	}{
		{
			name: "fast success",
			ctx:  context.Background(),
			fn: func(ctx context.Context) ([]byte, error) {
				return []byte("ok"), nil
			},
			wantResult: []byte("ok"),
		},
		{
			name:    "slow function times out",
			ctx:     context.Background(),
			timeout: 50 * time.Millisecond,
			fn: func(ctx context.Context) ([]byte, error) {
				select {
				case <-time.After(500 * time.Millisecond):
					return []byte("done"), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
			wantErr:     true,
			wantTimeout: true,
		},
		{
			name: "already cancelled parent context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			fn: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return []byte("ok"), nil
			},
			wantErr: true,
		},
		{
			name:    "zero timeout",
			ctx:     context.Background(),
			timeout: 0,
			fn: func(ctx context.Context) ([]byte, error) {
				select {
				case <-time.After(500 * time.Millisecond):
					return []byte("done"), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
			wantErr:     true,
			wantTimeout: true,
		},
		{
			name: "non-timeout error",
			ctx:  context.Background(),
			fn: func(ctx context.Context) ([]byte, error) {
				return nil, errors.New("something went wrong")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := WithTimeout(tt.ctx, tt.timeout, tt.fn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("WithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTimeout && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected timeout error wrapping context.DeadlineExceeded, got: %v", err)
			}
			if string(result) != string(tt.wantResult) {
				t.Fatalf("expected %s, got %s", tt.wantResult, result)
			}
		})
	}
}

func TestWithTimeoutDifferentType(t *testing.T) {
	fn := func(ctx context.Context) (string, error) {
		return "hello", nil
	}
	result, err := WithTimeout(context.Background(), 100*time.Millisecond, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Fatalf("expected hello, got %s", result)
	}
}
