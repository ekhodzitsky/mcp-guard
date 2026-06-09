// Package events provides an internal pub/sub event bus.
package events

import (
	"context"
	"sync"
)

// Event represents an internal event.
type Event struct {
	Type    string
	Server  string
	Payload any
}

// Bus is a pub/sub event bus.
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[string][]chan Event),
	}
}

// Subscribe registers a channel for events on a given server name.
func (b *Bus) Subscribe(server string) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 16)
	b.subs[server] = append(b.subs[server], ch)
	return ch
}

// Unsubscribe removes a channel.
func (b *Bus) Unsubscribe(server string, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subs[server]
	for i, c := range subs {
		if c == ch {
			close(c)
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

	for _, ch := range subs {
		select {
		case ch <- evt:
		case <-ctx.Done():
			return
		}
	}
}
