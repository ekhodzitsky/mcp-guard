package proxy

import (
	"context"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	ctx := context.Background()

	fast := func(ctx context.Context) ([]byte, error) {
		return []byte("ok"), nil
	}
	result, err := WithTimeout(ctx, 100*time.Millisecond, fast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "ok" {
		t.Fatalf("expected ok, got %s", string(result))
	}

	slow := func(ctx context.Context) ([]byte, error) {
		select {
		case <-time.After(500 * time.Millisecond):
			return []byte("done"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	_, err = WithTimeout(ctx, 50*time.Millisecond, slow)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
