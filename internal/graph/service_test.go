package graph_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

type testEnv struct {
	db    *sql.DB
	graph *graph.Service
	ctx   context.Context
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
	t.Cleanup(func() { d.Close() })
	return &testEnv{db: d, graph: graphSvc, ctx: ctx}
}

func (e *testEnv) beginTx(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := e.db.BeginTx(e.ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return tx
}

func (e *testEnv) createNode(t *testing.T, nodeType, title string) *model.Node {
	t.Helper()
	tx := e.beginTx(t)
	n, err := e.graph.CreateNode(e.ctx, tx, &graph.CreateNodeInput{
		Type:  nodeType,
		Title: title,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateNode(%s, %s): %v", nodeType, title, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Node tests
// ---------------------------------------------------------------------------

func TestCreateNode(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "Build API")

	if n.ID == "" {
		t.Error("node ID is empty")
	}
	if n.Type != "task" {
		t.Errorf("Type = %q, want %q", n.Type, "task")
	}
	if n.Title != "Build API" {
		t.Errorf("Title = %q, want %q", n.Title, "Build API")
	}
	if n.Properties == nil {
		t.Error("Properties is nil, want empty map")
	}
}

func TestCreateNodeWithProperties(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)

	n, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type:        "task",
		Title:       "With props",
		Description: "desc",
		Status:      "active",
		Properties:  map[string]any{"priority": "high", "estimate": 3.5},
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateNode: %v", err)
	}
	tx.Commit()

	// Re-fetch to verify persistence
	got, err := env.graph.GetNode(env.ctx, n.ID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Description != "desc" {
		t.Errorf("Description = %q, want %q", got.Description, "desc")
	}
	if got.Status != "active" {
		t.Errorf("Status = %q, want %q", got.Status, "active")
	}
	if got.Properties["priority"] != "high" {
		t.Errorf("Properties[priority] = %v, want %q", got.Properties["priority"], "high")
	}
}

func TestCreateNodeOpenType(t *testing.T) {
	// Open type system: any non-empty string is valid
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	node, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type:  "custom_type",
		Title: "Custom type node",
	})
	if err != nil {
		t.Fatalf("CreateNode with custom type: %v", err)
	}
	if node.Type != "custom_type" {
		t.Errorf("Type = %q, want %q", node.Type, "custom_type")
	}
}

func TestCreateNodeEmptyTitle(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type:  "task",
		Title: "",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateNodeVariousTypes(t *testing.T) {
	env := setup(t)

	types := []string{"event", "person", "place", "concept", "task"}
	for _, nodeType := range types {
		t.Run(nodeType, func(t *testing.T) {
			tx := env.beginTx(t)
			_, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
				Type:  nodeType,
				Title: "Test " + nodeType,
			})
			if err != nil {
				tx.Rollback()
				t.Fatalf("CreateNode(%s): %v", nodeType, err)
			}
			tx.Commit()
		})
	}
}

func TestGetNodeNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.graph.GetNode(env.ctx, "nonexistent-id")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListNodesEmpty(t *testing.T) {
	env := setup(t)
	nodes, err := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected empty list, got %d nodes", len(nodes))
	}
}

func TestListNodesWithFilters(t *testing.T) {
	env := setup(t)

	env.createNode(t, "task", "Task 1")
	env.createNode(t, "task", "Task 2")
	env.createNode(t, "epic", "Epic 1")

	// No filter
	all, err := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all nodes = %d, want 3", len(all))
	}

	// Filter by type
	tasks, err := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Type: "task"})
	if err != nil {
		t.Fatalf("ListNodes(type=task): %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("task nodes = %d, want 2", len(tasks))
	}

	// Limit
	limited, err := env.graph.ListNodes(env.ctx, graph.ListNodesFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListNodes(limit=1): %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limited nodes = %d, want 1", len(limited))
	}
}

// ---------------------------------------------------------------------------
// Edge tests
// ---------------------------------------------------------------------------

func TestCreateEdge(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	e, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "depends_on",
		FromID: a.ID,
		ToID:   b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateEdge: %v", err)
	}
	tx.Commit()

	if e.FromID != a.ID || e.ToID != b.ID {
		t.Errorf("edge endpoints: from=%s to=%s, want %s → %s", e.FromID, e.ToID, a.ID, b.ID)
	}
	if e.Type != "depends_on" {
		t.Errorf("Type = %q, want %q", e.Type, "depends_on")
	}
}

