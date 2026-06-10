package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore persists audit logs in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) an SQLite audit database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
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

// RecentEntry represents a recent audit log entry.
type RecentEntry struct {
	Timestamp string `json:"timestamp"`
	Server    string `json:"server"`
	Direction string `json:"direction"`
	Message   string `json:"message"`
}

// Recent returns the most recent audit log entries.
func (s *SQLiteStore) Recent(ctx context.Context, limit int) ([]RecentEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT timestamp, server, direction, message FROM audit_log ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []RecentEntry
	for rows.Next() {
		var e RecentEntry
		if err := rows.Scan(&e.Timestamp, &e.Server, &e.Direction, &e.Message); err != nil {
			return nil, fmt.Errorf("scan audit row: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
