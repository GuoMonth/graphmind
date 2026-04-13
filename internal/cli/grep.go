package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var (
	grepLimit int
	grepAfter string
)

var grepCmd = &cobra.Command{
	Use:   "grep <pattern>",
	Short: "Full-text search nodes (FTS5)",
	Long: `Search nodes by title and description using SQLite FTS5 full-text search.

The pattern uses FTS5 query syntax: simple words for prefix matching,
quoted phrases for exact match, AND/OR/NOT for boolean logic.

Results are ranked by relevance (best matches first).

FTS5 QUERY SYNTAX

  payment              match any word starting with "payment"
  "fix login"          exact phrase match
  payment OR billing   match either term
  payment NOT refund   match payment but exclude refund
  pay*                 prefix wildcard

FLAGS

  --limit <n>     Max results (default 50, max 1000)
  --after <id>    Cursor for pagination

EXAMPLES

  # Simple keyword search:
  $ gm grep payment

  # Exact phrase:
  $ gm grep '"fix login bug"'

  # Boolean query:
  $ gm grep "auth AND token"

  # With pagination:
  $ gm grep payment --limit 10
  $ gm grep payment --limit 10 --after 019abc...

OUTPUT

  Success (array of matching nodes, ranked by relevance):
  {"ok":true,"data":[
    {"id":"019...","type":"task","title":"Payment processing",...},
    {"id":"019...","type":"task","title":"Fix payment bug",...}
  ]}

  No matches:
  {"ok":true,"data":[]}

  Error — empty pattern:
  {"ok":false,"error":{"code":"INVALID_INPUT","message":"invalid input: search pattern is required"}}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		const maxLimit = 1000
		if grepLimit > maxLimit {
			grepLimit = maxLimit
		}

		nodes, err := svc.graph.SearchNodes(cmd.Context(), graph.SearchNodesFilter{
			Pattern: pattern,
			Limit:   grepLimit,
			After:   grepAfter,
		})
		if err != nil {
			// FTS5 syntax errors show as generic DB errors; wrap as invalid input
			return fmt.Errorf("%w: %s", model.ErrInvalidInput, err)
		}

		output(nodes)
		return nil
	},
}

func init() {
	grepCmd.Flags().IntVar(&grepLimit, "limit", 50, "Max results (default 50, max 1000)")
	grepCmd.Flags().StringVar(&grepAfter, "after", "", "Cursor for pagination")
}