func TestCreateEdgeVariousTypes(t *testing.T) {
	env := setup(t)

	types := []string{"caused_by", "followed_by", "related_to", "involves", "supersedes"}
	for _, edgeType := range types {
		t.Run(edgeType, func(t *testing.T) {
			a := env.createNode(t, "event", "From "+edgeType)
			b := env.createNode(t, "event", "To "+edgeType)
			tx := env.beginTx(t)
			_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
				Type:   edgeType,
				FromID: a.ID,
				ToID:   b.ID,
			})
			if err != nil {
				tx.Rollback()
				t.Fatalf("CreateEdge(%s): %v", edgeType, err)
			}
			tx.Commit()
		})
	}
}

func TestCreateEdgeEmptyType(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "event", "A")
	b := env.createNode(t, "event", "B")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "",
		FromID: a.ID,
		ToID:   b.ID,
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateEdgeOpenType(t *testing.T) {
	// Open type system: any non-empty string is valid
	env := setup(t)
	a := env.createNode(t, "event", "A")
	b := env.createNode(t, "event", "B")

	tx := env.beginTx(t)
	defer tx.Rollback()

	edge, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "custom_edge_type",
		FromID: a.ID,
		ToID:   b.ID,
	})
	if err != nil {
		t.Fatalf("CreateEdge with custom type: %v", err)
	}
	if edge.Type != "custom_edge_type" {
		t.Errorf("Type = %q, want %q", edge.Type, "custom_edge_type")
	}
}

func TestCreateEdgeMissingIDs(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "depends_on",
		FromID: "",
		ToID:   "",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateEdgeSelfReference(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "Self")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "depends_on",
		FromID: a.ID,
		ToID:   a.ID,
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("self-ref: err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateEdgeNodeNotFound(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "Exists")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "depends_on",
		FromID: a.ID,
		ToID:   "nonexistent",
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("to_id not found: err = %v, want ErrNotFound", err)
	}
}

func TestCreateEdgeFromNodeNotFound(t *testing.T) {
	env := setup(t)
	b := env.createNode(t, "task", "Exists")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "depends_on",
		FromID: "nonexistent",
		ToID:   b.ID,
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("from_id not found: err = %v, want ErrNotFound", err)
	}
}

func TestCreateEdgeDuplicate(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	// First edge succeeds
	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("first edge: %v", err)
	}
	tx.Commit()

	// Duplicate edge fails (same type, from, to)
	tx2 := env.beginTx(t)
	defer tx2.Rollback()

	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("duplicate: err = %v, want ErrConflict", err)
	}
}

func TestCreateEdgeSameNodesDifferentType(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	// Two edges between same nodes but different types should succeed
	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("depends_on: %v", err)
	}
	tx.Commit()

	tx2 := env.beginTx(t)
	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "related_to", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx2.Rollback()
		t.Fatalf("related_to same pair: %v", err)
	}
	tx2.Commit()
}

// ---------------------------------------------------------------------------
// Cycle detection tests
// ---------------------------------------------------------------------------

func TestCycleDirectA_B_A(t *testing.T) {
	// A → B, then B → A should be rejected
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("A→B: %v", err)
	}
	tx.Commit()

	tx2 := env.beginTx(t)
	defer tx2.Rollback()
	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "depends_on", FromID: b.ID, ToID: a.ID,
	})
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("B→A cycle: err = %v, want ErrConflict", err)
	}
}

func TestCycleTriangleA_B_C_A(t *testing.T) {
	// A → B → C, then C → A should be rejected
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")
	c := env.createNode(t, "task", "C")

	tx := env.beginTx(t)
	env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	tx.Commit()

	tx = env.beginTx(t)
	env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: b.ID, ToID: c.ID,
	})
	tx.Commit()

	tx = env.beginTx(t)
	defer tx.Rollback()
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: c.ID, ToID: a.ID,
	})
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("C→A cycle: err = %v, want ErrConflict", err)
	}
}

