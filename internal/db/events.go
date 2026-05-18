package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// AppendEvent writes a single immutable event to the event_log table.
// specID may be nil for workspace-level events.
func (d *DB) AppendEvent(eventType string, specID *string, payloadJSON string) error {
	now := nowUTC()
	_, err := d.db.Exec(
		`INSERT INTO event_log (event_type, spec_id, payload_json, created_at)
		 VALUES (?, ?, ?, ?)`,
		eventType, specID, payloadJSON, now,
	)
	if err != nil {
		return fmt.Errorf("db: append event %q: %w", eventType, err)
	}
	return nil
}

// GetEvent retrieves a single event log entry by its auto-incremented ID.
func (d *DB) GetEvent(id int64) (EventLog, error) {
	row := d.db.QueryRow(
		`SELECT id, event_type, spec_id, payload_json, created_at FROM event_log WHERE id = ?`, id,
	)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EventLog{}, fmt.Errorf("db: get event %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return EventLog{}, fmt.Errorf("db: get event %d: %w", id, err)
	}
	return e, nil
}

// ListEvents returns all event log entries in chronological order.
// Pass a non-empty specID to filter by spec.
func (d *DB) ListEvents(specID string) ([]EventLog, error) {
	query := `SELECT id, event_type, spec_id, payload_json, created_at FROM event_log WHERE 1=1`
	args := []any{}

	if specID != "" {
		query += " AND spec_id = ?"
		args = append(args, specID)
	}
	query += " ORDER BY id ASC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: list events: %w", err)
	}
	defer rows.Close()

	var events []EventLog
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("db: list events scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list events rows: %w", err)
	}
	return events, nil
}

// ListEventsByType returns all events of a given type in chronological order.
func (d *DB) ListEventsByType(eventType string) ([]EventLog, error) {
	rows, err := d.db.Query(
		`SELECT id, event_type, spec_id, payload_json, created_at
		 FROM event_log WHERE event_type = ? ORDER BY id ASC`, eventType,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list events by type %q: %w", eventType, err)
	}
	defer rows.Close()

	var events []EventLog
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("db: list events by type scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list events by type rows: %w", err)
	}
	return events, nil
}

func scanEvent(row rowScanner) (EventLog, error) {
	var e EventLog
	var specID sql.NullString
	var createdAt string
	err := row.Scan(&e.ID, &e.EventType, &specID, &e.PayloadJSON, &createdAt)
	if err != nil {
		return EventLog{}, err
	}
	if specID.Valid {
		s := specID.String
		e.SpecID = &s
	}
	e.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return EventLog{}, err
	}
	return e, nil
}
