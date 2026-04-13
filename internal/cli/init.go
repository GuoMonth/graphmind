package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize graph database",
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
