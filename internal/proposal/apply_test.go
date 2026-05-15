package proposal_test

// Tests for previously zero-covered apply* functions:
//   applyUpdateNode, applyDeleteNode, applyDeleteEdge,
//   applyCreateTagEdge, applyDeleteTagEdge
// and getInt (float64 / json.Number code paths).

import (
	"errors"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/proposal"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
)

// ---------------------------------------------------------------------------
// applyUpdateNode
// ---------------------------------------------------------------------------

func TestApplyUpdateNodeAllFields(t *testing.T) {
	env := setup(t)
	nodeID := env.createAndCommitNode(t, "event", "Original title")

	p, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpUpdateNode,
		Entity: "node",
		Data: map[string]any{
			"id":          nodeID,
			"title":       "Updated title",
			"description": "New description",
			"status":      "done",
			"type":        "decision",
			"who":         "Alice",
			"where":       "Berlin",
			"event_time":  "2025-06-01",
		},
	}})
	if err != nil {
		t.Fatalf("Create proposal: %v", err)
	}

	committed, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}

	node, err := env.graph.GetNode(env.ctx, nodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Title != "Updated title" {
		t.Errorf("title = %q, want %q", node.Title, "Updated title")
	}
	if node.Status != "done" {
		t.Errorf("status = %q, want done", node.Status)
	}
	if node.Type != "decision" {
		t.Errorf("type = %q, want decision", node.Type)
	}
	if node.Who != "Alice" {
		t.Errorf("who = %q, want Alice", node.Who)
	}
	if node.Where != "Berlin" {
		t.Errorf("where = %q, want Berlin", node.Where)
	}
	if node.EventTime != "2025-06-01" {
		t.Errorf("event_time = %q, want 2025-06-01", node.EventTime)
	}
}

func TestApplyUpdateNodeMissingID(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpUpdateNode,
		Entity: "node",
		Data:   map[string]any{"title": "No ID"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("error type = %v, want ErrInvalidInput", err)
	}
}

func TestApplyUpdateNodeNotFound(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpUpdateNode,
		Entity: "node",
		Data:   map[string]any{"id": "00000000-0000-0000-0000-000000000000"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// Partial update: only status changed, other fields preserved.
func TestApplyUpdateNodePartial(t *testing.T) {
	env := setup(t)
	nodeID := env.createAndCommitNode(t, "event", "Keep title")

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpUpdateNode,
		Entity: "node",
		Data:   map[string]any{"id": nodeID, "status": "archived"},
	}})
	env.proposal.Commit(env.ctx, p.ID)

	node, err := env.graph.GetNode(env.ctx, nodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Title != "Keep title" {
		t.Errorf("title changed unexpectedly: %q", node.Title)
	}
	if node.Status != "archived" {
		t.Errorf("status = %q, want archived", node.Status)
	}
}

// ---------------------------------------------------------------------------
// applyDeleteNode
// ---------------------------------------------------------------------------

func TestApplyDeleteNode(t *testing.T) {
	env := setup(t)
	nodeID := env.createAndCommitNode(t, "event", "To delete")

	p, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteNode,
		Entity: "node",
		Data:   map[string]any{"id": nodeID},
	}})
	if err != nil {
		t.Fatalf("Create proposal: %v", err)
	}
	committed, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}

	_, err = env.graph.GetNode(env.ctx, nodeID)
	if err == nil {
		t.Fatal("expected not-found after delete, got nil")
	}
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("error type = %v, want ErrNotFound", err)
	}
}

func TestApplyDeleteNodeCascadesEdge(t *testing.T) {
	env := setup(t)
	fromID := env.createAndCommitNode(t, "event", "Source")
	toID := env.createAndCommitNode(t, "event", "Target")

	// Create edge
	ep, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateEdge,
		Entity: "edge",
		Data: map[string]any{
			"type":    "caused_by",
			"from_id": fromID,
			"to_id":   toID,
		},
	}})
	env.proposal.Commit(env.ctx, ep.ID)

	// Delete source node — edge should cascade
	dp, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteNode,
		Entity: "node",
		Data:   map[string]any{"id": fromID},
	}})
	env.proposal.Commit(env.ctx, dp.ID)

	edges, err := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("edges = %d after node cascade delete, want 0", len(edges))
	}
}

