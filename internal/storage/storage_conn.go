// Package storage provides persistent key-value backends for STORAGE-typed workflow params.
//
// Usage in SPL:
//
//	WORKFLOW my_wf
//	INPUT
//	  @memory STORAGE(sqlite, '.spl/memory.db')
//	DO
//	  @memory['profile'] := @result
//	  @profile          := @memory['profile']
//	END
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const kvTable = "spl_kv"
const kvInit = `
CREATE TABLE IF NOT EXISTS spl_kv (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
`

// StorageConn is a persistent key-value backend for a STORAGE-typed workflow variable.
type StorageConn struct {
	backend string
	db      *sql.DB
}

// OpenStorageConn opens (or creates) a storage backend.
// Supported backends: "sqlite" (full support), "duckdb"/"postgres" (stub — returns error).
func OpenStorageConn(backend, path string) (*StorageConn, error) {
	switch strings.ToLower(backend) {
	case "sqlite":
		return openSQLite(path)
	case "duckdb":
		return nil, fmt.Errorf("storage: DuckDB backend requires the go-duckdb CGO driver — install github.com/marcboeker/go-duckdb and rebuild")
	case "postgres", "postgresql":
		return nil, fmt.Errorf("storage: Postgres backend requires a driver — add github.com/lib/pq to go.mod and rebuild")
	default:
		return nil, fmt.Errorf("storage: unsupported backend %q (supported: sqlite, duckdb, postgres)", backend)
	}
}

func openSQLite(path string) (*StorageConn, error) {
	resolved := os.ExpandEnv(path)
	if strings.HasPrefix(resolved, "~/") {
		home, _ := os.UserHomeDir()
		resolved = filepath.Join(home, resolved[2:])
	}
	if dir := filepath.Dir(resolved); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("storage: create directory %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite %s: %w", resolved, err)
	}
	if _, err = db.Exec(kvInit); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: init schema: %w", err)
	}
	return &StorageConn{backend: "sqlite", db: db}, nil
}

// Get returns the value for key, or ("", false) if absent.
func (c *StorageConn) Get(key string) (string, bool) {
	var value string
	err := c.db.QueryRow(
		"SELECT value FROM "+kvTable+" WHERE key = ?", key,
	).Scan(&value)
	if err != nil {
		return "", false
	}
	return value, true
}

// Set upserts a key-value pair.
func (c *StorageConn) Set(key, value string) error {
	_, err := c.db.Exec(
		`INSERT INTO `+kvTable+` (key, value, updated_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET
		     value      = excluded.value,
		     updated_at = excluded.updated_at`,
		key, value,
	)
	return err
}

// Delete removes a key; returns true if a row was removed.
func (c *StorageConn) Delete(key string) (bool, error) {
	res, err := c.db.Exec("DELETE FROM "+kvTable+" WHERE key = ?", key)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListKeys returns all keys ordered by most-recently updated.
func (c *StorageConn) ListKeys() ([]string, error) {
	rows, err := c.db.Query("SELECT key FROM " + kvTable + " ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err = rows.Scan(&k); err != nil {
			return keys, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Close releases the underlying database connection.
func (c *StorageConn) Close() {
	if c.db != nil {
		c.db.Close()
	}
}
