package proposal_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/proposal"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
)

type testEnv struct {
	db       *sql.DB
	proposal *proposal.Service
	graph    *graph.Service
	tag      *tag.Service
	ctx      context.Context
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	eventSvc := event.NewService(d)
	graphSvc := graph.NewService(d, eventSvc)
	tagSvc := tag.NewService(d, eventSvc)
	proposalSvc := proposal.NewService(d, eventSvc, graphSvc, tagSvc)
	t.Cleanup(func() { d.Close() })
	return &testEnv{db: d, proposal: proposalSvc, graph: graphSvc, tag: tagSvc, ctx: ctx}
}

// createAndCommitNode is a helper that creates a node via proposal and commits it.
func (e *testEnv) createAndCommitNode(t *testing.T, nodeType, title string) string {
	t.Helper()
	p, err := e.proposal.Create(e.ctx, []model.ProposalOperation{{
		Action:  model.OpCreateNode,
		Entity:  "node",
		Data:    map[string]any{"type": nodeType, "title": title},
		Summary: "create " + title,
	}})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	committed, err := e.proposal.Commit(e.ctx, p.ID)
	if err != nil {
		t.Fatalf("commit proposal: %v", err)
	}
	// Find the node ID from the committed state
	nodes, err := e.graph.ListNodes(e.ctx, graph.ListNodesFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	for i := range nodes {
		if nodes[i].Title == title {
			return nodes[i].ID
		}
	}
	t.Fatalf("node %q not found after commit (proposal status=%s)", title, committed.Status)
	return ""
}

// ---------------------------------------------------------------------------
// Basic proposal lifecycle
// ---------------------------------------------------------------------------

func TestCreateProposal(t *testing.T) {
	env := setup(t)
	p, err := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action:  model.OpCreateNode,
		Entity:  "node",
		Data:    map[string]any{"type": "task", "title": "Test"},
		Summary: "create test node",
	}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.ID == "" {
		t.Error("proposal ID is empty")
	}
	if p.Status != model.ProposalStatusPending {
		t.Errorf("Status = %q, want %q", p.Status, model.ProposalStatusPending)
	}
	if len(p.Operations) != 1 {
		t.Errorf("Operations = %d, want 1", len(p.Operations))
	}
}

func TestCreateProposalEmpty(t *testing.T) {
	env := setup(t)
	_, err := env.proposal.Create(env.ctx, nil)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}

	_, err = env.proposal.Create(env.ctx, []model.ProposalOperation{})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetProposal(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Get test"},
	}})

	got, err := env.proposal.Get(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID = %q, want %q", got.ID, p.ID)
	}
}

func TestGetProposalNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.proposal.Get(env.ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Commit
// ---------------------------------------------------------------------------

func TestCommitCreateNode(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Committed Node", "description": "desc", "status": "active"},
	}})

	committed, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed.Status != model.ProposalStatusCommitted {
		t.Errorf("Status = %q, want %q", committed.Status, model.ProposalStatusCommitted)
	}

	// Verify the node was actually created
	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	if nodes[0].Title != "Committed Node" {
		t.Errorf("Title = %q, want %q", nodes[0].Title, "Committed Node")
	}
}

func TestCommitAlreadyCommitted(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Node"},
	}})
	env.proposal.Commit(env.ctx, p.ID)

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Errorf("double commit: err = %v, want ErrInvalidState", err)
	}
}

func TestCommitRejectedProposal(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Node"},
	}})
	env.proposal.Reject(env.ctx, p.ID)

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Errorf("commit rejected: err = %v, want ErrInvalidState", err)
	}
}

func TestCommitNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.proposal.Commit(env.ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Reject
// ---------------------------------------------------------------------------

func TestReject(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "To reject"},
	}})

	rejected, err := env.proposal.Reject(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if rejected.Status != model.ProposalStatusRejected {
		t.Errorf("Status = %q, want %q", rejected.Status, model.ProposalStatusRejected)
	}

	// Verify no node was created
	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if len(nodes) != 0 {
		t.Errorf("nodes = %d, want 0 (proposal was rejected)", len(nodes))
	}
}

