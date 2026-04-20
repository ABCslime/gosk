// Package state is gosk's SQLite-backed persistence layer for in-flight
// downloads. Enables resume across process restarts: if the muzika binary
// crashes mid-transfer, the updater reconciles on startup by replaying the
// last persisted state and re-checking the filesystem.
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, matches muzika's choice
)

// Record is one in-flight or completed download.
type Record struct {
	HandleID string
	Peer     string
	Filename string
	Size     int64
	State    string
	Bytes    int64
	FilePath string
}

// ErrNotFound is returned when Get can't find a handle.
var ErrNotFound = errors.New("state: record not found")

// Store persists Record values to a SQLite file. Safe for concurrent use
// (backed by database/sql with max 1 open conn per muzika's convention).
type Store struct {
	db *sql.DB
}

// Open creates or reuses the SQLite file at path and ensures the schema.
// An empty path yields an in-memory DB (useful in tests).
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)"
	if path == "" {
		dsn = "file::memory:?cache=shared&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("state: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	for _, p := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	} {
		// WAL is a no-op on :memory:; ignore the mode-setting result.
		_, _ = db.Exec(p)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("state: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS gosk_downloads (
    handle_id  TEXT    PRIMARY KEY,
    peer       TEXT    NOT NULL,
    filename   TEXT    NOT NULL,
    size       INTEGER NOT NULL,
    state      TEXT    NOT NULL,
    bytes      INTEGER NOT NULL DEFAULT 0,
    file_path  TEXT    NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
`

// Upsert writes a record, creating or replacing by HandleID.
func (s *Store) Upsert(ctx context.Context, r Record) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO gosk_downloads (handle_id, peer, filename, size, state, bytes, file_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(handle_id) DO UPDATE SET
		     state      = excluded.state,
		     bytes      = excluded.bytes,
		     file_path  = excluded.file_path,
		     updated_at = unixepoch()`,
		r.HandleID, r.Peer, r.Filename, r.Size, r.State, r.Bytes, r.FilePath,
	)
	if err != nil {
		return fmt.Errorf("state: upsert: %w", err)
	}
	return nil
}

// Get returns the record for handleID, or ErrNotFound.
func (s *Store) Get(ctx context.Context, handleID string) (Record, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT handle_id, peer, filename, size, state, bytes, file_path
		 FROM gosk_downloads WHERE handle_id = ?`, handleID)
	var r Record
	err := row.Scan(&r.HandleID, &r.Peer, &r.Filename, &r.Size, &r.State, &r.Bytes, &r.FilePath)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("state: get: %w", err)
	}
	return r, nil
}

// Delete removes a record.
func (s *Store) Delete(ctx context.Context, handleID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM gosk_downloads WHERE handle_id = ?`, handleID)
	if err != nil {
		return fmt.Errorf("state: delete: %w", err)
	}
	return nil
}

// All returns every record. Used on startup to reconcile in-flight downloads.
func (s *Store) All(ctx context.Context) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT handle_id, peer, filename, size, state, bytes, file_path
		 FROM gosk_downloads ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("state: list: %w", err)
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.HandleID, &r.Peer, &r.Filename, &r.Size, &r.State, &r.Bytes, &r.FilePath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
