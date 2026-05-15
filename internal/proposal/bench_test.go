package proposal_test

// Benchmarks for the proposal service:
//   BenchmarkCreateProposal   — create a single-op proposal
//   BenchmarkCommitProposal   — create + commit a single-op proposal
//   BenchmarkBatchCommit10    — create + commit a 10-op batch

import (
	"context"
	"fmt"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/proposal"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
)

func newBenchEnv(b *testing.B) *proposal.Service {
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
	ev := event.NewService(d)
	gr := graph.NewService(d, ev)
	tg := tag.NewService(d, ev)
	return proposal.NewService(d, ev, gr, tg)
}

// BenchmarkCreateProposal measures the cost of creating a pending proposal.
func BenchmarkCreateProposal(b *testing.B) {
	svc := newBenchEnv(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Create(ctx, []model.ProposalOperation{{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    map[string]any{"type": "event", "title": fmt.Sprintf("bench-%d", i)},
			Summary: "bench",
		}})
		if err != nil {
			b.Fatalf("Create: %v", err)
		}
	}
}

// BenchmarkCommitProposal measures create + commit of a single-op proposal.
func BenchmarkCommitProposal(b *testing.B) {
	svc := newBenchEnv(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := svc.Create(ctx, []model.ProposalOperation{{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    map[string]any{"type": "event", "title": fmt.Sprintf("commit-bench-%d", i)},
			Summary: "bench",
		}})
		if err != nil {
			b.Fatalf("Create: %v", err)
		}
		if _, err := svc.Commit(ctx, p.ID); err != nil {
			b.Fatalf("Commit: %v", err)
		}
	}
}

// BenchmarkBatchCommit10 measures create + commit of a 10-op batch.
func BenchmarkBatchCommit10(b *testing.B) {
	svc := newBenchEnv(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := make([]model.ProposalOperation, 10)
		for j := 0; j < 10; j++ {
			ops[j] = model.ProposalOperation{
				Action:  model.OpCreateNode,
				Entity:  "node",
				Data:    map[string]any{"type": "task", "title": fmt.Sprintf("batch-%d-%d", i, j)},
				Summary: "batch",
			}
		}
		p, err := svc.Create(ctx, ops)
		if err != nil {
			b.Fatalf("Create batch: %v", err)
		}
		if _, err := svc.Commit(ctx, p.ID); err != nil {
			b.Fatalf("Commit batch: %v", err)
		}
	}
}
