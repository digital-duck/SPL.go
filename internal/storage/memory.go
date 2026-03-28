// Package storage provides a SQLite-backed key-value memory store for SPL.
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS kv_store (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS prompt_cache (
    prompt_hash TEXT PRIMARY KEY,
    result      TEXT NOT NULL,
    model       TEXT,
    tokens_used INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT (datetime('now')),
    expires_at  DATETIME
);
`

// MemoryStore is a SQLite-backed key-value store.
type MemoryStore struct {
	db *sql.DB
}

// NewMemoryStore opens (or creates) a SQLite database at dbPath and initialises the schema.
func NewMemoryStore(dbPath string) (*MemoryStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("storage: create directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open db: %w", err)
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: init schema: %w", err)
	}
	return &MemoryStore{db: db}, nil
}

// DefaultMemoryStore uses ~/.spl/memory.db as the database path.
func DefaultMemoryStore() (*MemoryStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("storage: cannot determine home directory: %w", err)
	}
	return NewMemoryStore(filepath.Join(home, ".spl", "memory.db"))
}

// Get retrieves a value by key. Returns ("", false) if not found.
func (m *MemoryStore) Get(key string) (string, bool) {
	var value string
	err := m.db.QueryRow(`SELECT value FROM kv_store WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", false
	}
	return value, true
}

// Set inserts or updates a key-value pair.
func (m *MemoryStore) Set(key, value string) error {
	_, err := m.db.Exec(`
		INSERT INTO kv_store (key, value, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, key, value)
	if err != nil {
		return fmt.Errorf("storage: set %q: %w", key, err)
	}
	return nil
}

// Delete removes a key from the store. Returns nil if the key did not exist.
func (m *MemoryStore) Delete(key string) error {
	_, err := m.db.Exec(`DELETE FROM kv_store WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}
	return nil
}

// ListKeys returns all keys ordered by updated_at DESC.
func (m *MemoryStore) ListKeys() ([]string, error) {
	rows, err := m.db.Query(`SELECT key FROM kv_store ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage: list keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// CacheGet retrieves a cached prompt result if it exists and has not expired.
func (m *MemoryStore) CacheGet(promptHash string) (string, bool) {
	var result string
	err := m.db.QueryRow(`
		SELECT result FROM prompt_cache
		WHERE prompt_hash = ?
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
	`, promptHash).Scan(&result)
	if err != nil {
		return "", false
	}
	return result, true
}

// CacheSet stores a prompt result in the cache with optional expiry.
func (m *MemoryStore) CacheSet(promptHash, result, model string, tokensUsed int, expiresAt *time.Time) error {
	var expiresStr interface{}
	if expiresAt != nil {
		expiresStr = expiresAt.UTC().Format("2006-01-02 15:04:05")
	}
	_, err := m.db.Exec(`
		INSERT INTO prompt_cache (prompt_hash, result, model, tokens_used, created_at, expires_at)
		VALUES (?, ?, ?, ?, datetime('now'), ?)
		ON CONFLICT(prompt_hash) DO UPDATE SET
			result      = excluded.result,
			model       = excluded.model,
			tokens_used = excluded.tokens_used,
			created_at  = datetime('now'),
			expires_at  = excluded.expires_at
	`, promptHash, result, model, tokensUsed, expiresStr)
	if err != nil {
		return fmt.Errorf("storage: cache set %q: %w", promptHash, err)
	}
	return nil
}

// CacheList returns all entries from prompt_cache ordered by created_at DESC.
// Each entry is a map with keys: hash, model, tokens, created_at, expires_at.
func (m *MemoryStore) CacheList() ([]map[string]string, error) {
	rows, err := m.db.Query(`
		SELECT prompt_hash, COALESCE(model,''), CAST(tokens_used AS TEXT),
		       created_at, COALESCE(expires_at,'')
		FROM prompt_cache
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("storage: cache list: %w", err)
	}
	defer rows.Close()

	var result []map[string]string
	for rows.Next() {
		var hash, model, tokens, createdAt, expiresAt string
		if err := rows.Scan(&hash, &model, &tokens, &createdAt, &expiresAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{
			"hash":       hash,
			"model":      model,
			"tokens":     tokens,
			"created_at": createdAt,
			"expires_at": expiresAt,
		})
	}
	return result, rows.Err()
}

// CacheClear deletes all rows from the prompt_cache table.
// Returns the number of rows deleted.
func (m *MemoryStore) CacheClear() (int, error) {
	res, err := m.db.Exec(`DELETE FROM prompt_cache`)
	if err != nil {
		return 0, fmt.Errorf("storage: cache clear: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Close closes the underlying database connection.
func (m *MemoryStore) Close() error {
	return m.db.Close()
}
