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
	n, err := e.graph.CreateNode(e.ctx, tx, graph.CreateNodeInput{
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

	n, err := env.graph.CreateNode(env.ctx, tx, graph.CreateNodeInput{
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

func TestCreateNodeInvalidType(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateNode(env.ctx, tx, graph.CreateNodeInput{
		Type:  "invalid_type",
		Title: "Bad node",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateNodeEmptyTitle(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateNode(env.ctx, tx, graph.CreateNodeInput{
		Type:  "task",
		Title: "",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateNodeAllTypes(t *testing.T) {
	env := setup(t)

	for nodeType := range model.ValidNodeTypes {
		t.Run(nodeType, func(t *testing.T) {
			tx := env.beginTx(t)
			_, err := env.graph.CreateNode(env.ctx, tx, graph.CreateNodeInput{
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

func TestCreateEdgeAllTypes(t *testing.T) {
	env := setup(t)

	for edgeType := range model.ValidEdgeTypes {
		t.Run(edgeType, func(t *testing.T) {
			a := env.createNode(t, "task", "From "+edgeType)
			b := env.createNode(t, "task", "To "+edgeType)
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

func TestCreateEdgeInvalidType(t *testing.T) {
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type:   "invalid_edge",
		FromID: a.ID,
		ToID:   b.ID,
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
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

func TestCycleRelatedToIsNotDirectional(t *testing.T) {
	// related_to is NOT directional, so A→B then B→A should succeed (no cycle check)
	env := setup(t)
	a := env.createNode(t, "task", "A")
	b := env.createNode(t, "task", "B")

	tx := env.beginTx(t)
	_, err := env.graph.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
		Type: "related_to", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("A→B related_to: %v", err)
	}
	tx.Commit()

	// related_to is not in DirectionalEdgeTypes so reverse should succeed
	tx2 := env.beginTx(t)
	_, err = env.graph.CreateEdge(env.ctx, tx2, graph.CreateEdgeInput{
		Type: "related_to", FromID: b.ID, ToID: a.ID,
	})
	if err != nil {
		tx2.Rollback()
		t.Fatalf("B→A related_to (non-directional, should succeed): %v", err)
	}
	tx2.Commit()
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