func TestApplyDeleteNodeMissingID(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteNode,
		Entity: "node",
		Data:   map[string]any{},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
}

func TestApplyDeleteNodeNotFound(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteNode,
		Entity: "node",
		Data:   map[string]any{"id": "00000000-0000-0000-0000-000000000000"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// ---------------------------------------------------------------------------
// applyDeleteEdge
// ---------------------------------------------------------------------------

func setupEdge(t *testing.T, env *testEnv, edgeType string) string {
	t.Helper()
	fromID := env.createAndCommitNode(t, "event", "From "+edgeType)
	toID := env.createAndCommitNode(t, "event", "To "+edgeType)

	ep, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateEdge,
		Entity: "edge",
		Data: map[string]any{
			"type":    edgeType,
			"from_id": fromID,
			"to_id":   toID,
		},
	}})
	env.proposal.Commit(env.ctx, ep.ID)

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Type: edgeType})
	if len(edges) == 0 {
		t.Fatalf("expected edge to exist after commit")
	}
	return edges[0].ID
}

func TestApplyDeleteEdge(t *testing.T) {
	env := setup(t)
	edgeID := setupEdge(t, env, "related_to")

	dp, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteEdge,
		Entity: "edge",
		Data:   map[string]any{"id": edgeID},
	}})
	if err != nil {
		t.Fatalf("Create delete-edge proposal: %v", err)
	}
	committed, err := env.proposal.Commit(env.ctx, dp.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Type: "related_to"})
	if len(edges) != 0 {
		t.Errorf("edges = %d after delete, want 0", len(edges))
	}
}

func TestApplyDeleteEdgeMissingID(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteEdge,
		Entity: "edge",
		Data:   map[string]any{},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
}

func TestApplyDeleteEdgeNotFound(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteEdge,
		Entity: "edge",
		Data:   map[string]any{"id": "00000000-0000-0000-0000-000000000000"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// ---------------------------------------------------------------------------
// applyCreateTagEdge
// ---------------------------------------------------------------------------

// tagTwoNodes creates two tags by tagging the same node and returns their IDs.
func tagTwoNodes(t *testing.T, env *testEnv, nameA, nameB string) (string, string) {
	t.Helper()
	nodeID := env.createAndCommitNode(t, "event", "tag-anchor-"+nameA)
	for _, name := range []string{nameA, nameB} {
		tp, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
			Action: model.OpTagNode, Entity: "tag",
			Data: map[string]any{"node_id": nodeID, "tag_name": name},
		}})
		env.proposal.Commit(env.ctx, tp.ID)
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	var idA, idB string
	for _, tg := range tags {
		switch tg.Name {
		case nameA:
			idA = tg.ID
		case nameB:
			idB = tg.ID
		}
	}
	return idA, idB
}

func TestApplyCreateTagEdge(t *testing.T) {
	env := setup(t)
	alphaID, betaID := tagTwoNodes(t, env, "alpha-te", "beta-te")

	p, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateTagEdge,
		Entity: "tag_edge",
		Data: map[string]any{
			"type":    "parent_of",
			"from_id": alphaID,
			"to_id":   betaID,
		},
	}})
	if err != nil {
		t.Fatalf("Create tag-edge proposal: %v", err)
	}
	committed, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}

	tes, err := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{Type: "parent_of"})
	if err != nil || len(tes) == 0 {
		t.Fatalf("ListTagEdges: %v, len=%d", err, len(tes))
	}
}

func TestApplyCreateTagEdgeMissingFields(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateTagEdge,
		Entity: "tag_edge",
		Data:   map[string]any{"type": "related_to"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing from_id/to_id, got nil")
	}
}

func TestApplyCreateTagEdgeMissingType(t *testing.T) {
	env := setup(t)
	alphaID, betaID := tagTwoNodes(t, env, "ta-mt", "tb-mt")

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateTagEdge,
		Entity: "tag_edge",
		Data: map[string]any{
			"from_id": alphaID,
			"to_id":   betaID,
		},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing type, got nil")
	}
}

// ---------------------------------------------------------------------------
// applyDeleteTagEdge
// ---------------------------------------------------------------------------

