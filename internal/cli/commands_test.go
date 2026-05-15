// Package cli integration tests.
// Tests run in the cli package (white-box) so they can access package-level
// globals (dbPath, quiet, pretty, svc) and unexported helpers.
//
// Tests are NOT parallel — they share the global cobra command tree.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

// setupCLI wires a fresh SQLite DB into the global service vars and
// registers cleanup to close the DB after the test.
func setupCLI(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath = filepath.Join(tmpDir, "test.db")
	quiet = false
	pretty = false

	// Close any previously open DB
	if svc.db != nil {
		svc.db.Close()
		svc.db = nil
	}
	svc = services{}

	if err := wireAndMigrate(context.Background()); err != nil {
		t.Fatalf("wireAndMigrate: %v", err)
	}

	t.Cleanup(func() {
		if svc.db != nil {
			svc.db.Close()
			svc.db = nil
		}
	})
}

// captureStdout replaces os.Stdout with a pipe during fn(), collects output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origOut := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origOut }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	r.Close()
	return buf.String()
}

// mustDecodeEnvelope parses a single-line JSON envelope and fails the test on error.
func mustDecodeEnvelope(t *testing.T, raw string) model.Envelope {
	t.Helper()
	raw = strings.TrimSpace(raw)
	var env model.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode envelope: %v\nraw: %q", err, raw)
	}
	return env
}

