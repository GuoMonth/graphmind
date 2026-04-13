package cli

import (
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/graph"
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
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(); err != nil {
			os.Exit(outputError(err))
			return nil
		}
		defer svc.db.Close()

		entity := "node"
		if len(args) > 0 {
			entity = args[0]
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
				os.Exit(outputError(err))
				return nil
			}
			output(nodes)

		case "edge":
			edges, err := svc.graph.ListEdges(ctx, graph.ListEdgesFilter{
				Type:  lsType,
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				os.Exit(outputError(err))
				return nil
			}
			output(edges)

		case "tag":
			tags, err := svc.tag.ListTags(ctx, tag.ListTagsFilter{
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				os.Exit(outputError(err))
				return nil
			}
			output(tags)

		case "proposal":
			proposals, err := svc.proposal.List(ctx, proposal.ListFilter{
				Status: lsStatus,
				Limit:  lsLimit,
				After:  lsAfter,
			})
			if err != nil {
				os.Exit(outputError(err))
				return nil
			}
			output(proposals)

		default:
			os.Exit(outputError(&entityError{entity}))
			return nil
		}

		return nil
	},
}

type entityError struct {
	entity string
}

func (e *entityError) Error() string {
	return "unknown entity type: " + e.entity + " (expected: node, edge, tag, proposal)"
}

func init() {
	lsCmd.Flags().StringVar(&lsType, "type", "", "Filter by type")
	lsCmd.Flags().StringVar(&lsStatus, "status", "", "Filter by status")
	lsCmd.Flags().IntVar(&lsLimit, "limit", 50, "Max results")
	lsCmd.Flags().StringVar(&lsAfter, "after", "", "Cursor for pagination")
}
