package db

import (
	"fmt"
)

// SearchResult represents a single FTS5 search result with BM25 rank.
type SearchResult struct {
	Spec
	Rank float64
}

// SearchSpecs performs a full-text search over specs using FTS5 with BM25 ranking.
// kind and status may be empty to skip those filters.
func (d *DB) SearchSpecs(query, kind, status string) ([]SearchResult, error) {
	// Build the base FTS query. BM25 assigns negative scores; ORDER BY rank ASC
	// gives best matches first.
	sql := `
		SELECT s.id, s.title, s.kind, s.status, s.version, s.body, s.hash,
		       s.created_at, s.updated_at,
		       bm25(specs_fts) AS rank
		FROM specs_fts
		JOIN specs s ON specs_fts.id = s.id
		WHERE specs_fts MATCH ?`
	args := []any{query}

	if kind != "" {
		sql += " AND s.kind = ?"
		args = append(args, kind)
	}
	if status != "" {
		sql += " AND s.status = ?"
		args = append(args, status)
	}
	sql += " ORDER BY rank"

	rows, err := d.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("db: search specs %q: %w", query, err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var createdAt, updatedAt string
		err := rows.Scan(
			&r.ID, &r.Title, &r.Kind, &r.Status, &r.Version, &r.Body, &r.Hash,
			&createdAt, &updatedAt,
			&r.Rank,
		)
		if err != nil {
			return nil, fmt.Errorf("db: search scan: %w", err)
		}
		r.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("db: search parse created_at: %w", err)
		}
		r.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("db: search parse updated_at: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: search rows: %w", err)
	}
	return results, nil
}