func TestCycleLongChain(t *testing.T) {
	// Chain of 5: A → B → C → D → E, then E → A should be rejected
	env := setup(t)
	nodes := make([]*model.Node, 5)
	for i := range nodes {
		nodes[i] = env.createNode(t, "task", string(rune('A'+i)))
	}

	for i := 0; i < 4; i++ {
		tx := env.beginTx(t)
		_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
			Type: "blocks", FromID: nodes[i].ID, ToID: nodes[i+1].ID,
		})
		if err != nil {
			tx.Rollback()
			t.Fatalf("edge %d→%d: %v", i, i+1, err)
		}
		tx.Commit()
	}

	tx := env.beginTx(t)
	defer tx.Rollback()
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "blocks", FromID: nodes[4].ID, ToID: nodes[0].ID,
	})
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("long chain cycle: err = %v, want ErrConflict", err)
	}
}

func TestCycleDiamondIsNotCycle(t *testing.T) {
	// Diamond: A → B, A → C, B → D, C → D — NO cycle
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")
	c := env.createNode(t, "task", "C")
	d := env.createNode(t, "task", "D")

	edges := []struct{ from, to *model.Node }{
		{a, b}, {a, c}, {b, d}, {c, d},
	}
	for _, e := range edges {
		tx := env.beginTx(t)
		_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
			Type: "depends_on", FromID: e.from.ID, ToID: e.to.ID,
		})
		if err != nil {
			tx.Rollback()
			t.Fatalf("diamond edge %s→%s: %v", e.from.Title, e.to.Title, err)
		}
		tx.Commit()
	}
}

func TestCycleCrossTypesAreIndependent(t *testing.T) {
	// A —depends_on→ B. B —blocks→ A should succeed (different type = different graph).
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("A→B depends_on: %v", err)
	}
	tx.Commit()

	// Different type: should succeed (cycle detection is per-type)
	tx2 := env.beginTx(t)
	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "blocks", FromID: b.ID, ToID: a.ID,
	})
	if err != nil {
		tx2.Rollback()
		t.Fatalf("B→A blocks (different type, should succeed): %v", err)
	}
	tx2.Commit()
}

func TestCycleDetectionAlwaysOn(t *testing.T) {
	// Open type system: ALL edge types get same-type cycle detection.
	// A→B related_to then B→A related_to should be rejected as a cycle.
	env := setup(t)
	a := env.createNode(t, "event", "A")
	b := env.createNode(t, "event", "B")

	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "related_to", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("A→B related_to: %v", err)
	}
	tx.Commit()

	// Reverse edge of same type should be rejected (cycle)
	tx2 := env.beginTx(t)
	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "related_to", FromID: b.ID, ToID: a.ID,
	})
	tx2.Rollback()
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("B→A related_to err = %v, want ErrConflict (cycle)", err)
	}

	// Different type between same nodes should succeed (cycle is per-type)
	tx3 := env.beginTx(t)
	_, err = env.graph.CreateEdge(env.ctx, tx3, graph.CreateEdgeInput{
		Type: "witnessed_by", FromID: b.ID, ToID: a.ID,
	})
	if err != nil {
		tx3.Rollback()
		t.Fatalf("B→A witnessed_by (different type, should succeed): %v", err)
	}
	tx3.Commit()
}

// ---------------------------------------------------------------------------
// List edges with filters
// ---------------------------------------------------------------------------

func TestListEdgesFilters(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")
	c := env.createNode(t, "task", "C")

	// A→B depends_on, A→C depends_on, B→C blocks
	for _, e := range []graph.CreateEdgeInput{
		{Type: "depends_on", FromID: a.ID, ToID: b.ID},
		{Type: "depends_on", FromID: a.ID, ToID: c.ID},
		{Type: "blocks", FromID: b.ID, ToID: c.ID},
	} {
		tx := env.beginTx(t)
		if _, err := env.graph.CreateEdge(env.ctx, tx, e); err != nil {
			tx.Rollback()
			t.Fatalf("CreateEdge: %v", err)
		}
		tx.Commit()
	}

	// All edges
	all, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{})
	if len(all) != 3 {
		t.Errorf("all edges = %d, want 3", len(all))
	}

	// Filter by type
	deps, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{Type: "depends_on"})
	if len(deps) != 2 {
		t.Errorf("depends_on edges = %d, want 2", len(deps))
	}

	// Filter by from_id
	fromA, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{FromID: a.ID})
	if len(fromA) != 2 {
		t.Errorf("edges from A = %d, want 2", len(fromA))
	}

	// Filter by to_id
	toC, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{ToID: c.ID})
	if len(toC) != 2 {
		t.Errorf("edges to C = %d, want 2", len(toC))
	}
}

func TestGetEdgeNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.graph.GetEdge(env.ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateNode tests
// ---------------------------------------------------------------------------

func TestUpdateNodeTitle(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "Original")

	tx := env.beginTx(t)
	updated, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:    n.ID,
		Title: "Updated",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("UpdateNode: %v", err)
	}
	tx.Commit()

	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Type != "task" {
		t.Errorf("Type changed to %q, want %q", updated.Type, "task")
	}
}

func TestUpdateNodeDescription(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "Desc test")

	tx := env.beginTx(t)
	desc := "new description"
	updated, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:          n.ID,
		Description: &desc,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("UpdateNode: %v", err)
	}
	tx.Commit()

	if updated.Description != "new description" {
		t.Errorf("Description = %q, want %q", updated.Description, "new description")
	}
}

func TestUpdateNodeEmptyDescription(t *testing.T) {
	env := setup(t)
	// Create node with description
	tx := env.beginTx(t)
	n, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type: "task", Title: "Desc clear", Description: "has desc",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateNode: %v", err)
	}
	tx.Commit()

	// Clear description with empty string
	tx = env.beginTx(t)
	empty := ""
	updated, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:          n.ID,
		Description: &empty,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("UpdateNode: %v", err)
	}
	tx.Commit()

	if updated.Description != "" {
		t.Errorf("Description = %q, want empty", updated.Description)
	}
}

func TestUpdateNodeMergeProperties(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	n, err := env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type:       "task",
		Title:      "Props merge",
		Properties: map[string]any{"a": "1", "b": "2"}, // JSON properties
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateNode: %v", err)
	}
	tx.Commit()

	// Update: overwrite "a", add "c", leave "b" untouched
	tx = env.beginTx(t)
	updated, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:         n.ID,
		Properties: map[string]any{"a": "X", "c": "3"}, // JSON properties
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("UpdateNode: %v", err)
	}
	tx.Commit()

	if updated.Properties["a"] != "X" {
		t.Errorf("Properties[a] = %v, want %q", updated.Properties["a"], "X")
	}
	if updated.Properties["b"] != "2" {
		t.Errorf("Properties[b] = %v, want %q (unchanged)", updated.Properties["b"], "2")
	}
	if updated.Properties["c"] != "3" {
		t.Errorf("Properties[c] = %v, want %q (new)", updated.Properties["c"], "3")
	}
}

func TestUpdateNodeNoFields(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "No change")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{ID: n.ID})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestUpdateNodeNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:    "nonexistent",
		Title: "Ghost",
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateNodeOpenType(t *testing.T) {
	// Open type system: updating to any non-empty type should succeed
	env := setup(t)
	n := env.createNode(t, "event", "Change type")

	tx := env.beginTx(t)
	defer tx.Rollback()

	updated, err := env.graph.UpdateNode(env.ctx, tx, &graph.UpdateNodeInput{
		ID:   n.ID,
		Type: "person",
	})
	if err != nil {
		t.Fatalf("UpdateNode to custom type: %v", err)
	}
	if updated.Type != "person" {
		t.Errorf("Type = %q, want %q", updated.Type, "person")
	}
}

// ---------------------------------------------------------------------------
// DeleteNode tests
// ---------------------------------------------------------------------------

