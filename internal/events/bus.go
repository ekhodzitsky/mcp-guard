// Package events provides an internal pub/sub event bus.
package events

import (
	"context"
	"log/slog"
	"sync"
)

const defaultBufferSize = 16

// Event represents an internal event.
type Event struct {
	Type    string
	Server  string
	Payload any
}

// Bus is a pub/sub event bus.
type Bus struct {
	mu     sync.RWMutex
	subs   map[string][]chan Event
	closed bool
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[string][]chan Event),
	}
}

// Subscribe registers a channel for events on a given server name.
// Callers must consume events from the returned channel or call Unsubscribe
// to avoid goroutine leaks.
func (b *Bus) Subscribe(server string) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	ch := make(chan Event, defaultBufferSize)
	b.subs[server] = append(b.subs[server], ch)
	return ch
}

// Unsubscribe removes a channel.
func (b *Bus) Unsubscribe(server string, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	subs := b.subs[server]
	for i, c := range subs {
		if c == ch {
			b.subs[server] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Publish sends an event to all subscribers for the server.
func (b *Bus) Publish(ctx context.Context, evt Event) {
	b.mu.RLock()
	subs := make([]chan Event, len(b.subs[evt.Server]))
	copy(subs, b.subs[evt.Server])
	b.mu.RUnlock()

	var wg sync.WaitGroup
	for _, ch := range subs {
		wg.Add(1)
		go func(c chan Event) {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); !ok || err.Error() != "send on closed channel" {
						slog.Error("events: unexpected panic in Publish", "error", r)
						panic(r)
					}
				}
				wg.Done()
			}()
			select {
			case c <- evt:
			case <-ctx.Done():
			default:
			}
		}(ch)
	}
	wg.Wait()
}

// Close locks, clears the map, and closes all remaining channels.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for _, subs := range b.subs {
		for _, ch := range subs {
			close(ch)
		}
	}
	b.subs = make(map[string][]chan Event)
}
