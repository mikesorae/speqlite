package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// CreateConstraint inserts a new constraint attached to a spec node.
func (d *DB) CreateConstraint(c Constraint) error {
	if c.CreatedAt.IsZero() {
		now := nowUTC()
		c.CreatedAt, _ = parseTime(now)
	}

	_, err := d.db.Exec(
		`INSERT INTO constraints (id, target_id, language, expression, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.TargetID, c.Language, c.Expression,
		c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return fmt.Errorf("db: create constraint %q: %w", c.ID, err)
	}
	return nil
}

// GetConstraint retrieves a single constraint by its ID.
func (d *DB) GetConstraint(id string) (Constraint, error) {
	row := d.db.QueryRow(
		`SELECT id, target_id, language, expression, created_at FROM constraints WHERE id = ?`, id,
	)
	c, err := scanConstraint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Constraint{}, fmt.Errorf("db: get constraint %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return Constraint{}, fmt.Errorf("db: get constraint %q: %w", id, err)
	}
	return c, nil
}

// ListConstraints returns all constraints for the given target spec ID.
func (d *DB) ListConstraints(targetID string) ([]Constraint, error) {
	rows, err := d.db.Query(
		`SELECT id, target_id, language, expression, created_at
		 FROM constraints WHERE target_id = ? ORDER BY created_at`, targetID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list constraints for %q: %w", targetID, err)
	}
	defer rows.Close()

	var cs []Constraint
	for rows.Next() {
		c, err := scanConstraint(rows)
		if err != nil {
			return nil, fmt.Errorf("db: list constraints scan: %w", err)
		}
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list constraints rows: %w", err)
	}
	return cs, nil
}

// ListAllConstraints returns every constraint in the database.
func (d *DB) ListAllConstraints() ([]Constraint, error) {
	rows, err := d.db.Query(
		`SELECT id, target_id, language, expression, created_at
		 FROM constraints ORDER BY target_id, created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list all constraints: %w", err)
	}
	defer rows.Close()

	var cs []Constraint
	for rows.Next() {
		c, err := scanConstraint(rows)
		if err != nil {
			return nil, fmt.Errorf("db: list all constraints scan: %w", err)
		}
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list all constraints rows: %w", err)
	}
	return cs, nil
}

// DeleteConstraint removes a constraint by ID.
func (d *DB) DeleteConstraint(id string) error {
	_, err := d.db.Exec(`DELETE FROM constraints WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("db: delete constraint %q: %w", id, err)
	}
	return nil
}

func scanConstraint(row rowScanner) (Constraint, error) {
	var c Constraint
	var createdAt string
	err := row.Scan(&c.ID, &c.TargetID, &c.Language, &c.Expression, &createdAt)
	if err != nil {
		return Constraint{}, err
	}
	c.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Constraint{}, err
	}
	return c, nil
}