// runAndAdd is a helper that calls the add command and returns the proposal
// from the output.  It also expects ok=true.
func runAdd(t *testing.T, typ, title string) string {
	t.Helper()
	addType = typ
	addTitle = title
	addDescription = ""
	addStatus = ""
	addWho = ""
	addWhere = ""
	addEventTime = ""

	ctx := context.Background()
	addCmd.SetContext(ctx)

	out := captureStdout(t, func() {
		if err := addCmd.RunE(addCmd, nil); err != nil {
			t.Fatalf("add RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("add: ok=false, error=%v", env.Error)
	}
	data, _ := env.Data.(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("add: empty proposal id")
	}
	return id
}

// runCommit commits a proposal and verifies ok=true.
func runCommit(t *testing.T, proposalID string) {
	t.Helper()
	ctx := context.Background()
	commitCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := commitCmd.RunE(commitCmd, []string{proposalID}); err != nil {
			t.Fatalf("commit RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("commit: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests (no DB required)
// ---------------------------------------------------------------------------

func TestOutputSuccess(t *testing.T) {
	setupCLI(t)
	out := captureStdout(t, func() {
		outputSuccess(map[string]string{"key": "val"}, "test summary", []string{"next step"})
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("expected ok=true")
	}
	if env.Summary != "test summary" {
		t.Errorf("summary = %q, want 'test summary'", env.Summary)
	}
}

func TestOutputSuccessQuiet(t *testing.T) {
	setupCLI(t)
	quiet = true
	out := captureStdout(t, func() {
		outputSuccess("data", "summary", nil)
	})
	if out != "" {
		t.Errorf("expected empty output in quiet mode, got %q", out)
	}
}

func TestOutputError(t *testing.T) {
	setupCLI(t)
	out := captureStdout(t, func() {
		code := outputError(model.ErrInvalidInput)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q, want INVALID_INPUT", env.Error.Code)
	}
}

func TestOutputErrorCodes(t *testing.T) {
	setupCLI(t)
	cases := []struct {
		err      error
		wantCode int
		wantStr  string
	}{
		{model.ErrInvalidInput, 1, "INVALID_INPUT"},
		{model.ErrNotFound, 2, "NOT_FOUND"},
		{model.ErrConflict, 3, "CONFLICT"},
		{model.ErrInvalidState, 3, "CONFLICT"},
	}
	for _, tc := range cases {
		code := outputError(tc.err)
		if code != tc.wantCode {
			t.Errorf("outputError(%v): code=%d, want %d", tc.err, code, tc.wantCode)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdefgh"); got != "abcdefgh" {
		t.Errorf("truncate 8-char: %q", got)
	}
	if got := truncate("abcdefghi"); got != "abcdefgh" {
		t.Errorf("truncate 9-char: %q", got)
	}
	if got := truncate("ab"); got != "ab" {
		t.Errorf("truncate 2-char: %q", got)
	}
}

func TestPluralize(t *testing.T) {
	if got := pluralize("node", "nodes", 1); got != "node" {
		t.Errorf("pluralize(1) = %q", got)
	}
	if got := pluralize("node", "nodes", 2); got != "nodes" {
		t.Errorf("pluralize(2) = %q", got)
	}
	if got := pluralize("node", "nodes", 0); got != "nodes" {
		t.Errorf("pluralize(0) = %q", got)
	}
}

func TestProposalNextSteps(t *testing.T) {
	steps := proposalNextSteps("abc-123")
	if len(steps) != 3 {
		t.Errorf("len(steps) = %d, want 3", len(steps))
	}
	for _, s := range steps {
		if !strings.Contains(s, "abc-123") {
			t.Errorf("step does not contain proposal id: %q", s)
		}
	}
}

func TestShouldSkipServiceWire(t *testing.T) {
	// init command → skip
	if !shouldSkipServiceWire(initCmd) {
		t.Error("expected init to skip service wire")
	}
	// update check → skip
	if !shouldSkipServiceWire(updateCheckCmd) {
		t.Error("expected update check to skip service wire")
	}
	// add command → don't skip
	if shouldSkipServiceWire(addCmd) {
		t.Error("expected add to not skip service wire")
	}
	// nil → skip
	if !shouldSkipServiceWire(nil) {
		t.Error("expected nil to skip service wire")
	}
}

func TestShouldAutoCheckForUpdates(t *testing.T) {
	quiet = false
	if !shouldAutoCheckForUpdates(addCmd) {
		t.Error("expected add to trigger update check")
	}
	if shouldAutoCheckForUpdates(initCmd) {
		t.Error("expected init to not trigger update check")
	}
	if shouldAutoCheckForUpdates(updateCheckCmd) {
		t.Error("expected update check to not trigger update check")
	}
	quiet = true
	if shouldAutoCheckForUpdates(addCmd) {
		t.Error("expected no update check when quiet")
	}
	quiet = false
}

func TestCommandInTree(t *testing.T) {
	if !commandInTree(updateCheckCmd, "update") {
		t.Error("expected update check to be in update tree")
	}
	if !commandInTree(updateCheckCmd, "check") {
		t.Error("expected update check to be in check tree")
	}
	if commandInTree(addCmd, "update") {
		t.Error("expected add to not be in update tree")
	}
}

// ---------------------------------------------------------------------------
// parseDuration
// ---------------------------------------------------------------------------

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
		wantH   float64 // expected hours
	}{
		{"1h", false, 1},
		{"24h", false, 24},
		{"7d", false, 168},
		{"30d", false, 720},
		{"1m", false, 0},     // minutes, not days
		{"invalid", true, 0}, // invalid
		{"xd", true, 0},      // invalid days
	}
	for _, tc := range cases {
		d, err := parseDuration(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q): expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if tc.wantH > 0 && d.Hours() != tc.wantH {
			t.Errorf("parseDuration(%q): %v hours, want %v", tc.input, d.Hours(), tc.wantH)
		}
	}
}

// ---------------------------------------------------------------------------
// batchCommandToOp
// ---------------------------------------------------------------------------

func TestBatchCommandToOp(t *testing.T) {
	cases := []struct {
		cmd     string
		data    map[string]any
		wantAct string
		wantErr bool
	}{
		{"add", map[string]any{"type": "event", "title": "T"}, model.OpCreateNode, false},
		{"ln", map[string]any{"type": "caused_by", "from_id": "a", "to_id": "b"}, model.OpCreateEdge, false},
		{"tag", map[string]any{"node_id": "a", "tag_name": "t"}, model.OpTagNode, false},
		{"mv", map[string]any{"id": "a", "status": "done"}, model.OpUpdateNode, false},
		{"rm", map[string]any{"id": "a"}, model.OpDeleteNode, false},
		{"rm", map[string]any{"id": "a", "entity": "edge"}, model.OpDeleteEdge, false},
		{"rm", map[string]any{"id": "a", "entity": "invalid"}, "", true}, // tag_edge not supported
		{"unknown", nil, "", true},
		{"add", nil, model.OpCreateNode, false}, // nil data is normalised to empty map
	}
	for _, tc := range cases {
		op, err := batchCommandToOp(tc.cmd, tc.data, 0)
		if tc.wantErr {
			if err == nil {
				t.Errorf("batchCommandToOp(%q): expected error, got nil", tc.cmd)
			}
			continue
		}
		if err != nil {
			t.Errorf("batchCommandToOp(%q): %v", tc.cmd, err)
			continue
		}
		if op.Action != tc.wantAct {
			t.Errorf("batchCommandToOp(%q): action=%q, want %q", tc.cmd, op.Action, tc.wantAct)
		}
	}
}

// ---------------------------------------------------------------------------
// init command
// ---------------------------------------------------------------------------

func TestInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath = filepath.Join(tmpDir, "init-test.db")
	defer func() {
		if svc.db != nil {
			svc.db.Close()
			svc.db = nil
		}
	}()

	ctx := context.Background()
	initCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := initCmd.RunE(initCmd, nil); err != nil {
			t.Fatalf("init RunE: %v", err)
		}
	})

	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("init: ok=false, error=%v", env.Error)
	}

	// DB file should exist
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}

// ---------------------------------------------------------------------------
// add command
// ---------------------------------------------------------------------------

func TestAddNodeViaFlags(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "Test event")
	if proposalID == "" {
		t.Fatal("expected non-empty proposal id")
	}
}

func TestAddNodeMissingType(t *testing.T) {
	setupCLI(t)
	addType = ""
	addTitle = "Some title"
	addDescription = ""
	addStatus = ""
	addWho = ""
	addWhere = ""
	addEventTime = ""

	ctx := context.Background()
	addCmd.SetContext(ctx)
	var capturedErr error
	captureStdout(t, func() {
		capturedErr = addCmd.RunE(addCmd, nil)
	})
	if capturedErr == nil {
		t.Fatal("expected error for missing type, got nil")
	}
}

func TestAddNodeViaStin(t *testing.T) {
	setupCLI(t)

	// Provide input via stdin pipe
	stdinJSON := `{"type":"person","title":"Alice","who":"self"}`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	io.WriteString(w, stdinJSON)
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	addType = ""
	addTitle = ""
	addDescription = ""
	addStatus = ""
	addWho = ""
	addWhere = ""
	addEventTime = ""

	ctx := context.Background()
	addCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := addCmd.RunE(addCmd, nil); err != nil {
			t.Fatalf("add via stdin RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("add stdin: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// commit command
// ---------------------------------------------------------------------------

func TestCommitCommand(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "To commit")
	runCommit(t, proposalID)
}

func TestCommitNotFound(t *testing.T) {
	setupCLI(t)
	ctx := context.Background()
	commitCmd.SetContext(ctx)
	err := commitCmd.RunE(commitCmd, []string{"00000000-0000-0000-0000-000000000000"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// reject command
// ---------------------------------------------------------------------------

func TestRejectCommand(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "To reject")

	ctx := context.Background()
	rejectCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := rejectCmd.RunE(rejectCmd, []string{proposalID}); err != nil {
			t.Fatalf("reject RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("reject: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// ls command
// ---------------------------------------------------------------------------

func TestLsNodesEmpty(t *testing.T) {
	setupCLI(t)
	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""

	ctx := context.Background()
	lsCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := lsCmd.RunE(lsCmd, []string{}); err != nil {
			t.Fatalf("ls RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("ls nodes: ok=false")
	}
}

func TestLsAllEntityTypes(t *testing.T) {
	setupCLI(t)
	for _, entity := range []string{"node", "edge", "tag", "tag_edge", "proposal"} {
		lsType = ""
		lsStatus = ""
		lsLimit = 50
		lsAfter = ""

		ctx := context.Background()
		lsCmd.SetContext(ctx)
		out := captureStdout(t, func() {
			if err := lsCmd.RunE(lsCmd, []string{entity}); err != nil {
				t.Fatalf("ls %s RunE: %v", entity, err)
			}
		})
		env := mustDecodeEnvelope(t, out)
		if !env.OK {
			t.Fatalf("ls %s: ok=false", entity)
		}
	}
}

func TestLsUnknownEntity(t *testing.T) {
	setupCLI(t)
	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""

	ctx := context.Background()
	lsCmd.SetContext(ctx)
	err := lsCmd.RunE(lsCmd, []string{"invalid_entity"})
	if err == nil {
		t.Fatal("expected error for unknown entity type")
	}
}

func TestLsNodeWithTypeFilter(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "Event1")
	runCommit(t, proposalID)
	proposalID2 := runAdd(t, "task", "Task1")
	runCommit(t, proposalID2)

	lsType = "event"
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""

	ctx := context.Background()
	lsCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := lsCmd.RunE(lsCmd, []string{"node"}); err != nil {
			t.Fatalf("ls node --type event: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("ls node --type event: ok=false")
	}
}

func TestLsLimitClamping(t *testing.T) {
	setupCLI(t)
	lsType = ""
	lsStatus = ""
	lsLimit = 9999 // above maxLimit; should be clamped
	lsAfter = ""

	ctx := context.Background()
	lsCmd.SetContext(ctx)
	out := captureStdout(t, func() {
		if err := lsCmd.RunE(lsCmd, []string{"node"}); err != nil {
			t.Fatalf("ls with clamped limit: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("ls clamped: ok=false")
	}
}

// ---------------------------------------------------------------------------
// cat command
// ---------------------------------------------------------------------------

func TestCatNode(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "Inspect me")
	runCommit(t, proposalID)

	// Get the node id
	lsType = "event"
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""
	ctx := context.Background()
	lsCmd.SetContext(ctx)
	var nodeID string
	captureStdout(t, func() {
		lsCmd.RunE(lsCmd, []string{"node"})
	}) // already committed

	// Use ls to find node id
	lsType = ""
	lsLimit = 50
	lsAfter = ""
	lsCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		lsCmd.RunE(lsCmd, []string{"node"})
	})
	env := mustDecodeEnvelope(t, out)
	items, _ := env.Data.([]any)
	if len(items) > 0 {
		if m, ok := items[0].(map[string]any); ok {
			nodeID, _ = m["id"].(string)
		}
	}
	if nodeID == "" {
		t.Skip("no nodes created yet — skip cat test")
	}

	catCmd.SetContext(context.Background())
	out = captureStdout(t, func() {
		if err := catCmd.RunE(catCmd, []string{nodeID}); err != nil {
			t.Fatalf("cat RunE: %v", err)
		}
	})
	env = mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("cat node: ok=false, error=%v", env.Error)
	}
}

func TestCatNotFound(t *testing.T) {
	setupCLI(t)
	catCmd.SetContext(context.Background())
	err := catCmd.RunE(catCmd, []string{"00000000-0000-0000-0000-000000000000"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCatProposal(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "Pending proposal")

	catCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := catCmd.RunE(catCmd, []string{proposalID}); err != nil {
			t.Fatalf("cat proposal: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("cat proposal: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// grep command
// ---------------------------------------------------------------------------

func TestGrepEmpty(t *testing.T) {
	setupCLI(t)
	grepLimit = 50
	grepAfter = ""

	grepCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := grepCmd.RunE(grepCmd, []string{"keyword"}); err != nil {
			t.Fatalf("grep RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("grep: ok=false")
	}
}

func TestGrepWithResults(t *testing.T) {
	setupCLI(t)
	proposalID := runAdd(t, "event", "Quarterly budget review")
	runCommit(t, proposalID)

	grepLimit = 50
	grepAfter = ""
	grepCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := grepCmd.RunE(grepCmd, []string{"budget"}); err != nil {
			t.Fatalf("grep RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("grep with results: ok=false")
	}
}

func TestGrepLimitClamping(t *testing.T) {
	setupCLI(t)
	grepLimit = 9999
	grepAfter = ""
	grepCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		grepCmd.RunE(grepCmd, []string{"anything"})
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("grep clamped: ok=false")
	}
}

// ---------------------------------------------------------------------------
// log command
// ---------------------------------------------------------------------------

func TestLogEmpty(t *testing.T) {
	setupCLI(t)
	logEntityID = ""
	logAction = ""
	logSince = ""
	logLimit = 50
	logAfter = ""

	logCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := logCmd.RunE(logCmd, nil); err != nil {
			t.Fatalf("log RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("log: ok=false")
	}
}

func TestLogWithSince(t *testing.T) {
	setupCLI(t)
	logEntityID = ""
	logAction = ""
	logSince = "24h"
	logLimit = 50
	logAfter = ""

	logCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := logCmd.RunE(logCmd, nil); err != nil {
			t.Fatalf("log --since 24h: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("log --since 24h: ok=false")
	}
}

func TestLogWithSinceDays(t *testing.T) {
	setupCLI(t)
	logSince = "7d"
	logEntityID = ""
	logAction = ""
	logLimit = 50
	logAfter = ""

	logCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := logCmd.RunE(logCmd, nil); err != nil {
			t.Fatalf("log --since 7d: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("log --since 7d: ok=false")
	}
}

func TestLogInvalidSince(t *testing.T) {
	setupCLI(t)
	logSince = "bad"
	logEntityID = ""
	logAction = ""
	logLimit = 50
	logAfter = ""

	logCmd.SetContext(context.Background())
	err := logCmd.RunE(logCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid --since")
	}
}

func TestLogLimitClamping(t *testing.T) {
	setupCLI(t)
	logEntityID = ""
	logAction = ""
	logSince = ""
	logLimit = 9999
	logAfter = ""

	logCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		logCmd.RunE(logCmd, nil)
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("log clamped: ok=false")
	}
}

// ---------------------------------------------------------------------------
// mv command
// ---------------------------------------------------------------------------

func TestMvNodeStatus(t *testing.T) {
	setupCLI(t)
	addPropID := runAdd(t, "event", "Move me")
	runCommit(t, addPropID)

	// Get node id
	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""
	lsCmd.SetContext(context.Background())
	out := captureStdout(t, func() { lsCmd.RunE(lsCmd, []string{"node"}) })
	env := mustDecodeEnvelope(t, out)
	items, _ := env.Data.([]any)
	if len(items) == 0 {
		t.Fatal("no nodes after commit")
	}
	m, _ := items[0].(map[string]any)
	nodeID, _ := m["id"].(string)

	// mv via flags
	mvTitle = ""
	mvDescription = ""
	mvWho = ""
	mvWhere = ""
	mvEventTime = ""
	mvType = ""

	// Use SetArgs to mark "status" as changed
	mvCmd.ResetFlags()
	mvCmd.Flags().StringVar(&mvTitle, "title", "", "")
	mvCmd.Flags().StringVar(&mvDescription, "description", "", "")
	mvCmd.Flags().StringVar(&mvStatus, "status", "", "")
	mvCmd.Flags().StringVar(&mvType, "type", "", "")
	mvCmd.Flags().StringVar(&mvWho, "who", "", "")
	mvCmd.Flags().StringVar(&mvWhere, "where", "", "")
	mvCmd.Flags().StringVar(&mvEventTime, "event-time", "", "")

	if err := mvCmd.Flags().Set("status", "done"); err != nil {
		t.Fatalf("set status flag: %v", err)
	}

	mvCmd.SetContext(context.Background())
	out = captureStdout(t, func() {
		if err := mvCmd.RunE(mvCmd, []string{nodeID}); err != nil {
			t.Fatalf("mv RunE: %v", err)
		}
	})
	env = mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("mv: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// rm command
// ---------------------------------------------------------------------------

func TestRmNode(t *testing.T) {
	setupCLI(t)
	addPropID := runAdd(t, "event", "Delete me")
	runCommit(t, addPropID)

	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""
	lsCmd.SetContext(context.Background())
	out := captureStdout(t, func() { lsCmd.RunE(lsCmd, []string{"node"}) })
	env := mustDecodeEnvelope(t, out)
	items, _ := env.Data.([]any)
	if len(items) == 0 {
		t.Fatal("no nodes after commit")
	}
	m, _ := items[0].(map[string]any)
	nodeID, _ := m["id"].(string)

	rmCmd.SetContext(context.Background())
	out = captureStdout(t, func() {
		if err := rmCmd.RunE(rmCmd, []string{nodeID}); err != nil {
			t.Fatalf("rm RunE: %v", err)
		}
	})
	env = mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("rm: ok=false, error=%v", env.Error)
	}
}

func TestRmNotFound(t *testing.T) {
	setupCLI(t)
	rmCmd.SetContext(context.Background())
	err := rmCmd.RunE(rmCmd, []string{"00000000-0000-0000-0000-000000000000"})
	if err == nil {
		t.Fatal("expected not-found error from rm")
	}
}

func TestRmNoArgs(t *testing.T) {
	setupCLI(t)
	rmCmd.SetContext(context.Background())
	err := rmCmd.RunE(rmCmd, []string{})
	if err == nil {
		t.Fatal("expected error for rm with no args")
	}
}

// ---------------------------------------------------------------------------
// ln command
// ---------------------------------------------------------------------------

func TestLnNodeEdge(t *testing.T) {
	setupCLI(t)
	p1 := runAdd(t, "event", "Source")
	runCommit(t, p1)
	p2 := runAdd(t, "event", "Target")
	runCommit(t, p2)

	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""
	lsCmd.SetContext(context.Background())
	out := captureStdout(t, func() { lsCmd.RunE(lsCmd, []string{"node"}) })
	env := mustDecodeEnvelope(t, out)
	items, _ := env.Data.([]any)
	if len(items) < 2 {
		t.Fatalf("need 2 nodes, got %d", len(items))
	}
	m0, _ := items[0].(map[string]any)
	m1, _ := items[1].(map[string]any)
	id0, _ := m0["id"].(string)
	id1, _ := m1["id"].(string)

	lnEdgeType = "caused_by"
	lnCmd.SetContext(context.Background())
	out = captureStdout(t, func() {
		if err := lnCmd.RunE(lnCmd, []string{id0, id1}); err != nil {
			t.Fatalf("ln RunE: %v", err)
		}
	})
	env = mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("ln: ok=false, error=%v", env.Error)
	}
}

func TestLnMissingType(t *testing.T) {
	setupCLI(t)
	lnEdgeType = ""
	lnCmd.SetContext(context.Background())
	err := lnCmd.RunE(lnCmd, []string{"id1", "id2"})
	if err == nil {
		t.Fatal("expected error for missing edge type")
	}
}

// ---------------------------------------------------------------------------
// tag command
// ---------------------------------------------------------------------------

func TestTagNode(t *testing.T) {
	setupCLI(t)
	p1 := runAdd(t, "event", "Tag me")
	runCommit(t, p1)

	lsType = ""
	lsStatus = ""
	lsLimit = 50
	lsAfter = ""
	lsCmd.SetContext(context.Background())
	out := captureStdout(t, func() { lsCmd.RunE(lsCmd, []string{"node"}) })
	env := mustDecodeEnvelope(t, out)
	items, _ := env.Data.([]any)
	if len(items) == 0 {
		t.Fatal("no nodes")
	}
	m, _ := items[0].(map[string]any)
	nodeID, _ := m["id"].(string)

	tagDescription = ""
	tagCmd.SetContext(context.Background())
	out = captureStdout(t, func() {
		if err := tagCmd.RunE(tagCmd, []string{nodeID, "sprint-1"}); err != nil {
			t.Fatalf("tag RunE: %v", err)
		}
	})
	env = mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("tag: ok=false, error=%v", env.Error)
	}
}

// ---------------------------------------------------------------------------
// batch command
// ---------------------------------------------------------------------------

func TestBatchCommand(t *testing.T) {
	setupCLI(t)

	batchInput := `[
		{"command":"add","data":{"type":"event","title":"Batch event A"}},
		{"command":"add","data":{"type":"event","title":"Batch event B"}}
	]`

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	io.WriteString(w, batchInput)
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	batchCmd.SetContext(context.Background())
	out := captureStdout(t, func() {
		if err := batchCmd.RunE(batchCmd, nil); err != nil {
			t.Fatalf("batch RunE: %v", err)
		}
	})
	env := mustDecodeEnvelope(t, out)
	if !env.OK {
		t.Fatalf("batch: ok=false, error=%v", env.Error)
	}
}

func TestBatchEmptyInput(t *testing.T) {
	setupCLI(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	batchCmd.SetContext(context.Background())
	err = batchCmd.RunE(batchCmd, nil)
	if err == nil {
		t.Fatal("expected error for empty batch stdin")
	}
}

func TestBatchInvalidJSON(t *testing.T) {
	setupCLI(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	io.WriteString(w, "not json")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	batchCmd.SetContext(context.Background())
	err = batchCmd.RunE(batchCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBatchEmptyArray(t *testing.T) {
	setupCLI(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	io.WriteString(w, "[]")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	batchCmd.SetContext(context.Background())
	err = batchCmd.RunE(batchCmd, nil)
	if err == nil {
		t.Fatal("expected error for empty operations array")
	}
}

func TestBatchUnknownCommand(t *testing.T) {
	setupCLI(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	io.WriteString(w, `[{"command":"invalid","data":{}}]`)
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin; r.Close() }()

	batchCmd.SetContext(context.Background())
	err = batchCmd.RunE(batchCmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// ---------------------------------------------------------------------------
// writeJSON with pretty mode
// ---------------------------------------------------------------------------

func TestWriteJSONPretty(t *testing.T) {
	setupCLI(t)
	pretty = true
	defer func() { pretty = false }()

	out := captureStdout(t, func() {
		outputSuccess(map[string]string{"key": "value"}, "pretty test", nil)
	})
	// Pretty-printed output should have indentation
	if !strings.Contains(out, "\n") {
		t.Errorf("expected newlines in pretty output, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// wireAndMigrate covers the full wiring path
// ---------------------------------------------------------------------------

func TestWireAndMigrateCreatesDB(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "new-subdir")
	dbPath = filepath.Join(subDir, "graph.db")

	defer func() {
		if svc.db != nil {
			svc.db.Close()
			svc.db = nil
		}
	}()

	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := wireAndMigrate(context.Background()); err != nil {
		t.Fatalf("wireAndMigrate: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}
