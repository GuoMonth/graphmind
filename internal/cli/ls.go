package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/graph"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/senguoyun-guosheng/graphmind/internal/proposal"
	"github.com/senguoyun-guosheng/graphmind/internal/tag"
	"github.com/spf13/cobra"
)

var (
	lsType   string
	lsStatus string
	lsLimit  int
	lsAfter  string
)

var lsCmd = &cobra.Command{
	Use:   "ls [node|edge|tag|proposal]",
	Short: "List entities with filters",
	Long: `List entities in the graph. Defaults to listing nodes if no entity type given.

Supports filtering by type and status, pagination via --limit and --after.
Returns an array of entities wrapped in the standard JSON envelope.

ENTITY TYPES
  node       Project entities (task, epic, decision, risk, release, discussion)
  edge       Relationships between nodes
  tag        Semantic labels
  proposal   Staged change batches (filter by status: pending, committed, rejected)

PAGINATION
  --limit N        Max results to return (default 50)
  --after <cursor>  Cursor-based pagination. For nodes/edges/proposals, pass the last
                   item's ID. For tags, pass the last tag's name.`,
	Example: `  # List all nodes (default entity)
  gm ls

  # List nodes filtered by type
  gm ls node --type task

  # List nodes filtered by type and status
  gm ls node --type task --status active

  # List edges of a specific type
  gm ls edge --type depends_on

  # List all tags
  gm ls tag

  # List pending proposals
  gm ls proposal --status pending

  # Pagination: get first 10, then next 10
  gm ls node --limit 10
  gm ls node --limit 10 --after 019abc...

  # Output (array of entities):
  # {
  #   "ok": true,
  #   "data": [
  #     {
  #       "id": "019abc...",
  #       "type": "task",
  #       "title": "Build auth module",
  #       "description": "",
  #       "status": "",
  #       "properties": {},
  #       "created_at": "2025-01-15T10:30:00.000Z",
  #       "updated_at": "2025-01-15T10:30:00.000Z"
  #     }
  #   ]
  # }

  # Empty result (no error, just empty array):
  # {"ok":true,"data":[]}

  # Error — unknown entity type:
  # {"ok":false,"error":{"code":"INVALID_INPUT",
  #   "message":"invalid input: unknown entity type: foo (expected: node, edge, tag, proposal)"}}`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entity := "node"
		if len(args) > 0 {
			entity = args[0]
		}

		const maxLimit = 1000
		if lsLimit > maxLimit {
			lsLimit = maxLimit
		}

		ctx := cmd.Context()

		switch entity {
		case "node":
			nodes, err := svc.graph.ListNodes(ctx, graph.ListNodesFilter{
				Type:   lsType,
				Status: lsStatus,
				Limit:  lsLimit,
				After:  lsAfter,
			})
			if err != nil {
				return err
			}
			output(nodes)

		case "edge":
			edges, err := svc.graph.ListEdges(ctx, graph.ListEdgesFilter{
				Type:  lsType,
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				return err
			}
			output(edges)

		case "tag":
			tags, err := svc.tag.ListTags(ctx, tag.ListTagsFilter{
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				return err
			}
			output(tags)

		case "proposal":
			proposals, err := svc.proposal.List(ctx, proposal.ListFilter{
				Status: lsStatus,
				Limit:  lsLimit,
				After:  lsAfter,
			})
			if err != nil {
				return err
			}
			output(proposals)

		default:
			return fmt.Errorf("%w: unknown entity type: %s (expected: node, edge, tag, proposal)",
				model.ErrInvalidInput, entity)
		}

		return nil
	},
}

func init() {
	lsCmd.Flags().StringVar(&lsType, "type", "", "Filter by type")
	lsCmd.Flags().StringVar(&lsStatus, "status", "", "Filter by status")
	lsCmd.Flags().IntVar(&lsLimit, "limit", 50, "Max results")
	lsCmd.Flags().StringVar(&lsAfter, "after", "", "Cursor for pagination")
}