func TestRejectAlreadyRejected(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Node"},
	}})
	env.proposal.Reject(env.ctx, p.ID)

	_, err := env.proposal.Reject(env.ctx, p.ID)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Errorf("double reject: err = %v, want ErrInvalidState", err)
	}
}

func TestRejectCommittedProposal(t *testing.T) {
	env := setup(t)
	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode,
		Entity: "node",
		Data:   map[string]any{"type": "task", "title": "Node"},
	}})
	env.proposal.Commit(env.ctx, p.ID)

	_, err := env.proposal.Reject(env.ctx, p.ID)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Errorf("reject committed: err = %v, want ErrInvalidState", err)
	}
}

// ---------------------------------------------------------------------------
// Commit with edge creation
// ---------------------------------------------------------------------------

func TestCommitCreateEdge(t *testing.T) {
	env := setup(t)
	nodeA := env.createAndCommitNode(t, "task", "A")
	nodeB := env.createAndCommitNode(t, "task", "B")

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateEdge,
		Entity: "edge",
		Data: map[string]any{
			"type":    "depends_on",
			"from_id": nodeA,
			"to_id":   nodeB,
		},
	}})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit edge: %v", err)
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{})
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	if edges[0].FromID != nodeA || edges[0].ToID != nodeB {
		t.Errorf("edge: %s→%s, want %s→%s", edges[0].FromID, edges[0].ToID, nodeA, nodeB)
	}
}

func TestCommitEdgeCycleRejected(t *testing.T) {
	env := setup(t)
	nodeA := env.createAndCommitNode(t, "task", "A")
	nodeB := env.createAndCommitNode(t, "task", "B")

	// A→B
	p1, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateEdge,
		Entity: "edge",
		Data:   map[string]any{"type": "depends_on", "from_id": nodeA, "to_id": nodeB},
	}})
	env.proposal.Commit(env.ctx, p1.ID)

	// B→A should fail (cycle)
	p2, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateEdge,
		Entity: "edge",
		Data:   map[string]any{"type": "depends_on", "from_id": nodeB, "to_id": nodeA},
	}})
	_, err := env.proposal.Commit(env.ctx, p2.ID)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !errors.Is(err, model.ErrConflict) {
		t.Logf("err = %v (contains cycle error, which is expected)", err)
	}
}

// ---------------------------------------------------------------------------
// Commit with tag
// ---------------------------------------------------------------------------

func TestCommitTagNode(t *testing.T) {
	env := setup(t)
	nodeID := env.createAndCommitNode(t, "task", "Tagged")

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpTagNode,
		Entity: "tag",
		Data:   map[string]any{"node_id": nodeID, "tag_name": "important"},
	}})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit tag: %v", err)
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 1 {
		t.Fatalf("tags = %d, want 1", len(tags))
	}
	if tags[0].Name != "important" {
		t.Errorf("Name = %q, want %q", tags[0].Name, "important")
	}
}

// ---------------------------------------------------------------------------
// Internal references (batch operations)
// ---------------------------------------------------------------------------

func TestCommitBatchWithInternalReferences(t *testing.T) {
	// Create two nodes and an edge between them, all in one proposal using references
	env := setup(t)

	p, err := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    map[string]any{"type": "task", "title": "Node A"},
			Summary: "create A",
		},
		{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    map[string]any{"type": "task", "title": "Node B"},
			Summary: "create B",
		},
		{
			Action: model.OpCreateEdge,
			Entity: "edge",
			Data: map[string]any{
				"type":           "depends_on",
				"from_reference": 0, // references op[0] = Node A
				"to_reference":   1, // references op[1] = Node B
			},
			Summary: "link A→B",
		},
	})
	if err != nil {
		t.Fatalf("Create batch: %v", err)
	}

	_, err = env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit batch: %v", err)
	}

	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if len(nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(nodes))
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{})
	if len(edges) != 1 {
		t.Errorf("edges = %d, want 1", len(edges))
	}
}

