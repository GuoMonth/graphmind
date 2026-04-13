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
	PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
		if svc.db != nil {
			svc.db.Close()
			svc.db = nil
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", ".graphmind/graph.db", "Path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress stdout, exit code only")
	rootCmd.PersistentFlags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(lnCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(rejectCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(catCmd)
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

// output writes the JSON envelope to stdout.
func output(data any) {
	if quiet {
		return
	}
	env := model.Envelope{OK: true, Data: data}
	writeJSON(env)
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
			OK:    false,
			Error: &model.ErrorBody{Code: code, Message: err.Error()},
		}
		writeJSON(env)
	}

	return exitCode
}

// truncate returns at most the first n characters of s.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	enc.Encode(v)
}
