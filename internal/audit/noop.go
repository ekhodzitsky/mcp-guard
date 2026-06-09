package audit

import "context"

// NoopLogger is a no-op audit logger.
type NoopLogger struct{}

// Log does nothing.
func (n *NoopLogger) Log(_ context.Context, _ LogEntry) error { return nil }

// Close does nothing.
func (n *NoopLogger) Close() error { return nil }
