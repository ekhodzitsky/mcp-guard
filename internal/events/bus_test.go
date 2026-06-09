package events

import (
	"context"
	"testing"
	"time"
)

func TestBus(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "happy path single subscriber",
			run: func(t *testing.T) {
				bus := NewBus()
				defer bus.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				ch := bus.Subscribe("server.echo")

				go bus.Publish(ctx, Event{Type: "health.changed", Server: "server.echo", Payload: map[string]string{"status": "ok"}})

				select {
				case evt := <-ch:
					if evt.Server != "server.echo" {
						t.Fatalf("expected server echo, got %s", evt.Server)
					}
				case <-ctx.Done():
					t.Fatal("timed out waiting for event")
				}
			},
		},
		{
			name: "multiple subscribers same server",
			run: func(t *testing.T) {
				bus := NewBus()
				defer bus.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				ch1 := bus.Subscribe("server.echo")
				ch2 := bus.Subscribe("server.echo")

				go bus.Publish(ctx, Event{Type: "test", Server: "server.echo", Payload: "data"})

				var got1, got2 bool
				for i := 0; i < 2; i++ {
					select {
					case <-ch1:
						got1 = true
					case <-ch2:
						got2 = true
					case <-ctx.Done():
						t.Fatal("timed out waiting for events")
					}
				}

				if !got1 || !got2 {
					t.Fatal("both subscribers should have received the event")
				}
			},
		},
		{
			name: "multiple servers",
			run: func(t *testing.T) {
				bus := NewBus()
				defer bus.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				ch1 := bus.Subscribe("server.a")
				ch2 := bus.Subscribe("server.b")

				go bus.Publish(ctx, Event{Type: "test", Server: "server.a", Payload: "data"})

				select {
				case evt := <-ch1:
					if evt.Server != "server.a" {
						t.Fatalf("expected server.a, got %s", evt.Server)
					}
				case <-ctx.Done():
					t.Fatal("timed out waiting for server.a")
				}

				select {
				case <-ch2:
					t.Fatal("server.b should not have received the event")
				default:
				}
			},
		},
		{
			name: "context cancellation",
			run: func(t *testing.T) {
				bus := NewBus()
				defer bus.Close()

				ctx, cancel := context.WithCancel(context.Background())
				ch := bus.Subscribe("server.echo")

				// Fill the channel buffer so the next send blocks.
				for i := 0; i < defaultBufferSize; i++ {
					select {
					case ch <- Event{Type: "fill"}:
					default:
						t.Fatal("failed to fill buffer")
					}
				}

				cancel()

				done := make(chan struct{})
				go func() {
					bus.Publish(ctx, Event{Type: "test", Server: "server.echo"})
					close(done)
				}()

				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("Publish should return after context cancellation")
				}
			},
		},
		{
			name: "unsubscribe does not panic on publish",
			run: func(t *testing.T) {
				bus := NewBus()
				defer bus.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				ch := bus.Subscribe("server.echo")
				bus.Unsubscribe("server.echo", ch)

				// Publish should not panic even though ch is unsubscribed.
				bus.Publish(ctx, Event{Type: "test", Server: "server.echo"})

				// Verify the channel was not closed by Unsubscribe.
				select {
				case _, ok := <-ch:
					if !ok {
						t.Fatal("channel was closed by Unsubscribe")
					}
				default:
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