func TestApplyDeleteTagEdge(t *testing.T) {
	env := setup(t)
	xID, yID := tagTwoNodes(t, env, "x-dte", "y-dte")

	// Create tag edge
	cep, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateTagEdge,
		Entity: "tag_edge",
		Data: map[string]any{
			"type":    "related_to",
			"from_id": xID,
			"to_id":   yID,
		},
	}})
	env.proposal.Commit(env.ctx, cep.ID)

	tes, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{})
	if len(tes) == 0 {
		t.Fatal("expected a tag edge to exist")
	}
	teID := tes[0].ID

	// Delete tag edge
	dp, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteTagEdge,
		Entity: "tag_edge",
		Data:   map[string]any{"id": teID},
	}})
	if err != nil {
		t.Fatalf("Create proposal: %v", err)
	}
	committed, err := env.proposal.Commit(env.ctx, dp.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}

	remaining, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{})
	if len(remaining) != 0 {
		t.Errorf("tag edges = %d after delete, want 0", len(remaining))
	}
}

func TestApplyDeleteTagEdgeMissingID(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteTagEdge,
		Entity: "tag_edge",
		Data:   map[string]any{},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
}

func TestApplyDeleteTagEdgeNotFound(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpDeleteTagEdge,
		Entity: "tag_edge",
		Data:   map[string]any{"id": "00000000-0000-0000-0000-000000000000"},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// ---------------------------------------------------------------------------
// getInt — cover float64 and json.Number code paths
// ---------------------------------------------------------------------------

func TestGetIntFloat64Reference(t *testing.T) {
	// float64 is the default JSON-decoded number type; test it via batch references.
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "event", "title": "F64-A"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "event", "title": "F64-B"}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{
			"type":           "caused_by",
			"from_reference": float64(0),
			"to_reference":   float64(1),
		}},
	})
	committed, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit with float64 references: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Fatalf("status = %s, want committed", committed.Status)
	}
}

func TestGetIntInvalidReference(t *testing.T) {
	// out-of-range reference should fail
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "event", "title": "R-A"}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{
			"type":           "caused_by",
			"from_reference": float64(0),
			"to_reference":   float64(99), // out of range
		}},
	})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for out-of-range reference, got nil")
	}
}

// ---------------------------------------------------------------------------
// applyOperation unknown action — cover the default case
// ---------------------------------------------------------------------------

func TestApplyOperationUnknownAction(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: "invalid_action",
		Entity: "node",
		Data:   map[string]any{},
	}})
	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
}

// ---------------------------------------------------------------------------
// Reject edge cases
// ---------------------------------------------------------------------------

func TestRejectAlreadyCommitted(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode, Entity: "node",
		Data: map[string]any{"type": "event", "title": "Already committed"},
	}})
	env.proposal.Commit(env.ctx, p.ID)

	_, err := env.proposal.Reject(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error rejecting a committed proposal, got nil")
	}
	if !errors.Is(err, model.ErrInvalidState) {
		t.Errorf("error type = %v, want ErrInvalidState", err)
	}
}

// ---------------------------------------------------------------------------
// List edge cases
// ---------------------------------------------------------------------------

func TestListProposalsByAfterCursor(t *testing.T) {
	env := setup(t)
	for i := 0; i < 3; i++ {
		env.proposal.Create(env.ctx, []model.ProposalOperation{{
			Action: model.OpCreateNode, Entity: "node",
			Data: map[string]any{"type": "event", "title": "cursor-test"},
		}})
	}

	all, err := env.proposal.List(env.ctx, proposal.ListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("total proposals = %d, want 3", len(all))
	}

	// Fetch page1 (limit 2) and page2 using the cursor from page1.
	page1, err := env.proposal.List(env.ctx, proposal.ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}

	page2, err := env.proposal.List(env.ctx, proposal.ListFilter{
		Limit: 10,
		After: page1[len(page1)-1].ID,
	})
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}

	// The two pages must together cover all IDs exactly once
	seen := map[string]bool{}
	for _, p := range append(page1, page2...) {
		seen[p.ID] = true
	}
	if len(seen) != 3 {
		t.Errorf("page1+page2 covered %d unique proposals, want 3", len(seen))
	}
}
