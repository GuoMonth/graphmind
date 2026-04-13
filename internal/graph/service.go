package graph

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

// Service handles node and edge operations.
type Service struct {
	db    *sql.DB
	event *event.Service
}

// NewService creates a new graph service.
func NewService(db *sql.DB, eventSvc *event.Service) *Service {
	return &Service{db: db, event: eventSvc}
}

// CreateNodeInput defines the input for creating a node.
type CreateNodeInput struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Properties  map[string]any `json:"properties"` // JSON flexible properties
}

// CreateNode creates a new node within the given transaction.
func (s *Service) CreateNode(ctx context.Context, tx *sql.Tx, input CreateNodeInput) (*model.Node, error) {
	if !model.ValidNodeTypes[input.Type] {
		return nil, fmt.Errorf("%w: invalid node type %q", model.ErrInvalidInput, input.Type)
	}
	if input.Title == "" {
		return nil, fmt.Errorf("%w: title is required", model.ErrInvalidInput)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate node id: %w", err)
	}

	props := input.Properties
	if props == nil {
		props = map[string]any{} // JSON flexible properties — default empty
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO nodes (id, type, title, description, status, properties) VALUES (?, ?, ?, ?, ?, ?)`,
		id.String(), input.Type, input.Title, input.Description, input.Status, string(propsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("insert node: %w", err)
	}

	if err := s.event.Append(ctx, tx, "node", id.String(), model.ActionNodeCreated, input); err != nil {
		return nil, fmt.Errorf("append node_created event: %w", err)
	}

	return s.getNodeTx(ctx, tx, id.String())
}

// GetNode retrieves a node by ID.
func (s *Service) GetNode(ctx context.Context, id string) (*model.Node, error) {
	return s.scanNode(s.db.QueryRowContext(ctx,
		`SELECT id, type, title, description, status, properties, created_at, updated_at FROM nodes WHERE id = ?`, id,
	))
}

func (s *Service) getNodeTx(ctx context.Context, tx *sql.Tx, id string) (*model.Node, error) {
	return s.scanNode(tx.QueryRowContext(ctx,
		`SELECT id, type, title, description, status, properties, created_at, updated_at FROM nodes WHERE id = ?`, id,
	))
}

func (s *Service) scanNode(row *sql.Row) (*model.Node, error) {
	var n model.Node
	var propsJSON, createdAt, updatedAt string
	err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Description, &n.Status, &propsJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: node", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan node: %w", err)
	}

	if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
		return nil, fmt.Errorf("unmarshal node properties: %w", err)
	}
	if n.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if n.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}

	return &n, nil
}

// ListNodesFilter defines filters for listing nodes.
type ListNodesFilter struct {
	Type   string
	Status string
	Limit  int
	After  string
}

// ListNodes returns nodes matching the given filters.
func (s *Service) ListNodes(ctx context.Context, f ListNodesFilter) ([]model.Node, error) {
	query := "SELECT id, type, title, description, status, properties, created_at, updated_at FROM nodes WHERE 1=1"
	var args []any // sql scanning requires any

	if f.Type != "" {
		query += " AND type = ?"
		args = append(args, f.Type)
	}
	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, f.Status)
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
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	nodes := []model.Node{}
	for rows.Next() {
		var n model.Node
		var propsJSON, createdAt, updatedAt string
		err := rows.Scan(
			&n.ID, &n.Type, &n.Title, &n.Description,
			&n.Status, &propsJSON, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
			return nil, fmt.Errorf("unmarshal node properties: %w", err)
		}
		if n.CreatedAt, err = model.ParseTime(createdAt); err != nil {
			return nil, err
		}
		if n.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}

	return nodes, rows.Err()
}

// CreateEdgeInput defines the input for creating an edge.
type CreateEdgeInput struct {
	Type       string         `json:"type"`
	FromID     string         `json:"from_id"`
	ToID       string         `json:"to_id"`
	Properties map[string]any `json:"properties"` // JSON flexible properties
}

// CreateEdge creates a new edge within the given transaction.
func (s *Service) CreateEdge(ctx context.Context, tx *sql.Tx, input CreateEdgeInput) (*model.Edge, error) {
	if err := s.validateEdgeInput(ctx, tx, input); err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate edge id: %w", err)
	}

	props := input.Properties
	if props == nil {
		props = map[string]any{} // JSON flexible properties — default empty
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO edges (id, type, from_id, to_id, properties) VALUES (?, ?, ?, ?, ?)`,
		id.String(), input.Type, input.FromID, input.ToID, string(propsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrConflict, err)
	}

	if err := s.event.Append(ctx, tx, "edge", id.String(), model.ActionEdgeCreated, input); err != nil {
		return nil, fmt.Errorf("append edge_created event: %w", err)
	}

	return s.getEdgeTx(ctx, tx, id.String())
}

