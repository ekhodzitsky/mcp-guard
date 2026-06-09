// Package audit provides audit logging for MCP messages.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogEntry represents a single audit log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Server    string    `json:"server"`
	Direction string    `json:"direction"` // "request" or "response"
	Message   any       `json:"message"`
}

// Logger is the interface for audit logging.
type Logger interface {
	Log(ctx context.Context, entry LogEntry) error
	Close() error
}

// MultiLogger logs to multiple backends.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a multi-backend logger.
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Log writes to all backends.
func (m *MultiLogger) Log(ctx context.Context, entry LogEntry) error {
	var errs []error
	for _, l := range m.loggers {
		if err := l.Log(ctx, entry); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Close closes all backends.
func (m *MultiLogger) Close() error {
	var errs []error
	for _, l := range m.loggers {
		if err := l.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// JSONLinesLogger writes audit entries as newline-delimited JSON.
type JSONLinesLogger struct {
	mu     sync.Mutex
	writer io.WriteCloser
	closed bool
}

// NewJSONLinesLogger creates a JSON Lines audit logger.
func NewJSONLinesLogger(path string) (*JSONLinesLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create audit dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &JSONLinesLogger{writer: f}, nil
}

// Log writes a JSON Lines entry.
func (j *JSONLinesLogger) Log(_ context.Context, entry LogEntry) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return errors.New("jsonlines logger: already closed")
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	b = append(b, '\n')
	if _, err := j.writer.Write(b); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	return nil
}

// Close closes the file.
func (j *JSONLinesLogger) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return errors.New("jsonlines logger: already closed")
	}
	j.closed = true
	return j.writer.Close()
}
