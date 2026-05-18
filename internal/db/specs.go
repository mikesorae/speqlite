package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a requested spec does not exist.
var ErrNotFound = errors.New("not found")

// CreateSpec inserts a new spec into the database and syncs FTS.
func (d *DB) CreateSpec(s Spec) error {
	if s.CreatedAt.IsZero() {
		now := nowUTC()
		s.CreatedAt, _ = parseTime(now)
		s.UpdatedAt = s.CreatedAt
	}

	_, err := d.db.Exec(
		`INSERT INTO specs (id, title, kind, status, version, body, hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Title, s.Kind, s.Status, s.Version, s.Body, s.Hash,
		s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		s.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return fmt.Errorf("db: create spec %q: %w", s.ID, err)
	}
	return nil
}

// GetSpec retrieves a single spec by ID.
func (d *DB) GetSpec(id string) (Spec, error) {
	row := d.db.QueryRow(
		`SELECT id, title, kind, status, version, body, hash, created_at, updated_at
		 FROM specs WHERE id = ?`, id,
	)
	s, err := scanSpec(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Spec{}, fmt.Errorf("db: get spec %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return Spec{}, fmt.Errorf("db: get spec %q: %w", id, err)
	}
	return s, nil
}

// ListSpecs returns all specs, optionally filtered by kind and/or status.
// Pass empty string to skip a filter.
func (d *DB) ListSpecs(kind, status string) ([]Spec, error) {
	query := `SELECT id, title, kind, status, version, body, hash, created_at, updated_at FROM specs WHERE 1=1`
	args := []any{}

	if kind != "" {
		query += " AND kind = ?"
		args = append(args, kind)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY id"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: list specs: %w", err)
	}
	defer rows.Close()

	var specs []Spec
	for rows.Next() {
		s, err := scanSpec(rows)
		if err != nil {
			return nil, fmt.Errorf("db: list specs scan: %w", err)
		}
		specs = append(specs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list specs rows: %w", err)
	}
	return specs, nil
}

// UpdateSpec updates an existing spec's mutable fields and increments version.
// The hash, title, kind, status, and body may all change.
func (d *DB) UpdateSpec(s Spec) error {
	now := nowUTC()
	_, err := d.db.Exec(
		`UPDATE specs
		 SET title = ?, kind = ?, status = ?, version = version + 1,
		     body = ?, hash = ?, updated_at = ?
		 WHERE id = ?`,
		s.Title, s.Kind, s.Status, s.Body, s.Hash, now, s.ID,
	)
	if err != nil {
		return fmt.Errorf("db: update spec %q: %w", s.ID, err)
	}
	return nil
}

// DeleteSpec removes a spec by ID. Outgoing relations are CASCADE deleted;
// incoming RESTRICT relations must be removed first.
func (d *DB) DeleteSpec(id string) error {
	_, err := d.db.Exec(`DELETE FROM specs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("db: delete spec %q: %w", id, err)
	}
	return nil
}

// SpecExists returns true if a spec with the given ID exists.
func (d *DB) SpecExists(id string) (bool, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(1) FROM specs WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("db: spec exists %q: %w", id, err)
	}
	return count > 0, nil
}

// scanSpec scans a single row from the specs table. The row argument accepts
// both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSpec(row rowScanner) (Spec, error) {
	var s Spec
	var createdAt, updatedAt string
	err := row.Scan(
		&s.ID, &s.Title, &s.Kind, &s.Status, &s.Version, &s.Body, &s.Hash,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return Spec{}, err
	}
	s.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Spec{}, err
	}
	s.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Spec{}, err
	}
	return s, nil
}