func (s *Service) validateEdgeInput(
	ctx context.Context, tx *sql.Tx, input CreateEdgeInput,
) error {
	if !model.ValidEdgeTypes[input.Type] {
		return fmt.Errorf("%w: invalid edge type %q", model.ErrInvalidInput, input.Type)
	}
	if input.FromID == "" || input.ToID == "" {
		return fmt.Errorf("%w: from_id and to_id are required", model.ErrInvalidInput)
	}
	if input.FromID == input.ToID {
		return fmt.Errorf("%w: self-referencing edge not allowed", model.ErrInvalidInput)
	}

	var exists int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM nodes WHERE id = ?", input.FromID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check from_id node: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("%w: from_id node does not exist", model.ErrNotFound)
	}
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM nodes WHERE id = ?", input.ToID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check to_id node: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("%w: to_id node does not exist", model.ErrNotFound)
	}

	if model.DirectionalEdgeTypes[input.Type] {
		return s.detectCycle(ctx, tx, input.Type, input.FromID, input.ToID)
	}
	return nil
}

// detectCycle checks if adding an edge from->to would create a cycle using recursive CTE.
func (s *Service) detectCycle(ctx context.Context, tx *sql.Tx, edgeType, fromID, toID string) error {
	// If toID can reach fromID via existing edges of the same type, adding from->to creates a cycle.
	var found int
	err := tx.QueryRowContext(ctx, `
		WITH RECURSIVE reachable(node_id) AS (
			SELECT ? 
			UNION
			SELECT e.to_id FROM edges e
			JOIN reachable r ON e.from_id = r.node_id
			WHERE e.type = ?
		)
		SELECT COUNT(*) FROM reachable WHERE node_id = ?
	`, toID, edgeType, fromID).Scan(&found)
	if err != nil {
		return fmt.Errorf("cycle detection query: %w", err)
	}
	if found > 0 {
		return fmt.Errorf("%w: edge would create a cycle", model.ErrConflict)
	}
	return nil
}

// GetEdge retrieves an edge by ID.
func (s *Service) GetEdge(ctx context.Context, id string) (*model.Edge, error) {
	return s.scanEdge(s.db.QueryRowContext(ctx,
		`SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM edges WHERE id = ?`, id,
	))
}

func (s *Service) getEdgeTx(ctx context.Context, tx *sql.Tx, id string) (*model.Edge, error) {
	return s.scanEdge(tx.QueryRowContext(ctx,
		`SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM edges WHERE id = ?`, id,
	))
}

func (s *Service) scanEdge(row *sql.Row) (*model.Edge, error) {
	var e model.Edge
	var propsJSON, createdAt, updatedAt string
	err := row.Scan(&e.ID, &e.Type, &e.FromID, &e.ToID, &propsJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: edge", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan edge: %w", err)
	}

	if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
		return nil, fmt.Errorf("unmarshal edge properties: %w", err)
	}
	if e.CreatedAt, err = model.ParseTime(createdAt); err != nil {
		return nil, err
	}
	if e.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
		return nil, err
	}

	return &e, nil
}

// ListEdgesFilter defines filters for listing edges.
type ListEdgesFilter struct {
	Type   string
	FromID string
	ToID   string
	Limit  int
	After  string
}

// ListEdges returns edges matching the given filters.
func (s *Service) ListEdges(ctx context.Context, f ListEdgesFilter) ([]model.Edge, error) {
	query := "SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM edges WHERE 1=1"
	var args []any // sql scanning requires any

	if f.Type != "" {
		query += " AND type = ?"
		args = append(args, f.Type)
	}
	if f.FromID != "" {
		query += " AND from_id = ?"
		args = append(args, f.FromID)
	}
	if f.ToID != "" {
		query += " AND to_id = ?"
		args = append(args, f.ToID)
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
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer rows.Close()

	edges := []model.Edge{}
	for rows.Next() {
		var e model.Edge
		var propsJSON, createdAt, updatedAt string
		if err := rows.Scan(&e.ID, &e.Type, &e.FromID, &e.ToID, &propsJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
			return nil, fmt.Errorf("unmarshal edge properties: %w", err)
		}
		if e.CreatedAt, err = model.ParseTime(createdAt); err != nil {
			return nil, err
		}
		if e.UpdatedAt, err = model.ParseTime(updatedAt); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}

	return edges, rows.Err()
}
