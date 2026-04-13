package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize graph database",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			os.Exit(outputError(fmt.Errorf("open database: %w", err)))
			return nil
		}
		defer database.Close()

		if err := db.Migrate(context.Background(), database); err != nil {
			os.Exit(outputError(fmt.Errorf("run migrations: %w", err)))
			return nil
		}

		output(map[string]string{
			"status":  "initialized",
			"db_path": dbPath,
		})
		return nil
	},
}
