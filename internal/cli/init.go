package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize graph database",
	Long: `Initialize the GraphMind SQLite database at the path specified by --db.

Creates the database file and directory if they do not exist, then runs
all schema migrations (nodes, edges, tags, proposals, events, FTS5 index).

Safe to run multiple times — migrations are idempotent.`,
	Example: `  # Initialize with default path (.graphmind/graph.db)
  gm init

  # Initialize with custom path
  gm init --db /tmp/project.db

  # Output on success:
  # {"ok":true,"data":{"status":"initialized","db_path":".graphmind/graph.db"}}`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if err := db.Migrate(cmd.Context(), database); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}

		output(map[string]string{
			"status":  "initialized",
			"db_path": dbPath,
		})
		return nil
	},
}
