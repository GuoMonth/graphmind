package event_test

import (
	"context"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
)

func setup(t *testing.T) (*event.Service, context.Context, func()) {
	t.Helper()
	ctx := context.Background()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	svc := event.NewService(d)
	return svc, ctx, func() { d.Close() }
}

func TestAppendAndList(t *testing.T) {
	svc, ctx, cleanup := setup(t)
	defer cleanup()

	d, _ := db.OpenMemory()
	defer d.Close()
	// Use the real DB from svc — we need a tx from the same DB.
	// Re-setup with access to DB.
	d2, _ := db.OpenMemory()
	defer d2.Close()
	db.Migrate(ctx, d2)
	svc2 := event.NewService(d2)

	tx, err := d2.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// Append events
	if err := svc2.Append(ctx, tx, "node", "n1", "node_created", map[string]string{"title": "test"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := svc2.Append(ctx, tx, "node", "n1", "node_updated", map[string]string{"title": "updated"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := svc2.Append(ctx, tx, "edge", "e1", "edge_created", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// List all events
	events, err := svc2.List(ctx, &event.ListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("List returned %d events, want 3", len(events))
	}

	// Filter by entity_type
	nodeEvents, err := svc2.List(ctx, &event.ListFilter{EntityType: "node"})
	if err != nil {
		t.Fatalf("List(EntityType=node): %v", err)
	}
	if len(nodeEvents) != 2 {
		t.Errorf("node events = %d, want 2", len(nodeEvents))
	}

	// Filter by entity_id
	n1Events, err := svc2.List(ctx, &event.ListFilter{EntityID: "n1"})
	if err != nil {
		t.Fatalf("List(EntityID=n1): %v", err)
	}
	if len(n1Events) != 2 {
		t.Errorf("n1 events = %d, want 2", len(n1Events))
	}

	// Filter by action
	createdEvents, err := svc2.List(ctx, &event.ListFilter{Action: "node_created"})
	if err != nil {
		t.Fatalf("List(Action=node_created): %v", err)
	}
	if len(createdEvents) != 1 {
		t.Errorf("node_created events = %d, want 1", len(createdEvents))
	}

	_ = svc // suppress unused
}

func TestListEmpty(t *testing.T) {
	_, ctx, cleanup := setup(t)
	defer cleanup()

	d, _ := db.OpenMemory()
	defer d.Close()
	db.Migrate(ctx, d)
	svc := event.NewService(d)

	events, err := svc.List(ctx, &event.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if events != nil {
		if len(events) != 0 {
			t.Errorf("empty list should return empty slice, got %d events", len(events))
		}
	}
}

func TestListDefaultLimit(t *testing.T) {
	_, ctx, cleanup := setup(t)
	defer cleanup()

	d, _ := db.OpenMemory()
	defer d.Close()
	db.Migrate(ctx, d)
	svc := event.NewService(d)

	tx, _ := d.Begin()
	for i := 0; i < 60; i++ {
		svc.Append(ctx, tx, "node", "n1", "node_created", nil)
	}
	tx.Commit()

	events, err := svc.List(ctx, &event.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 50 {
		t.Errorf("default limit: got %d events, want 50", len(events))
	}
}
