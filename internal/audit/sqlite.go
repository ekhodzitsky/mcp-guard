package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore persists audit logs in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) an SQLite audit database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		server TEXT NOT NULL,
		direction TEXT NOT NULL,
		message TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_audit_server ON audit_log(server);
	CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Log inserts an entry into SQLite.
func (s *SQLiteStore) Log(ctx context.Context, entry LogEntry) error {
	b, err := json.Marshal(entry.Message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO audit_log (timestamp, server, direction, message) VALUES (?, ?, ?, ?)",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Server,
		entry.Direction,
		string(b),
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
