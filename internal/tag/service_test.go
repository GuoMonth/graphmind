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
	n, err := e.graph.CreateNode(e.ctx, tx, &graph.CreateNodeInput{
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

// ---------------------------------------------------------------------------
// Tag Edge CRUD
// ---------------------------------------------------------------------------

func (e *testEnv) createTag(t *testing.T, name string) *model.Tag {
	t.Helper()
	tx := e.beginTx(t)
	tg, err := e.tag.CreateTag(e.ctx, tx, tag.CreateTagInput{Name: name})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTag(%s): %v", name, err)
	}
	tx.Commit()
	return tg
}

func TestCreateTagEdge(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "golang")
	b := env.createTag(t, "programming")

	tx := env.beginTx(t)
	edge, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type:   "parent_of",
		FromID: a.ID,
		ToID:   b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTagEdge: %v", err)
	}
	tx.Commit()

	if edge.ID == "" {
		t.Error("tag edge ID is empty")
	}
	if edge.Type != "parent_of" {
		t.Errorf("Type = %q, want %q", edge.Type, "parent_of")
	}
	if edge.FromID != a.ID {
		t.Errorf("FromID = %q, want %q", edge.FromID, a.ID)
	}
	if edge.ToID != b.ID {
		t.Errorf("ToID = %q, want %q", edge.ToID, b.ID)
	}
}

func TestCreateTagEdgeWithProperties(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "react")
	b := env.createTag(t, "frontend")

	tx := env.beginTx(t)
	edge, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type:       "related_to",
		FromID:     a.ID,
		ToID:       b.ID,
		Properties: map[string]any{"confidence": 0.95}, // JSON flexible
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTagEdge: %v", err)
	}
	tx.Commit()

	if edge.Properties["confidence"] != 0.95 {
		t.Errorf("Properties[confidence] = %v, want 0.95",
			edge.Properties["confidence"])
	}
}

func TestCreateTagEdgeEmptyType(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "t1")
	b := env.createTag(t, "t2")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "", FromID: a.ID, ToID: b.ID,
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateTagEdgeMissingIDs(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "related_to", FromID: "", ToID: "",
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateTagEdgeSelfReference(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "self")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "synonym_of", FromID: a.ID, ToID: a.ID,
	})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput (self-reference)", err)
	}
}

func TestCreateTagEdgeFromTagNotFound(t *testing.T) {
	env := setup(t)
	b := env.createTag(t, "exists")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "parent_of", FromID: "nonexistent", ToID: b.ID,
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateTagEdgeToTagNotFound(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "exists")

	tx := env.beginTx(t)
	defer tx.Rollback()

	_, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "parent_of", FromID: a.ID, ToID: "nonexistent",
	})
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetTagEdge(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "ta")
	b := env.createTag(t, "tb")

	tx := env.beginTx(t)
	created, err := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "synonym_of", FromID: a.ID, ToID: b.ID,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("CreateTagEdge: %v", err)
	}
	tx.Commit()

	got, err := env.tag.GetTagEdge(env.ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTagEdge: %v", err)
	}
	if got.Type != "synonym_of" {
		t.Errorf("Type = %q, want %q", got.Type, "synonym_of")
	}
}

func TestGetTagEdgeNotFound(t *testing.T) {
	env := setup(t)
	_, err := env.tag.GetTagEdge(env.ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteTagEdge(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "da")
	b := env.createTag(t, "db")

	tx := env.beginTx(t)
	edge, _ := env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "opposite_of", FromID: a.ID, ToID: b.ID,
	})
	tx.Commit()

	tx2 := env.beginTx(t)
	if err := env.tag.DeleteTagEdge(env.ctx, tx2, edge.ID); err != nil {
		tx2.Rollback()
		t.Fatalf("DeleteTagEdge: %v", err)
	}
	tx2.Commit()

	_, err := env.tag.GetTagEdge(env.ctx, edge.ID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteTagEdgeNotFound(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	err := env.tag.DeleteTagEdge(env.ctx, tx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteTagEdgeEmptyID(t *testing.T) {
	env := setup(t)
	tx := env.beginTx(t)
	defer tx.Rollback()

	err := env.tag.DeleteTagEdge(env.ctx, tx, "")
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestListTagEdgesEmpty(t *testing.T) {
	env := setup(t)
	edges, err := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{})
	if err != nil {
		t.Fatalf("ListTagEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected empty list, got %d", len(edges))
	}
}

func TestListTagEdgesWithTypeFilter(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "la")
	b := env.createTag(t, "lb")
	c := env.createTag(t, "lc")

	tx := env.beginTx(t)
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "parent_of", FromID: a.ID, ToID: b.ID,
	})
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "synonym_of", FromID: a.ID, ToID: c.ID,
	})
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "parent_of", FromID: b.ID, ToID: c.ID,
	})
	tx.Commit()

	all, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{})
	if len(all) != 3 {
		t.Fatalf("all edges = %d, want 3", len(all))
	}

	parents, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{
		Type: "parent_of",
	})
	if len(parents) != 2 {
		t.Errorf("parent_of edges = %d, want 2", len(parents))
	}

	synonyms, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{
		Type: "synonym_of",
	})
	if len(synonyms) != 1 {
		t.Errorf("synonym_of edges = %d, want 1", len(synonyms))
	}
}

func TestListTagEdgesLimit(t *testing.T) {
	env := setup(t)
	a := env.createTag(t, "x1")
	b := env.createTag(t, "x2")
	c := env.createTag(t, "x3")

	tx := env.beginTx(t)
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "related_to", FromID: a.ID, ToID: b.ID,
	})
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "related_to", FromID: a.ID, ToID: c.ID,
	})
	env.tag.CreateTagEdge(env.ctx, tx, tag.CreateTagEdgeInput{
		Type: "related_to", FromID: b.ID, ToID: c.ID,
	})
	tx.Commit()

	edges, _ := env.tag.ListTagEdges(env.ctx, tag.ListTagEdgesFilter{
		Limit: 2,
	})
	if len(edges) != 2 {
		t.Errorf("edges = %d, want 2 (limit)", len(edges))
	}
}
