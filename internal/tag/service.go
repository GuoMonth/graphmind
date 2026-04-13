package tag

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

// Service handles tag CRUD and node-tag associations.
type Service struct {
	db    *sql.DB
	event *event.Service
}

// NewService creates a new tag service.
func NewService(db *sql.DB, eventSvc *event.Service) *Service {
	return &Service{db: db, event: eventSvc}
}

// CreateTagInput defines the input for creating a tag.
type CreateTagInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateTag creates a new tag within the given transaction.
func (s *Service) CreateTag(ctx context.Context, tx *sql.Tx, input CreateTagInput) (*model.Tag, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: tag name is required", model.ErrInvalidInput)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate tag id: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tags (id, name, description) VALUES (?, ?, ?)`,
		id.String(), input.Name, input.Description,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: tag name %q already exists", model.ErrConflict, input.Name)
	}

	if err := s.event.Append(ctx, tx, "tag", id.String(), model.ActionTagCreated, input); err != nil {
		return nil, fmt.Errorf("append tag_created event: %w", err)
	}

	return s.getTagTx(ctx, tx, id.String())
}

// GetTagByName retrieves a tag by name.
func (s *Service) GetTagByName(ctx context.Context, tx *sql.Tx, name string) (*model.Tag, error) {
	var t model.Tag
	var createdAt, updatedAt string
	err := tx.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM tags WHERE name = ?`, name,
	).Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: tag", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
	t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
	return &t, nil
}

// GetTag retrieves a tag by ID.
func (s *Service) GetTag(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: tag", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
	t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
	return &t, nil
}

func (s *Service) getTagTx(ctx context.Context, tx *sql.Tx, id string) (*model.Tag, error) {
	var t model.Tag
	var createdAt, updatedAt string
	err := tx.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: tag", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
	t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
	return &t, nil
}

// NodeInput defines the input for tagging a node.
type NodeInput struct {
	NodeID      string `json:"node_id"`
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

// TagNode associates a tag with a node. Creates the tag if it doesn't exist (upsert).
func (s *Service) TagNode(ctx context.Context, tx *sql.Tx, input NodeInput) (*model.Tag, error) {
	if input.NodeID == "" {
		return nil, fmt.Errorf("%w: node_id is required", model.ErrInvalidInput)
	}
	if input.TagName == "" {
		return nil, fmt.Errorf("%w: tag_name is required", model.ErrInvalidInput)
	}

	// Verify node exists
	var nodeExists int
	err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM nodes WHERE id = ?", input.NodeID,
	).Scan(&nodeExists)
	if err != nil || nodeExists == 0 {
		return nil, fmt.Errorf("%w: node does not exist", model.ErrNotFound)
	}

	// Upsert tag: find or create
	t, err := s.GetTagByName(ctx, tx, input.TagName)
	if err != nil {
		// Tag doesn't exist, create it
		t, err = s.CreateTag(ctx, tx, CreateTagInput{
			Name:        input.TagName,
			Description: input.Description,
		})
		if err != nil {
			return nil, fmt.Errorf("create tag: %w", err)
		}
	}

	// Create association (ignore if already exists)
	_, err = tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO node_tags (node_id, tag_id) VALUES (?, ?)`,
		input.NodeID, t.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert node_tag: %w", err)
	}

	if err := s.event.Append(ctx, tx, "node", input.NodeID, model.ActionNodeTagged, map[string]string{
		"node_id":  input.NodeID,
		"tag_id":   t.ID,
		"tag_name": t.Name,
	}); err != nil {
		return nil, fmt.Errorf("append node_tagged event: %w", err)
	}

	return t, nil
}

// ListTagsFilter defines filters for listing tags.
type ListTagsFilter struct {
	Limit int
	After string
}

// ListTags returns tags matching the given filters.
func (s *Service) ListTags(ctx context.Context, f ListTagsFilter) ([]model.Tag, error) {
	query := "SELECT id, name, description, created_at, updated_at FROM tags WHERE 1=1"
	var args []any // sql scanning requires any

	if f.After != "" {
		query += " AND id > ?"
		args = append(args, f.After)
	}

	query += " ORDER BY name ASC"

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
		t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
		tags = append(tags, t)
	}

	return tags, rows.Err()
}