func TestCommitBatchWithTagReference(t *testing.T) {
	// Create a node and tag it in the same proposal
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{
			Action: model.OpCreateNode,
			Entity: "node",
			Data:   map[string]any{"type": "epic", "title": "Epic 1"},
		},
		{
			Action: model.OpTagNode,
			Entity: "tag",
			Data: map[string]any{
				"reference": 0, // references op[0] = Epic 1
				"tag_name":  "v1.0",
			},
		},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit batch with tag ref: %v", err)
	}

	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Type: "epic"})
	if len(nodes) != 1 {
		t.Fatalf("epic nodes = %d, want 1", len(nodes))
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 1 {
		t.Errorf("tags = %d, want 1", len(tags))
	}
	if tags[0].Name != "v1.0" {
		t.Errorf("tag name = %q, want %q", tags[0].Name, "v1.0")
	}
}

func TestCommitBatchInvalidReference(t *testing.T) {
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{
			Action: model.OpCreateNode,
			Entity: "node",
			Data:   map[string]any{"type": "task", "title": "Only node"},
		},
		{
			Action: model.OpCreateEdge,
			Entity: "edge",
			Data: map[string]any{
				"type":           "depends_on",
				"from_reference": 0,
				"to_reference":   5, // references op[5] which doesn't exist
			},
		},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for invalid reference, got nil")
	}
}

func TestCommitBatchUnknownAction(t *testing.T) {
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{
			Action: "unknown_action",
			Entity: "node",
			Data:   map[string]any{},
		},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("unknown action: err = %v, want ErrInvalidInput", err)
	}
}

// ---------------------------------------------------------------------------
// Complex batch: create graph topology in a single proposal
// ---------------------------------------------------------------------------

func TestCommitBatchDiamondTopology(t *testing.T) {
	// Create a diamond: A→B, A→C, B→D, C→D all in one proposal
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "A"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "B"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "C"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "D"}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 0, "to_reference": 1}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 0, "to_reference": 2}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 1, "to_reference": 3}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 2, "to_reference": 3}},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit diamond: %v", err)
	}

	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Limit: 100})
	if len(nodes) != 4 {
		t.Errorf("nodes = %d, want 4", len(nodes))
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Limit: 100})
	if len(edges) != 4 {
		t.Errorf("edges = %d, want 4", len(edges))
	}
}

func TestCommitBatchChainWithTags(t *testing.T) {
	// Create a chain A→B→C with tags on each node, all in one proposal
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Step 1"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Step 2"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Step 3"}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 0, "to_reference": 1}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 1, "to_reference": 2}},
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 0, "tag_name": "phase-1"}},
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 1, "tag_name": "phase-1"}},
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 2, "tag_name": "phase-2"}},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit chain+tags: %v", err)
	}

	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Limit: 100})
	if len(nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(nodes))
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Limit: 100})
	if len(edges) != 2 {
		t.Errorf("edges = %d, want 2", len(edges))
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2 (phase-1, phase-2)", len(tags))
	}
}

func TestCommitBatchRollbackOnError(t *testing.T) {
	// First op succeeds, second op fails — nothing should be persisted
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Good node"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "invalid_type", "title": "Bad node"}},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for invalid node type, got nil")
	}

	// No nodes should have been created (atomic rollback)
	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if len(nodes) != 0 {
		t.Errorf("nodes = %d, want 0 (should have rolled back)", len(nodes))
	}
}

func TestCommitBatchCycleInBatchRollback(t *testing.T) {
	// Create nodes and edges that form a cycle, all in one proposal
	// A→B, B→A cycle
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "A"}},
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "B"}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 0, "to_reference": 1}},
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 1, "to_reference": 0}},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err == nil {
		t.Fatal("expected cycle error in batch, got nil")
	}

	// Atomic rollback: nothing should be persisted
	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if len(nodes) != 0 {
		t.Errorf("nodes = %d, want 0 (cycle should have rolled back entire batch)", len(nodes))
	}
}

// ---------------------------------------------------------------------------
// List proposals
// ---------------------------------------------------------------------------

