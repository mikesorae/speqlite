package db

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Snapshot is the JSON representation of the current database state.
// It is written to .spec/state.snapshot.json after each apply.
type Snapshot struct {
	Version     int                  `json:"version"`
	GeneratedAt time.Time            `json:"generated_at"`
	Specs       []SnapshotSpec       `json:"specs"`
	Relations   []SnapshotRelation   `json:"relations"`
	Constraints []SnapshotConstraint `json:"constraints"`
}

// SnapshotSpec is the JSON-serialisable form of a Spec.
type SnapshotSpec struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Version   int    `json:"version"`
	Body      string `json:"body"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SnapshotRelation is the JSON-serialisable form of a Relation.
type SnapshotRelation struct {
	FromID   string `json:"from_id"`
	Relation string `json:"relation"`
	ToID     string `json:"to_id"`
}

// SnapshotConstraint is the JSON-serialisable form of a Constraint.
type SnapshotConstraint struct {
	ID         string `json:"id"`
	TargetID   string `json:"target_id"`
	Language   string `json:"language"`
	Expression string `json:"expression"`
	CreatedAt  string `json:"created_at"`
}

// TakeSnapshot reads the current state from the database and serialises it
// to the given path as a JSON file. The file is written atomically via a
// temporary file and rename.
func (d *DB) TakeSnapshot(path string) (*Snapshot, error) {
	snap, err := d.buildSnapshot()
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("db: marshal snapshot: %w", err)
	}

	// Write atomically: write to temp file then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("db: write snapshot tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("db: rename snapshot: %w", err)
	}

	return snap, nil
}

// LoadSnapshot reads a snapshot JSON file from disk.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("db: read snapshot %q: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("db: unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

// buildSnapshot reads all tables and builds a Snapshot value.
func (d *DB) buildSnapshot() (*Snapshot, error) {
	specs, err := d.ListSpecs("", "")
	if err != nil {
		return nil, fmt.Errorf("db: snapshot list specs: %w", err)
	}

	rels, err := d.ListAllRelations()
	if err != nil {
		return nil, fmt.Errorf("db: snapshot list relations: %w", err)
	}

	cons, err := d.ListAllConstraints()
	if err != nil {
		return nil, fmt.Errorf("db: snapshot list constraints: %w", err)
	}

	snap := &Snapshot{
		Version:     schemaVersion,
		GeneratedAt: time.Now().UTC(),
	}

	for _, s := range specs {
		snap.Specs = append(snap.Specs, SnapshotSpec{
			ID:        s.ID,
			Title:     s.Title,
			Kind:      s.Kind,
			Status:    s.Status,
			Version:   s.Version,
			Body:      s.Body,
			Hash:      s.Hash,
			CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	for _, r := range rels {
		snap.Relations = append(snap.Relations, SnapshotRelation{
			FromID:   r.FromID,
			Relation: r.Relation,
			ToID:     r.ToID,
		})
	}

	for _, c := range cons {
		snap.Constraints = append(snap.Constraints, SnapshotConstraint{
			ID:         c.ID,
			TargetID:   c.TargetID,
			Language:   c.Language,
			Expression: c.Expression,
			CreatedAt:  c.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	return snap, nil
}

// SnapshotHash returns a stable hash of the snapshot for change detection.
// It is computed as the SHA-256 of the sorted, minified JSON of all spec hashes.
func (snap *Snapshot) SnapshotHash() (string, error) {
	type entry struct {
		ID   string `json:"id"`
		Hash string `json:"hash"`
	}
	// Specs are already sorted by ID from ListSpecs (ORDER BY id).
	entries := make([]entry, 0, len(snap.Specs))
	for _, s := range snap.Specs {
		entries = append(entries, entry{ID: s.ID, Hash: s.Hash})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("db: snapshot hash marshal: %w", err)
	}
	return hexSHA256(data), nil
}

// hexSHA256 returns the lowercase hex-encoded SHA-256 digest of b.
func hexSHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// CanonicalHash computes the canonical hash for a spec node.
// The hash is the SHA-256 of: "{id}\n{title}\n{kind}\n{status}\n{body}".
func CanonicalHash(id, title, kind, status, body string) string {
	s := id + "\n" + title + "\n" + kind + "\n" + status + "\n" + body
	return hexSHA256([]byte(s))
}
