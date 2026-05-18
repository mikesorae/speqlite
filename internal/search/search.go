// Package search provides FTS5 BM25-ranked full-text search over specs.
package search

import (
	"fmt"

	"github.com/mikesorae/speqlite/internal/db"
)

// Result is a single search hit with its BM25 rank.
type Result struct {
	Spec db.Spec
	Rank float64
}

// Options carries optional filters for a search query.
type Options struct {
	// Kind filters results to a specific spec kind (empty = no filter).
	Kind string
	// Status filters results to a specific spec status (empty = no filter).
	Status string
}

// Search runs a FTS5 full-text query against the specs_fts virtual table using
// BM25 ranking. Results are returned best-first (lowest BM25 score = best match).
func Search(database *db.DB, query string, opts Options) ([]Result, error) {
	if query == "" {
		return nil, fmt.Errorf("search: query must not be empty")
	}

	hits, err := database.SearchSpecs(query, opts.Kind, opts.Status)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := make([]Result, 0, len(hits))
	for _, h := range hits {
		results = append(results, Result{
			Spec: h.Spec,
			Rank: h.Rank,
		})
	}
	return results, nil
}
