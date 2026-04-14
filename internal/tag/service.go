package tag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
		return nil, fmt.Errorf("%w: %v", model.ErrConflict, err)
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: tag", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	if t.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if t.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTag retrieves a tag by ID.
func (s *Service) GetTag(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.WithHint(
			fmt.Errorf("%w: tag", model.ErrNotFound),
			"Use 'gm ls tag' to list existing tags.",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	if t.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if t.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) getTagTx(ctx context.Context, tx *sql.Tx, id string) (*model.Tag, error) {
	var t model.Tag
	var createdAt, updatedAt string
	err := tx.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.WithHint(
			fmt.Errorf("%w: tag", model.ErrNotFound),
			"Use 'gm ls tag' to list existing tags.",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag: %w", err)
	}
	if t.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if t.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}
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
	if err != nil {
		return nil, fmt.Errorf("check node exists: %w", err)
	}
	if nodeExists == 0 {
		return nil, model.WithHint(
			fmt.Errorf("%w: node does not exist", model.ErrNotFound),
			fmt.Sprintf("Node %q not found. Use 'gm ls node' to list nodes.", input.NodeID),
		)
	}

	// Upsert tag: find or create
	t, err := s.GetTagByName(ctx, tx, input.TagName)
	if err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return nil, fmt.Errorf("get tag by name: %w", err)
		}
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
		query += " AND name > ?"
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

	tags := []model.Tag{}
	for rows.Next() {
		var t model.Tag
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		if t.CreatedAt, err = model.ParseTime(createdAt); err != nil {
			return nil, err
		}
		if t.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}

	return tags, rows.Err()
}

// CreateTagEdgeInput defines the input for creating a tag edge.
type CreateTagEdgeInput struct {
	Type       string         `json:"type"`
	FromID     string         `json:"from_id"`
	ToID       string         `json:"to_id"`
	Properties map[string]any `json:"properties"` // JSON flexible properties
}

// CreateTagEdge creates a directed edge between two tags within the given transaction.
func (s *Service) CreateTagEdge(ctx context.Context, tx *sql.Tx, input CreateTagEdgeInput) (*model.TagEdge, error) {
	if input.Type == "" {
		return nil, model.WithHint(
			fmt.Errorf("%w: tag edge type is required", model.ErrInvalidInput),
			"Provide --type <type>. Common types: parent_of, synonym_of, related_to, opposite_of.",
		)
	}
	if input.FromID == "" || input.ToID == "" {
		return nil, model.WithHint(
			fmt.Errorf("%w: from_id and to_id are required", model.ErrInvalidInput),
			"Usage: gm ln <tag-id> <tag-id> --type <edge-type>",
		)
	}
	if input.FromID == input.ToID {
		return nil, model.WithHint(
			fmt.Errorf("%w: self-referencing tag edge not allowed", model.ErrInvalidInput),
			"The from_id and to_id must be different tags.",
		)
	}

	// Verify both tags exist
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE id = ?", input.FromID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check from_id tag: %w", err)
	}
	if exists == 0 {
		return nil, model.WithHint(
			fmt.Errorf("%w: from_id tag does not exist", model.ErrNotFound),
			fmt.Sprintf("Tag %q not found. Use 'gm ls tag' to list tags.", input.FromID),
		)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE id = ?", input.ToID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check to_id tag: %w", err)
	}
	if exists == 0 {
		return nil, model.WithHint(
			fmt.Errorf("%w: to_id tag does not exist", model.ErrNotFound),
			fmt.Sprintf("Tag %q not found. Use 'gm ls tag' to list tags.", input.ToID),
		)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate tag edge id: %w", err)
	}

	props := input.Properties
	if props == nil {
		props = map[string]any{} // default empty
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tag_edges (id, type, from_id, to_id, properties) VALUES (?, ?, ?, ?, ?)`,
		id.String(), input.Type, input.FromID, input.ToID, string(propsJSON),
	)
	if err != nil {
		return nil, model.WithHint(
			fmt.Errorf("%w: %v", model.ErrConflict, err),
			"A tag edge with the same type between these tags may already exist. Use 'gm ls tag_edge' to check.",
		)
	}

	if err := s.event.Append(ctx, tx, "tag_edge", id.String(), model.ActionTagEdgeCreated, input); err != nil {
		return nil, fmt.Errorf("append tag_edge_created event: %w", err)
	}

	return s.getTagEdgeTx(ctx, tx, id.String())
}

// GetTagEdge retrieves a tag edge by ID.
func (s *Service) GetTagEdge(ctx context.Context, id string) (*model.TagEdge, error) {
	return s.scanTagEdge(s.db.QueryRowContext(ctx,
		`SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM tag_edges WHERE id = ?`, id,
	))
}

func (s *Service) getTagEdgeTx(ctx context.Context, tx *sql.Tx, id string) (*model.TagEdge, error) {
	return s.scanTagEdge(tx.QueryRowContext(ctx,
		`SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM tag_edges WHERE id = ?`, id,
	))
}

func (s *Service) scanTagEdge(row *sql.Row) (*model.TagEdge, error) {
	var e model.TagEdge
	var propsJSON, createdAt, updatedAt string
	err := row.Scan(&e.ID, &e.Type, &e.FromID, &e.ToID, &propsJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.WithHint(
			fmt.Errorf("%w: tag_edge", model.ErrNotFound),
			"Use 'gm ls tag_edge' to list existing tag edges.",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("scan tag edge: %w", err)
	}
	if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
		return nil, fmt.Errorf("unmarshal tag edge properties: %w", err)
	}
	if e.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if e.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

// DeleteTagEdge removes a tag edge by ID within the given transaction.
func (s *Service) DeleteTagEdge(ctx context.Context, tx *sql.Tx, id string) error {
	if id == "" {
		return model.WithHint(
			fmt.Errorf("%w: id is required", model.ErrInvalidInput),
			"Provide the tag edge UUID. Use 'gm ls tag_edge' to find IDs.",
		)
	}

	if _, err := s.getTagEdgeTx(ctx, tx, id); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM tag_edges WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete tag edge: %w", err)
	}

	if err := s.event.Append(ctx, tx, "tag_edge", id, model.ActionTagEdgeDeleted, map[string]string{"id": id}); err != nil {
		return fmt.Errorf("append tag_edge_deleted event: %w", err)
	}

	return nil
}

// ListTagEdgesFilter defines filters for listing tag edges.
type ListTagEdgesFilter struct {
	Type  string
	Limit int
	After string
}

// ListTagEdges returns tag edges matching the given filters.
func (s *Service) ListTagEdges(ctx context.Context, f ListTagEdgesFilter) ([]model.TagEdge, error) {
	query := "SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM tag_edges WHERE 1=1"
	var args []any // sql scanning requires any

	if f.Type != "" {
		query += " AND type = ?"
		args = append(args, f.Type)
	}
	if f.After != "" {
		query += " AND id < ?"
		args = append(args, f.After)
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
		return nil, fmt.Errorf("query tag edges: %w", err)
	}
	defer rows.Close()

	tagEdges := []model.TagEdge{}
	for rows.Next() {
		var e model.TagEdge
		var propsJSON, createdAt, updatedAt string
		if err := rows.Scan(&e.ID, &e.Type, &e.FromID, &e.ToID, &propsJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag edge: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
			return nil, fmt.Errorf("unmarshal tag edge properties: %w", err)
		}
		if e.CreatedAt, err = model.ParseTime(createdAt); err != nil {
			return nil, err
		}
		if e.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
			return nil, err
		}
		tagEdges = append(tagEdges, e)
	}

	return tagEdges, rows.Err()
}
