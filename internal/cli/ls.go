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
	Use:   "ls [node|edge|tag|tag_edge|proposal]",
	Short: "List entities with filters",
	Long: `List entities in the graph. Defaults to listing nodes if no entity type given.

Supports filtering by type and status, pagination via --limit and --after.
Returns an array of entities wrapped in the standard JSON envelope.

ENTITY TYPES
  node       Recorded events and entities
  edge       Relationships between nodes
  tag        Semantic labels
  tag_edge   Relationships between tags
  proposal   Staged change batches (filter by status: pending, committed, rejected)

PAGINATION
  --limit N        Max results to return (default 50)
  --after <cursor>  Cursor-based pagination. For nodes/edges/proposals, pass the last
                   item's ID. For tags, pass the last tag's name.`,
	Example: `  # List all nodes (default entity)
  gm ls

  # List nodes filtered by type
  gm ls node --type event

  # List edges of a specific type
  gm ls edge --type caused_by

  # List all tags
  gm ls tag

  # List tag-to-tag relationships
  gm ls tag_edge --type parent_of

  # List pending proposals
  gm ls proposal --status pending

  # Pagination: get first 10, then next 10
  gm ls node --limit 10
  gm ls node --limit 10 --after 019abc...`,
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
			summary := fmt.Sprintf("Listed %d %s.", len(nodes), pluralize("node", "nodes", len(nodes)))
			next := []string{
				"gm cat <id>  — inspect a specific node",
				"gm grep <keyword>  — search nodes by keyword",
			}
			if len(nodes) == lsLimit {
				next = append(next, fmt.Sprintf("gm ls node --limit %d --after %s  — next page", lsLimit, nodes[len(nodes)-1].ID))
			}
			outputSuccess(nodes, summary, next)

		case "edge":
			edges, err := svc.graph.ListEdges(ctx, graph.ListEdgesFilter{
				Type:  lsType,
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("Listed %d %s.", len(edges), pluralize("edge", "edges", len(edges)))
			next := []string{"gm cat <id>  — inspect a specific edge"}
			if len(edges) == lsLimit {
				next = append(next, fmt.Sprintf("gm ls edge --limit %d --after %s  — next page", lsLimit, edges[len(edges)-1].ID))
			}
			outputSuccess(edges, summary, next)

		case "tag":
			tags, err := svc.tag.ListTags(ctx, tag.ListTagsFilter{
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("Listed %d %s.", len(tags), pluralize("tag", "tags", len(tags)))
			next := []string{"gm cat <id>  — inspect a specific tag"}
			if len(tags) == lsLimit {
				next = append(next, fmt.Sprintf("gm ls tag --limit %d --after %s  — next page", lsLimit, tags[len(tags)-1].Name))
			}
			outputSuccess(tags, summary, next)

		case "tag_edge":
			tagEdges, err := svc.tag.ListTagEdges(ctx, tag.ListTagEdgesFilter{
				Type:  lsType,
				Limit: lsLimit,
				After: lsAfter,
			})
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("Listed %d tag %s.", len(tagEdges), pluralize("edge", "edges", len(tagEdges)))
			next := []string{"gm cat <id>  — inspect a specific tag edge"}
			if len(tagEdges) == lsLimit {
				next = append(next, fmt.Sprintf("gm ls tag_edge --limit %d --after %s  — next page", lsLimit, tagEdges[len(tagEdges)-1].ID))
			}
			outputSuccess(tagEdges, summary, next)

		case "proposal":
			proposals, err := svc.proposal.List(ctx, proposal.ListFilter{
				Status: lsStatus,
				Limit:  lsLimit,
				After:  lsAfter,
			})
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("Listed %d %s.", len(proposals), pluralize("proposal", "proposals", len(proposals)))
			next := []string{
				"gm commit <id>  — apply a pending proposal",
				"gm cat <id>  — inspect a specific proposal",
			}
			if len(proposals) == lsLimit {
				next = append(next,
					fmt.Sprintf("gm ls proposal --limit %d --after %s  — next page",
						lsLimit, proposals[len(proposals)-1].ID))
			}
			outputSuccess(proposals, summary, next)

		default:
			return fmt.Errorf("%w: unknown entity type: %s (expected: node, edge, tag, tag_edge, proposal)",
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
