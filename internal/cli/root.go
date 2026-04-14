package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/proposal"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
	"github.com/spf13/cobra"
)

var (
	dbPath string
	quiet  bool
	pretty bool
)

// Services wired at runtime.
type services struct {
	db       *sql.DB
	event    *event.Service
	graph    *graph.Service
	tag      *tag.Service
	proposal *proposal.Service
}

var svc services

var rootCmd = &cobra.Command{
	Use:           "gm",
	Short:         "GraphMind — graph-based project management for AI agents",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `GraphMind (gm) — graph-based project management CLI designed for AI agents.

All data is stored as a directed graph in SQLite. All write operations go
through a proposal workflow: write commands create pending proposals, which
must be explicitly committed or rejected before they modify the graph.

CORE CONCEPTS

  Graph Model
    Node  — a project entity with type, title, description, status, properties
    Edge  — a directed relationship between two nodes (from → to)
    Tag   — a semantic label attached to one or more nodes

  Proposal-First Writes
    Write commands (add, ln, tag) never modify the graph directly.
    They return a pending proposal containing one or more operations.
    You then commit or reject the proposal:

      gm add / gm ln / gm tag  →  creates a pending proposal
      gm commit <proposal-id>  →  applies all operations atomically
      gm reject <proposal-id>  →  discards the proposal

  Output Format
    Every command writes exactly one JSON object to stdout:

      Success:  {"ok":true,"data":<payload>,"summary":"...","next_steps":["..."]}
      Error:    {"ok":false,"error":{"code":"<CODE>","message":"<detail>","hint":"<AI guidance>"}}

    Use --pretty for human-readable indented output.
    Use --quiet to suppress stdout entirely (exit code only).

  Exit Codes
    0   Success
    1   INVALID_INPUT   bad arguments, missing fields, unknown type
    2   NOT_FOUND       entity does not exist
    3   CONFLICT        duplicate, cycle detected, or invalid state transition
    10  INTERNAL        unexpected error

ENTITY TYPES

  Node types:  task | epic | decision | risk | release | discussion
  Edge types:  depends_on | blocks | decompose | caused_by | related_to | supersedes
  Tag:         any string name (auto-created on first use)

  Directional edges (all except related_to) are checked for same-type
  cycles when created. related_to is a symmetric association (no cycle check).

COMMANDS

  Write (returns a pending proposal):
    add        Create a node                  gm add --type task --title "..."
    ln         Create an edge                 gm ln <from-id> <to-id> --type depends_on
    tag        Tag a node                     gm tag <node-id> <tag-name>
    mv         Update a node                  gm mv <id> --status done
    rm         Delete entities                gm rm <id> [<id>...]
    batch      Multi-op proposal from stdin   echo '[...]' | gm batch

  Control (apply or discard proposals):
    commit     Apply a pending proposal       gm commit <proposal-id>
    reject     Discard a pending proposal     gm reject <proposal-id>

  Read (query the graph):
    ls         List entities with filters     gm ls node --type task --limit 10
    cat        Show full detail by ID         gm cat <entity-id>
    grep       Full-text search (FTS5)        gm grep "payment"
    log        View event history             gm log --since 24h

  Setup:
    init       Initialize graph database      gm init

GLOBAL FLAGS

  --db string   Path to SQLite database (default ".graphmind/graph.db")
  --quiet       Suppress stdout, exit code only
  --pretty      Pretty-print JSON output

TYPICAL WORKFLOW

  $ gm init
  $ gm add --type task --title "Build auth module"
  # → {"ok":true,"data":{"id":"019...","status":"pending","operations":[...],...}}
  $ gm commit 019...
  # → {"ok":true,"data":{"id":"019...","status":"committed",...}}
  $ gm ls node --type task
  # → {"ok":true,"data":[{"id":"019...","type":"task","title":"Build auth module",...}]}
  $ gm cat <node-id>
  # → {"ok":true,"data":{"id":"019...","type":"task","title":"Build auth module",...}}

  To discard instead of applying:
  $ gm reject 019...
  # → {"ok":true,"data":{"id":"019...","status":"rejected",...}}

STDIN PIPELINE

  The add command accepts JSON from stdin instead of flags:

  $ echo '{"type":"task","title":"Fix bug #42"}' | gm add

Use "gm <command> --help" for detailed usage, examples, and output samples.`,
	PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
		if svc.db != nil {
			svc.db.Close()
			svc.db = nil
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// init handles its own DB setup; skip for root-only (help/version)
		if cmd.Name() == "init" || cmd.Parent() == nil {
			return nil
		}
		return wireAndMigrate(cmd.Context())
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", ".graphmind/graph.db", "Path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress stdout, exit code only")
	rootCmd.PersistentFlags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(mvCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(lnCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(rejectCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(catCmd)
	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(batchCmd)
}

// Execute runs the root command and handles exit codes.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		code := outputError(err)
		os.Exit(code)
	}
}

// wireServices opens the database and wires all services.
func wireServices() error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	svc.db = database
	svc.event = event.NewService(database)
	svc.graph = graph.NewService(database, svc.event)
	svc.tag = tag.NewService(database, svc.event)
	svc.proposal = proposal.NewService(database, svc.event, svc.graph, svc.tag)

	return nil
}

// wireAndMigrate opens DB, runs migrations, then wires services.
func wireAndMigrate(ctx context.Context) error {
	if err := wireServices(); err != nil {
		return err
	}
	return db.Migrate(ctx, svc.db)
}

// outputSuccess writes a JSON success envelope with summary and next-step guidance.
func outputSuccess(data any, summary string, nextSteps []string) {
	if quiet {
		return
	}
	env := model.Envelope{
		OK:        true,
		Data:      data,
		Summary:   summary,
		NextSteps: nextSteps,
	}
	writeJSON(env)
}

// proposalNextSteps returns common next-step suggestions for write commands that produce proposals.
func proposalNextSteps(proposalID string) []string {
	return []string{
		fmt.Sprintf("gm commit %s  — apply the changes to the graph", proposalID),
		fmt.Sprintf("gm cat %s  — inspect proposal details before committing", proposalID),
		fmt.Sprintf("gm reject %s  — discard this proposal", proposalID),
	}
}

// outputError writes a JSON error envelope to stdout and returns the appropriate exit code.
func outputError(err error) int {
	code := "INTERNAL"
	exitCode := 10

	switch {
	case errors.Is(err, model.ErrInvalidInput):
		code = "INVALID_INPUT"
		exitCode = 1
	case errors.Is(err, model.ErrNotFound):
		code = "NOT_FOUND"
		exitCode = 2
	case errors.Is(err, model.ErrConflict), errors.Is(err, model.ErrInvalidState):
		code = "CONFLICT"
		exitCode = 3
	}

	if !quiet {
		env := model.Envelope{
			OK: false,
			Error: &model.ErrorBody{
				Code:    code,
				Message: err.Error(),
				Hint:    model.GetHint(err),
			},
		}
		writeJSON(env)
	}

	return exitCode
}

// truncate returns at most the first 8 runes of s.
func truncate(s string) string {
	const n = 8
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// pluralize returns singular when n == 1, plural otherwise.
func pluralize(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	enc.Encode(v)
}
