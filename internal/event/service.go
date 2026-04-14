package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

// Service handles event append and query operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new event service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Append records an event within the given transaction.
func (s *Service) Append(ctx context.Context, tx *sql.Tx, entityType, entityID, action string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate event id: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, entity_type, entity_id, action, payload) VALUES (?, ?, ?, ?, ?)`,
		id.String(), entityType, entityID, action, string(payloadJSON),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

// ListFilter defines filters for listing events.
type ListFilter struct {
	EntityType string
	EntityID   string
	Action     string
	Since      string // ISO 8601 timestamp or Go duration string
	Limit      int
	After      string
}

// List returns events matching the given filters.
func (s *Service) List(ctx context.Context, f *ListFilter) ([]model.Event, error) {
	query := "SELECT id, entity_type, entity_id, action, payload, created_at FROM events WHERE 1=1"
	var args []any // sql scanning requires any

	if f.EntityType != "" {
		query += " AND entity_type = ?"
		args = append(args, f.EntityType)
	}
	if f.EntityID != "" {
		query += " AND entity_id = ?"
		args = append(args, f.EntityID)
	}
	if f.Action != "" {
		query += " AND action = ?"
		args = append(args, f.Action)
	}
	if f.After != "" {
		// Cursor pagination: UUIDv7 IDs are time-ordered (RFC 9562).
		query += " AND id < ?"
		args = append(args, f.After)
	}
	if f.Since != "" {
		query += " AND created_at >= ?"
		args = append(args, f.Since)
	}

	query += " ORDER BY created_at DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	events := []model.Event{}
	for rows.Next() {
		var e model.Event
		var createdAt string
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &e.Payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if e.CreatedAt, err = model.ParseTime(createdAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}