func TestDeleteNode(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "To delete")

	tx := env.beginTx(t)
	if err := env.graph.DeleteNode(env.ctx, tx, n.ID); err != nil {
		tx.Rollback()
		t.Fatalf("DeleteNode: %v", err)
	}
	tx.Commit()

	_, err := env.graph.GetNode(env.ctx, n.ID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteNodeCascadeEdges(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")
	c := env.createNode(t, "task", "C")

	// A→B, B→C
	for _, e := range []graph.CreateEdgeInput{
		{Type: "depends_on", FromID: a.ID, ToID: b.ID},
		{Type: "depends_on", FromID: b.ID, ToID: c.ID},
	} {
		tx := env.beginTx(t)
		env.graph.CreateEdge(env.ctx, tx, e)
		tx.Commit()
	}

	// Delete B — should remove both edges
	tx := env.beginTx(t)
	if err := env.graph.DeleteNode(env.ctx, tx, b.ID); err != nil {
		tx.Rollback()
		t.Fatalf("DeleteNode(B): %v", err)
	}
	tx.Commit()

	edges, _ := env.graph.ListEdges(env.ctx, graph.ListEdgesFilter{})
	if len(edges) != 0 {
		t.Errorf("edges after cascade delete = %d, want 0", len(edges))
	}
}

func TestDeleteNodeCascadeTags(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "task", "Tagged node")

	// Insert a tag and node_tags association directly
	tx := env.beginTx(t)
	tx.ExecContext(env.ctx,
		"INSERT INTO tags (id, name, created_at, updated_at) VALUES ('tag1', 'backend', datetime('now'), datetime('now'))")
	tx.ExecContext(env.ctx,
		"INSERT INTO node_tags (node_id, tag_id) VALUES (?, 'tag1')", n.ID)
	tx.Commit()

	// Verify tag association exists
	var count int
	env.db.QueryRowContext(env.ctx,
		"SELECT COUNT(*) FROM node_tags WHERE node_id = ?", n.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("node_tags before delete = %d, want 1", count)
	}

	// Delete node — should cascade to node_tags
	tx = env.beginTx(t)
	if err := env.graph.DeleteNode(env.ctx, tx, n.ID); err != nil {
		tx.Rollback()
		t.Fatalf("DeleteNode: %v", err)
	}
	tx.Commit()

	// Tag association should be gone, but the tag itself remains
	env.db.QueryRowContext(env.ctx,
		"SELECT COUNT(*) FROM node_tags WHERE node_id = ?", n.ID).Scan(&count)
	if count != 0 {
		t.Errorf("node_tags after cascade delete = %d, want 0", count)
	}

	var tagCount int
	env.db.QueryRowContext(env.ctx,
		"SELECT COUNT(*) FROM tags WHERE id = 'tag1'").Scan(&tagCount)
	if tagCount != 1 {
		t.Errorf("tag should remain after node delete, got count=%d", tagCount)
	}
}

func TestDeleteNodeNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	err := env.graph.DeleteNode(env.ctx, tx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteEdge tests
// ---------------------------------------------------------------------------

func TestDeleteEdge(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	e, _ := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "depends_on", FromID: a.ID, ToID: b.ID,
	})
	tx.Commit()

	tx = env.beginTx(t)
	if err := env.graph.DeleteEdge(env.ctx, tx, e.ID); err != nil {
		tx.Rollback()
		t.Fatalf("DeleteEdge: %v", err)
	}
	tx.Commit()

	_, err := env.graph.GetEdge(env.ctx, e.ID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteEdgeNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	err := env.graph.DeleteEdge(env.ctx, tx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// SearchNodes (FTS5) tests
// ---------------------------------------------------------------------------

func TestSearchNodesBasic(t *testing.T) {
	env := setup(t)
	env.createNode(t, "task", "Payment processing")
	env.createNode(t, "task", "Authentication service")
	env.createNode(t, "task", "Payment refund handler")

	nodes, err := env.graph.SearchNodes(env.ctx, graph.SearchNodesFilter{
		Pattern: "payment",
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("search results = %d, want 2", len(nodes))
	}
}

func TestSearchNodesDescription(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	env.graph.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
		Type: "task", Title: "Generic title", Description: "handles payment logic",
	})
	tx.Commit()

	nodes, err := env.graph.SearchNodes(env.ctx, graph.SearchNodesFilter{
		Pattern: "payment",
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("search results = %d, want 1", len(nodes))
	}
}

func TestSearchNodesNoResults(t *testing.T) {
	env := setup(t)
	env.createNode(t, "task", "Unrelated task")

	nodes, err := env.graph.SearchNodes(env.ctx, graph.SearchNodesFilter{
		Pattern: "payment",
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("search results = %d, want 0", len(nodes))
	}
}

func TestSearchNodesLimit(t *testing.T) {
	env := setup(t)
	for i := 0; i < 5; i++ {
		env.createNode(t, "task", "Payment task")
	}

	nodes, err := env.graph.SearchNodes(env.ctx, graph.SearchNodesFilter{
		Pattern: "payment",
		Limit:   3,
	})
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("search results = %d, want 3", len(nodes))
	}
}
