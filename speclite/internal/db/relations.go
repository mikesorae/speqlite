package db

import (
	"fmt"
)

// CreateRelation inserts a directed typed edge between two spec nodes.
// Returns an error if either spec does not exist or the relation type is invalid.
func (d *DB) CreateRelation(r Relation) error {
	_, err := d.db.Exec(
		`INSERT INTO relations (from_id, relation, to_id) VALUES (?, ?, ?)`,
		r.FromID, r.Relation, r.ToID,
	)
	if err != nil {
		return fmt.Errorf("db: create relation (%s -[%s]-> %s): %w", r.FromID, r.Relation, r.ToID, err)
	}
	return nil
}

// DeleteRelation removes a specific directed edge.
func (d *DB) DeleteRelation(fromID, relation, toID string) error {
	_, err := d.db.Exec(
		`DELETE FROM relations WHERE from_id = ? AND relation = ? AND to_id = ?`,
		fromID, relation, toID,
	)
	if err != nil {
		return fmt.Errorf("db: delete relation (%s -[%s]-> %s): %w", fromID, relation, toID, err)
	}
	return nil
}

// ListRelationsFrom returns all outgoing relations from a given spec.
func (d *DB) ListRelationsFrom(fromID string) ([]Relation, error) {
	rows, err := d.db.Query(
		`SELECT from_id, relation, to_id FROM relations WHERE from_id = ? ORDER BY relation, to_id`,
		fromID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list relations from %q: %w", fromID, err)
	}
	defer rows.Close()
	return scanRelations(rows)
}

// ListRelationsTo returns all incoming relations to a given spec.
func (d *DB) ListRelationsTo(toID string) ([]Relation, error) {
	rows, err := d.db.Query(
		`SELECT from_id, relation, to_id FROM relations WHERE to_id = ? ORDER BY relation, from_id`,
		toID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list relations to %q: %w", toID, err)
	}
	defer rows.Close()
	return scanRelations(rows)
}

// ListAllRelations returns every relation in the database.
func (d *DB) ListAllRelations() ([]Relation, error) {
	rows, err := d.db.Query(
		`SELECT from_id, relation, to_id FROM relations ORDER BY from_id, relation, to_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list all relations: %w", err)
	}
	defer rows.Close()
	return scanRelations(rows)
}

// RelationExists returns true if the specific directed edge already exists.
func (d *DB) RelationExists(fromID, relation, toID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(1) FROM relations WHERE from_id = ? AND relation = ? AND to_id = ?`,
		fromID, relation, toID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("db: relation exists: %w", err)
	}
	return count > 0, nil
}

func scanRelations(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]Relation, error) {
	var rels []Relation
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.FromID, &r.Relation, &r.ToID); err != nil {
			return nil, fmt.Errorf("db: scan relation: %w", err)
		}
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: relations rows: %w", err)
	}
	return rels, nil
}