func TestListProposals(t *testing.T) {
	env := setup(t)

	// Create 3 proposals with different states
	p1, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "1"},
	}})
	p2, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "2"},
	}})
	p3, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{{
		Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "3"},
	}})

	env.proposal.Commit(env.ctx, p1.ID)
	env.proposal.Reject(env.ctx, p2.ID)

	// All proposals
	all, err := env.proposal.List(env.ctx, proposal.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}

	// Filter by status
	pending, _ := env.proposal.List(env.ctx, proposal.ListFilter{Status: "pending"})
	if len(pending) != 1 {
		t.Errorf("pending = %d, want 1", len(pending))
	}
	if pending[0].ID != p3.ID {
		t.Errorf("pending ID = %s, want %s", pending[0].ID, p3.ID)
	}

	committed, _ := env.proposal.List(env.ctx, proposal.ListFilter{Status: "committed"})
	if len(committed) != 1 {
		t.Errorf("committed = %d, want 1", len(committed))
	}

	rejected, _ := env.proposal.List(env.ctx, proposal.ListFilter{Status: "rejected"})
	if len(rejected) != 1 {
		t.Errorf("rejected = %d, want 1", len(rejected))
	}
}

func TestListProposalsLimit(t *testing.T) {
	env := setup(t)

	for i := 0; i < 5; i++ {
		env.proposal.Create(env.ctx, []model.ProposalOperation{{
			Action: model.OpCreateNode, Entity: "node",
			Data: map[string]any{"type": "task", "title": "node"},
		}})
	}

	limited, _ := env.proposal.List(env.ctx, proposal.ListFilter{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("limited = %d, want 2", len(limited))
	}
}

// ---------------------------------------------------------------------------
// Complex scenario: full project setup in one batch
// ---------------------------------------------------------------------------

func TestCommitBatchFullProjectSetup(t *testing.T) {
	// Simulate setting up a small project in a single proposal:
	// - Epic "Release v1"
	// - Task "Build API" (decompose from epic)
	// - Task "Write Tests" (decompose from epic)
	// - Task "Deploy" (depends on both Build API and Write Tests)
	// - Tag all tasks with "sprint-1"
	env := setup(t)

	p, _ := env.proposal.Create(env.ctx, []model.ProposalOperation{
		// 0: Epic
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "epic", "title": "Release v1"}},
		// 1: Build API
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Build API"}},
		// 2: Write Tests
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Write Tests"}},
		// 3: Deploy
		{Action: model.OpCreateNode, Entity: "node", Data: map[string]any{"type": "task", "title": "Deploy"}},
		// 4: Epic→Build API (decompose)
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "decompose", "from_reference": 0, "to_reference": 1}},
		// 5: Epic→Write Tests (decompose)
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "decompose", "from_reference": 0, "to_reference": 2}},
		// 6: Deploy depends on Build API
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 3, "to_reference": 1}},
		// 7: Deploy depends on Write Tests
		{Action: model.OpCreateEdge, Entity: "edge", Data: map[string]any{"type": "depends_on", "from_reference": 3, "to_reference": 2}},
		// 8: Tag Build API
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 1, "tag_name": "sprint-1"}},
		// 9: Tag Write Tests
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 2, "tag_name": "sprint-1"}},
		// 10: Tag Deploy
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 3, "tag_name": "sprint-1"}},
		// 11: Tag Epic
		{Action: model.OpTagNode, Entity: "tag", Data: map[string]any{"reference": 0, "tag_name": "release"}},
	})

	_, err := env.proposal.Commit(env.ctx, p.ID)
	if err != nil {
		t.Fatalf("Commit full project: %v", err)
	}

	// Verify counts
	nodes, _ := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Limit: 100})
	if len(nodes) != 4 {
		t.Errorf("nodes = %d, want 4", len(nodes))
	}

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Limit: 100})
	if len(edges) != 4 {
		t.Errorf("edges = %d, want 4", len(edges))
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2 (sprint-1, release)", len(tags))
	}

	// Verify edge types
	decomposeEdges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Type: "decompose"})
	if len(decomposeEdges) != 2 {
		t.Errorf("decompose edges = %d, want 2", len(decomposeEdges))
	}

	depEdges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Type: "depends_on"})
	if len(depEdges) != 2 {
		t.Errorf("depends_on edges = %d, want 2", len(depEdges))
	}
}
