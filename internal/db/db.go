// Package db provides the SQLite access layer for Speclite.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps an *sql.DB and exposes typed methods for all Speclite tables.
type DB struct {
	db *sql.DB
}

// Spec represents a single specification node stored in the specs table.
type Spec struct {
	ID        string
	Title     string
	Kind      string
	Status    string
	Version   int
	Body      string
	Hash      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Relation represents a directed typed edge between two spec nodes.
type Relation struct {
	FromID   string
	Relation string
	ToID     string
}

// Constraint represents a formal or semi-formal constraint attached to a spec.
type Constraint struct {
	ID         string
	TargetID   string
	Language   string
	Expression string
	CreatedAt  time.Time
}

// EventLog represents a single immutable audit log entry.
type EventLog struct {
	ID          int64
	EventType   string
	SpecID      *string
	PayloadJSON string
	CreatedAt   time.Time
}

// Open opens (or creates) the SQLite database at path, applies PRAGMA settings,
// and runs any pending migrations.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open %q: %w", path, err)
	}

	// SQLite works best with a single connection for WAL mode writes.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{db: sqlDB}

	if err := db.applyPragmas(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB for advanced use (e.g., transactions).
func (d *DB) SqlDB() *sql.DB {
	return d.db
}

// applyPragmas sets WAL journal mode and enables foreign keys.
func (d *DB) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := d.db.Exec(p); err != nil {
			return fmt.Errorf("db: pragma %q: %w", p, err)
		}
	}
	return nil
}

// userVersion reads the current PRAGMA user_version.
func (d *DB) userVersion() (int, error) {
	var v int
	row := d.db.QueryRow("PRAGMA user_version")
	if err := row.Scan(&v); err != nil {
		return 0, fmt.Errorf("db: read user_version: %w", err)
	}
	return v, nil
}

// migrate checks user_version and applies pending migrations.
func (d *DB) migrate() error {
	ver, err := d.userVersion()
	if err != nil {
		return err
	}

	if ver >= schemaVersion {
		return nil
	}

	// Apply v1 schema migration.
	if ver < 1 {
		if err := d.applySchemaV1(); err != nil {
			return err
		}
	}

	return nil
}

// applySchemaV1 executes the v1 DDL.
// modernc.org/sqlite supports multi-statement exec via the underlying C API.
// We execute the entire schema in one call to handle trigger bodies correctly.
func (d *DB) applySchemaV1() error {
	if _, err := d.db.Exec(schemaV1); err != nil {
		return fmt.Errorf("db: apply schema v1: %w", err)
	}
	return nil
}

// Init initialises a fresh database. It is equivalent to Open followed by
// appending an init_workspace event to the event log.
func (d *DB) Init() error {
	payload := fmt.Sprintf(`{"version":%d}`, schemaVersion)
	return d.AppendEvent("init_workspace", nil, payload)
}

// nowUTC returns the current UTC time formatted as ISO-8601.
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// parseTime parses an ISO-8601 UTC timestamp string.
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("db: parse time %q: %w", s, err)
	}
	return t, nil
}

