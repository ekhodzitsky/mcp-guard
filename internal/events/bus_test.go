package events

import (
	"context"
	"testing"
	"time"
)

func TestBusPubSub(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := bus.Subscribe("server.echo")
	defer bus.Unsubscribe("server.echo", ch)

	go func() {
		bus.Publish(ctx, Event{Type: "health.changed", Server: "server.echo", Payload: map[string]string{"status": "ok"}})
	}()

	select {
	case evt := <-ch:
		if evt.Server != "server.echo" {
			t.Fatalf("expected server echo, got %s", evt.Server)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}
