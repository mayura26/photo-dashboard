package persist

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	keyGroceries = "groceries"
	keyChores    = "chores"
	keyStars     = "stars"
	keyWeekHits  = "week_hits"
	keyNextGID   = "next_gid"
	keyGarmin    = "garmin"
	keyPhotos    = "photos"
)

// DB is a thin SQLite store for shopping/TODO mutable state.
type DB struct {
	sql *sql.DB
}

// Open creates the parent directory if needed, opens SQLite, and ensures schema.
func Open(path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("persist: empty db path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("persist: mkdir: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("persist: open: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("persist: pragma: %w", err)
	}
	if _, err := sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS kv (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("persist: migrate: %w", err)
	}
	return &DB{sql: sqlDB}, nil
}

// Close closes the underlying database.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// Mutable holds groceries, chores, stars, week hits, and the grocery id counter.
type Mutable struct {
	Groceries []map[string]any
	Chores    []map[string]any
	Stars     map[string]float64
	// WeekHits maps person id → YYYY-MM-DD → completed-all-chores that day.
	WeekHits map[string]map[string]bool
	NextGID  int
}

// LoadMutable returns persisted shopping/TODO state.
// ok is false when the DB has never been bootstrapped (missing groceries key).
func (d *DB) LoadMutable() (m Mutable, ok bool, err error) {
	raw, found, err := d.get(keyGroceries)
	if err != nil {
		return m, false, err
	}
	if !found {
		return m, false, nil
	}
	if err := json.Unmarshal(raw, &m.Groceries); err != nil {
		return m, false, fmt.Errorf("persist: groceries: %w", err)
	}
	if m.Groceries == nil {
		m.Groceries = []map[string]any{}
	}

	raw, found, err = d.get(keyChores)
	if err != nil {
		return m, false, err
	}
	if found {
		if err := json.Unmarshal(raw, &m.Chores); err != nil {
			return m, false, fmt.Errorf("persist: chores: %w", err)
		}
	}
	if m.Chores == nil {
		m.Chores = []map[string]any{}
	}

	raw, found, err = d.get(keyStars)
	if err != nil {
		return m, false, err
	}
	if found {
		if err := json.Unmarshal(raw, &m.Stars); err != nil {
			return m, false, fmt.Errorf("persist: stars: %w", err)
		}
	}
	if m.Stars == nil {
		m.Stars = map[string]float64{}
	}

	raw, found, err = d.get(keyWeekHits)
	if err != nil {
		return m, false, err
	}
	if found {
		if err := json.Unmarshal(raw, &m.WeekHits); err != nil {
			return m, false, fmt.Errorf("persist: week_hits: %w", err)
		}
	}
	if m.WeekHits == nil {
		m.WeekHits = map[string]map[string]bool{}
	}

	raw, found, err = d.get(keyNextGID)
	if err != nil {
		return m, false, err
	}
	if found {
		if err := json.Unmarshal(raw, &m.NextGID); err != nil {
			return m, false, fmt.Errorf("persist: next_gid: %w", err)
		}
	}
	if m.NextGID < 1 {
		m.NextGID = 100
	}
	return m, true, nil
}

// SaveMutable replaces groceries, chores, stars, week hits, and next_gid in one transaction.
func (d *DB) SaveMutable(m Mutable) error {
	if m.Groceries == nil {
		m.Groceries = []map[string]any{}
	}
	if m.Chores == nil {
		m.Chores = []map[string]any{}
	}
	if m.Stars == nil {
		m.Stars = map[string]float64{}
	}
	if m.WeekHits == nil {
		m.WeekHits = map[string]map[string]bool{}
	}
	if m.NextGID < 1 {
		m.NextGID = 100
	}

	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("persist: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	pairs := []struct {
		key string
		v   any
	}{
		{keyGroceries, m.Groceries},
		{keyChores, m.Chores},
		{keyStars, m.Stars},
		{keyWeekHits, m.WeekHits},
		{keyNextGID, m.NextGID},
	}
	for _, p := range pairs {
		b, err := json.Marshal(p.v)
		if err != nil {
			return fmt.Errorf("persist: marshal %s: %w", p.key, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO kv(key, value) VALUES(?, ?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			p.key, string(b),
		); err != nil {
			return fmt.Errorf("persist: upsert %s: %w", p.key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("persist: commit: %w", err)
	}
	return nil
}

// LoadGarmin returns the last ingested Garmin JSON blob.
// ok is false when no garmin key has been saved yet.
func (d *DB) LoadGarmin() (raw json.RawMessage, ok bool, err error) {
	b, found, err := d.get(keyGarmin)
	if err != nil || !found {
		return nil, found, err
	}
	if len(b) == 0 || !json.Valid(b) {
		return nil, false, fmt.Errorf("persist: garmin: invalid json")
	}
	return json.RawMessage(b), true, nil
}

// SaveGarmin replaces the persisted Garmin JSON blob.
func (d *DB) SaveGarmin(raw json.RawMessage) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return fmt.Errorf("persist: garmin: invalid json")
	}
	if _, err := d.sql.Exec(
		`INSERT INTO kv(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		keyGarmin, string(raw),
	); err != nil {
		return fmt.Errorf("persist: upsert garmin: %w", err)
	}
	return nil
}

// LoadPhotos returns persisted photo metadata.
// ok is false when no photos key has been saved yet.
func (d *DB) LoadPhotos() (photos []map[string]any, ok bool, err error) {
	raw, found, err := d.get(keyPhotos)
	if err != nil || !found {
		return nil, found, err
	}
	if err := json.Unmarshal(raw, &photos); err != nil {
		return nil, false, fmt.Errorf("persist: photos: %w", err)
	}
	if photos == nil {
		photos = []map[string]any{}
	}
	return photos, true, nil
}

// SavePhotos replaces the persisted photo metadata list.
func (d *DB) SavePhotos(photos []map[string]any) error {
	if photos == nil {
		photos = []map[string]any{}
	}
	b, err := json.Marshal(photos)
	if err != nil {
		return fmt.Errorf("persist: marshal photos: %w", err)
	}
	if _, err := d.sql.Exec(
		`INSERT INTO kv(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		keyPhotos, string(b),
	); err != nil {
		return fmt.Errorf("persist: upsert photos: %w", err)
	}
	return nil
}

func (d *DB) get(key string) ([]byte, bool, error) {
	var value string
	err := d.sql.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("persist: get %s: %w", key, err)
	}
	return []byte(value), true, nil
}
