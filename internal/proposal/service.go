package proposal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
)

// Service handles proposal create, commit, and reject.
type Service struct {
	db    *sql.DB
	event *event.Service
	graph *graph.Service
	tag   *tag.Service
}

// NewService creates a new proposal service.
func NewService(db *sql.DB, eventSvc *event.Service, graphSvc *graph.Service, tagSvc *tag.Service) *Service {
	return &Service{db: db, event: eventSvc, graph: graphSvc, tag: tagSvc}
}

// Create creates a pending proposal with the given operations.
func (s *Service) Create(ctx context.Context, operations []model.ProposalOperation) (*model.Proposal, error) {
	if len(operations) == 0 {
		return nil, fmt.Errorf("%w: proposal must have at least one operation", model.ErrInvalidInput)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate proposal id: %w", err)
	}

	opsJSON, err := json.Marshal(operations)
	if err != nil {
		return nil, fmt.Errorf("marshal operations: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO proposals (id, status, operations) VALUES (?, ?, ?)`,
		id.String(), model.ProposalStatusPending, string(opsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("insert proposal: %w", err)
	}

	if err := s.event.Append(ctx, tx, "proposal", id.String(),
		model.ActionProposalCreated, map[string]any{ // event payload requires any
			"operation_count": len(operations),
		}); err != nil {
		return nil, fmt.Errorf("append proposal_created event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.Get(ctx, id.String())
}

// Commit applies all operations in a pending proposal atomically.
func (s *Service) Commit(ctx context.Context, proposalID string) (*model.Proposal, error) {
	p, err := s.Get(ctx, proposalID)
	if err != nil {
		return nil, err
	}

	if p.Status != model.ProposalStatusPending {
		return nil, fmt.Errorf("%w: proposal is %s, not pending", model.ErrInvalidState, p.Status)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Track created entity IDs for internal references
	createdIDs := make(map[int]string)

	for i, op := range p.Operations {
		entityID, err := s.applyOperation(ctx, tx, op, createdIDs)
		if err != nil {
			return nil, fmt.Errorf("operation %d (%s): %w", i, op.Action, err)
		}
		if entityID != "" {
			createdIDs[i] = entityID
		}
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err = tx.ExecContext(ctx,
		`UPDATE proposals SET status = ?, updated_at = ? WHERE id = ?`,
		model.ProposalStatusCommitted, now, proposalID,
	)
	if err != nil {
		return nil, fmt.Errorf("update proposal status: %w", err)
	}

	if err := s.event.Append(ctx, tx, "proposal", proposalID, model.ActionProposalCommitted, nil); err != nil {
		return nil, fmt.Errorf("append proposal_committed event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.Get(ctx, proposalID)
}

// Reject marks a pending proposal as rejected.
func (s *Service) Reject(ctx context.Context, proposalID string) (*model.Proposal, error) {
	p, err := s.Get(ctx, proposalID)
	if err != nil {
		return nil, err
	}

	if p.Status != model.ProposalStatusPending {
		return nil, fmt.Errorf("%w: proposal is %s, not pending", model.ErrInvalidState, p.Status)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err = tx.ExecContext(ctx,
		`UPDATE proposals SET status = ?, updated_at = ? WHERE id = ?`,
		model.ProposalStatusRejected, now, proposalID,
	)
	if err != nil {
		return nil, fmt.Errorf("update proposal status: %w", err)
	}

	if err := s.event.Append(ctx, tx, "proposal", proposalID, model.ActionProposalRejected, nil); err != nil {
		return nil, fmt.Errorf("append proposal_rejected event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.Get(ctx, proposalID)
}

func (s *Service) applyOperation(
	ctx context.Context, tx *sql.Tx, op model.ProposalOperation, createdIDs map[int]string,
) (string, error) {
	switch op.Action {
	case model.OpCreateNode:
		return s.applyCreateNode(ctx, tx, op.Data)
	case model.OpCreateEdge:
		return s.applyCreateEdge(ctx, tx, op.Data, createdIDs)
	case model.OpTagNode:
		return s.applyTagNode(ctx, tx, op.Data, createdIDs)
	default:
		return "", fmt.Errorf("%w: unknown operation action %q", model.ErrInvalidInput, op.Action)
	}
}

func (s *Service) applyCreateNode(ctx context.Context, tx *sql.Tx, data map[string]any) (string, error) {
	input := graph.CreateNodeInput{
		Type:        getString(data, "type"),
		Title:       getString(data, "title"),
		Description: getString(data, "description"),
		Status:      getString(data, "status"),
	}
	if props, ok := data["properties"]; ok {
		if m, ok := props.(map[string]any); ok { // JSON properties require any
			input.Properties = m
		}
	}

	node, err := s.graph.CreateNode(ctx, tx, input)
	if err != nil {
		return "", err
	}
	return node.ID, nil
}

func (s *Service) applyCreateEdge(
	ctx context.Context, tx *sql.Tx, data map[string]any, createdIDs map[int]string,
) (string, error) {
	input := graph.CreateEdgeInput{
		Type:   getString(data, "type"),
		FromID: getString(data, "from_id"),
		ToID:   getString(data, "to_id"),
	}

	// Resolve internal references
	if ref, ok := getInt(data, "from_reference"); ok && input.FromID == "" {
		if id, exists := createdIDs[ref]; exists {
			input.FromID = id
		} else {
			return "", fmt.Errorf("%w: from_reference %d not yet created", model.ErrInvalidInput, ref)
		}
	}
	if ref, ok := getInt(data, "to_reference"); ok && input.ToID == "" {
		if id, exists := createdIDs[ref]; exists {
			input.ToID = id
		} else {
			return "", fmt.Errorf("%w: to_reference %d not yet created", model.ErrInvalidInput, ref)
		}
	}

	if props, ok := data["properties"]; ok {
		if m, ok := props.(map[string]any); ok { // JSON properties require any
			input.Properties = m
		}
	}

	edge, err := s.graph.CreateEdge(ctx, tx, input)
	if err != nil {
		return "", err
	}
	return edge.ID, nil
}

func (s *Service) applyTagNode(
	ctx context.Context, tx *sql.Tx, data map[string]any, createdIDs map[int]string,
) (string, error) {
	input := tag.NodeInput{
		NodeID:      getString(data, "node_id"),
		TagName:     getString(data, "tag_name"),
		Description: getString(data, "description"),
	}

	// Resolve internal reference for node_id
	if ref, ok := getInt(data, "reference"); ok && input.NodeID == "" {
		if id, exists := createdIDs[ref]; exists {
			input.NodeID = id
		} else {
			return "", fmt.Errorf("%w: reference %d not yet created", model.ErrInvalidInput, ref)
		}
	}

	t, err := s.tag.TagNode(ctx, tx, input)
	if err != nil {
		return "", err
	}
	return t.ID, nil
}

// Get retrieves a proposal by ID.
func (s *Service) Get(ctx context.Context, id string) (*model.Proposal, error) {
	var p model.Proposal
	var opsJSON, createdAt, updatedAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, status, operations, created_at, updated_at FROM proposals WHERE id = ?`, id,
	).Scan(&p.ID, &p.Status, &opsJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: proposal", model.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scan proposal: %w", err)
	}

	if err := json.Unmarshal([]byte(opsJSON), &p.Operations); err != nil {
		return nil, fmt.Errorf("unmarshal operations: %w", err)
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)

	return &p, nil
}

// ListFilter defines filters for listing proposals.
type ListFilter struct {
	Status string
	Limit  int
	After  string
}

// List returns proposals matching the given filters.
func (s *Service) List(ctx context.Context, f ListFilter) ([]model.Proposal, error) {
	query := "SELECT id, status, operations, created_at, updated_at FROM proposals WHERE 1=1"
	var args []any // sql scanning requires any

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
		return nil, fmt.Errorf("query proposals: %w", err)
	}
	defer rows.Close()

	var proposals []model.Proposal
	for rows.Next() {
		var p model.Proposal
		var opsJSON, createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Status, &opsJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		if err := json.Unmarshal([]byte(opsJSON), &p.Operations); err != nil {
			return nil, fmt.Errorf("unmarshal operations: %w", err)
		}
		p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", createdAt)
		p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
		proposals = append(proposals, p)
	}

	return proposals, rows.Err()
}

// Helper to extract string from map[string]any
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Helper to extract int from map[string]any (JSON numbers are float64)
func getInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
}
