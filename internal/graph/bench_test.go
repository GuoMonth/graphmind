package graph_test

// Benchmarks for key graph operations.

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
)

type benchEnv struct {
	database *sql.DB
	svc      *graph.Service
	ctx      context.Context
}

func newBenchDB(b *testing.B) *benchEnv {
	b.Helper()
	ctx := context.Background()
	d, err := db.OpenMemory()
	if err != nil {
		b.Fatalf("OpenMemory: %v", err)
	}
	if err := db.Migrate(ctx, d); err != nil {
		b.Fatalf("Migrate: %v", err)
	}
	b.Cleanup(func() { d.Close() })
	return &benchEnv{database: d, svc: graph.NewService(d, event.NewService(d)), ctx: ctx}
}

func (e *benchEnv) beginTx(b *testing.B) *sql.Tx {
	b.Helper()
	tx, err := e.database.BeginTx(e.ctx, nil)
	if err != nil {
		b.Fatalf("begin tx: %v", err)
	}
	return tx
}

func (e *benchEnv) createNodeTx(b *testing.B, typ, title string) string {
	b.Helper()
	tx := e.beginTx(b)
	n, err := e.svc.CreateNode(e.ctx, tx, &graph.CreateNodeInput{Type: typ, Title: title})
	if err != nil {
		tx.Rollback()
		b.Fatalf("CreateNode: %v", err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
	return n.ID
}

// BenchmarkCreateNode measures the cost of creating a single node (includes begin/commit).
func BenchmarkCreateNode(b *testing.B) {
	env := newBenchDB(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := env.beginTx(b)
		_, err := env.svc.CreateNode(env.ctx, tx, &graph.CreateNodeInput{
			Type:  "event",
			Title: fmt.Sprintf("bench-node-%d", i),
		})
		if err != nil {
			tx.Rollback()
			b.Fatalf("CreateNode: %v", err)
		}
		tx.Commit()
	}
}

// BenchmarkListNodes1k measures ListNodes against 1 000 pre-existing nodes.
func BenchmarkListNodes1k(b *testing.B) {
	env := newBenchDB(b)

	for i := 0; i < 1000; i++ {
		env.createNodeTx(b, "event", fmt.Sprintf("seed-node-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes, err := env.svc.ListNodes(env.ctx, graph.ListNodesFilter{Limit: 50})
		if err != nil {
			b.Fatalf("ListNodes: %v", err)
		}
		_ = nodes
	}
}

// BenchmarkSearchNodes1k measures FTS5 search over 1 000 nodes.
func BenchmarkSearchNodes1k(b *testing.B) {
	env := newBenchDB(b)

	for i := 0; i < 1000; i++ {
		env.createNodeTx(b, "event", fmt.Sprintf("quarterly budget review item %d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes, err := env.svc.SearchNodes(env.ctx, graph.SearchNodesFilter{
			Pattern: "budget",
			Limit:   50,
		})
		if err != nil {
			b.Fatalf("SearchNodes: %v", err)
		}
		_ = nodes
	}
}

// BenchmarkListEdges measures listing edges in a graph with 200 edges.
func BenchmarkListEdges(b *testing.B) {
	env := newBenchDB(b)

	// Build a chain of 201 nodes with 200 edges
	ids := make([]string, 201)
	for i := range ids {
		ids[i] = env.createNodeTx(b, "event", fmt.Sprintf("chain-%d", i))
	}
	for i := 0; i < 200; i++ {
		tx := env.beginTx(b)
		_, err := env.svc.CreateEdge(env.ctx, tx, graph.CreateEdgeInput{
			Type:   "followed_by",
			FromID: ids[i],
			ToID:   ids[i+1],
		})
		if err != nil {
			tx.Rollback()
			b.Fatalf("CreateEdge: %v", err)
		}
		tx.Commit()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edges, err := env.svc.ListEdges(env.ctx, graph.ListEdgesFilter{Limit: 50})
		if err != nil {
			b.Fatalf("ListEdges: %v", err)
		}
		_ = edges
	}
}

// BenchmarkGetNode measures single-entity lookup.
func BenchmarkGetNode(b *testing.B) {
	env := newBenchDB(b)
	id := env.createNodeTx(b, "event", "lookup-target")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node, err := env.svc.GetNode(env.ctx, id)
		if err != nil {
			b.Fatalf("GetNode: %v", err)
		}
		_ = node
	}
}
