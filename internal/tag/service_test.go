package tag_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
)

type testEnv struct {
	db    *sql.DB
	tag   *tag.Service
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
	tagSvc := tag.NewService(d, eventSvc)
	t.Cleanup(func() { d.Close() })
	return &testEnv{db: d, tag: tagSvc, graph: graphSvc, ctx: ctx}
}

func (e *testEnv) beginTx(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := e.db.BeginTx(e.ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return tx
}

func (e *testEnv) createNode(t *testing.T, title string) *model.Node {
	t.Helper()
	tx := e.beginTx(t)
	n, err := e.graph.CreateNode(e.ctx, tx, graph.CreateNodeInput{
		Type:  "task",
		Title: title,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateNode: %v", err)
	}
	tx.Commit()
	return n
}

// ---------------------------------------------------------------------------
// Tag CRUD
// ---------------------------------------------------------------------------

func TestCreateTag(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)

	tg, err := env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{
		Name:        "backend",
		Description: "Backend services",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTag: %v", err)
	}
	tx.Commit()

	if tg.ID == "" {
		t.Error("tag ID is empty")
	}
	if tg.Name != "backend" {
		t.Errorf("Name = %q, want %q", tg.Name, "backend")
	}
	if tg.Description != "Backend services" {
		t.Errorf("Description = %q, want %q", tg.Description, "Backend services")
	}
}

func TestCreateTagEmptyName(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: ""})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateTagDuplicateName(t *testing.T) {
	env := setup(t)

	tx := env.beginTx(t)
	_, err := env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: "dup"})
	if err != nil {
		tx.Rollback()
		t.Fatalf("first create: %v", err)
	}
	tx.Commit()

	tx2 := env.beginTx(t)
	defer tx2.Rollback()
	_, err = env.tag.CreateTag(env.ctx, tx2, tag.CreateTagInput{Name: "dup"})
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("duplicate: err = %v, want ErrConflict", err)
	}
}

func TestGetTag(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	created, _ := env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: "api"})
	tx.Commit()

	got, err := env.tag.GetTag(env.ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTag: %v", err)
	}
	if got.Name != "api" {
		t.Errorf("Name = %q, want %q", got.Name, "api")
	}
}

func TestGetTagNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.tag.GetTag(env.ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetTagByName(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	_, err := env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: "frontend"})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTag: %v", err)
	}
	tx.Commit()

	tx2 := env.beginTx(t)
	defer tx2.Rollback()
	got, err := env.tag.GetTagByName(env.ctx, tx2, "frontend")
	if err != nil {
		t.Fatalf("GetTagByName: %v", err)
	}
	if got.Name != "frontend" {
		t.Errorf("Name = %q, want %q", got.Name, "frontend")
	}
}

func TestGetTagByNameNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.GetTagByName(env.ctx, tx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// TagNode (upsert)
// ---------------------------------------------------------------------------

func TestTagNode(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "My Task")

	tx := env.beginTx(t)
	tg, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{
		NodeID:  n.ID,
		TagName: "urgent",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("TagNode: %v", err)
	}
	tx.Commit()

	if tg.Name != "urgent" {
		t.Errorf("tag name = %q, want %q", tg.Name, "urgent")
	}
}

func TestTagNodeCreatesTag(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "Task")

	// Tag doesn't exist yet — TagNode should create it
	tx := env.beginTx(t)
	_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{
		NodeID:      n.ID,
		TagName:     "new-tag",
		Description: "auto-created",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("TagNode: %v", err)
	}
	tx.Commit()

	// Verify tag was created via ListTags
	tags, err := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("tags = %d, want 1", len(tags))
	}
	if tags[0].Name != "new-tag" {
		t.Errorf("Name = %q, want %q", tags[0].Name, "new-tag")
	}
}

func TestTagNodeReusesExistingTag(t *testing.T) {
	env := setup(t)
	n1 := env.createNode(t, "Task 1")
	n2 := env.createNode(t, "Task 2")

	// Tag first node
	tx := env.beginTx(t)
	_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{NodeID: n1.ID, TagName: "shared"})
	if err != nil {
		tx.Rollback()
		t.Fatalf("TagNode(n1): %v", err)
	}
	tx.Commit()

	// Tag second node with same tag name
	tx2 := env.beginTx(t)
	_, err = env.tag.TagNode(env.ctx, tx2, tag.NodeInput{NodeID: n2.ID, TagName: "shared"})
	if err != nil {
		tx2.Rollback()
		t.Fatalf("TagNode(n2): %v", err)
	}
	tx2.Commit()

	// Should still be only one tag
	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 1 {
		t.Errorf("tags = %d, want 1 (reused)", len(tags))
	}
}

func TestTagNodeIdempotent(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "Task")

	// Tag the same node with the same tag twice
	for i := 0; i < 2; i++ {
		tx := env.beginTx(t)
		_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{NodeID: n.ID, TagName: "repeat"})
		if err != nil {
			tx.Rollback()
			t.Fatalf("TagNode attempt %d: %v", i, err)
		}
		tx.Commit()
	}

	// Should have only one tag and one association
	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if len(tags) != 1 {
		t.Errorf("tags = %d, want 1", len(tags))
	}
}

func TestTagNodeMissingNodeID(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{
		NodeID:  "",
		TagName: "test",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestTagNodeMissingTagName(t *testing.T) {
	env := setup(t)
	n := env.createNode(t, "Task")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{
		NodeID:  n.ID,
		TagName: "",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestTagNodeNodeNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.TagNode(env.ctx, tx, tag.NodeInput{
		NodeID:  "nonexistent",
		TagName: "test",
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// ListTags
// ---------------------------------------------------------------------------

func TestListTagsEmpty(t *testing.T) {
	env := setup(t)
	tags, err := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty list, got %d tags", len(tags))
	}
}

func TestListTagsOrdered(t *testing.T) {
	env := setup(t)

	names := []string{"zebra", "alpha", "middle"}
	for _, name := range names {
		tx := env.beginTx(t)
		env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: name})
		tx.Commit()
	}

	tags, err := env.tag.ListTags(env.ctx, tag.ListTagsFilter{})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("tags = %d, want 3", len(tags))
	}
	// Should be ordered by name ASC
	if tags[0].Name != "alpha" || tags[1].Name != "middle" || tags[2].Name != "zebra" {
		t.Errorf("order: %s, %s, %s — want alpha, middle, zebra",
			tags[0].Name, tags[1].Name, tags[2].Name)
	}
}

func TestListTagsLimit(t *testing.T) {
	env := setup(t)
	for i := 0; i < 5; i++ {
		tx := env.beginTx(t)
		env.tag.CreateTag(env.ctx, tx, tag.CreateTagInput{Name: string(rune('a' + i))})
		tx.Commit()
	}

	tags, _ := env.tag.ListTags(env.ctx, tag.ListTagsFilter{Limit: 2})
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}
}
