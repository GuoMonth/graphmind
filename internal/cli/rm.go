package cli

import (
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <id> [<id>...]",
	Short: "Delete entities → proposal",
	Long: `Delete one or more entities by ID. Returns a pending proposal.

Auto-detects entity type (node, edge, or tag_edge) from the ID. Deleting a node
cascades: its edges and tag associations are also removed.

Multiple IDs create a single proposal with multiple delete operations,
committed or rejected atomically.

STDIN PIPELINE

  When stdin is a pipe, reads JSONL (one JSON object per line) and
  extracts "id" from each object. This enables pipeline composition:

  gm ls node --type event | gm grep "deprecated" | gm rm

EXAMPLES

  # Delete a single node (cascade removes its edges and tag associations):
  $ gm rm 019abc...

  # Delete multiple entities in one atomic proposal:
  $ gm rm 019abc... 019def... 019ghi...

  # Pipeline: delete all nodes matching a search:
  $ gm grep "deprecated" | gm rm
  $ gm commit <proposal-id>

OUTPUT

  Success (pending proposal):
  {"ok":true,"data":{"id":"<proposal-id>","status":"pending","operations":[{"action":"delete_node",...},{"action":"delete_edge",...}],...}}

  Error — entity not found:
  {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: node"}}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ids := args

		// Read IDs from stdin pipe if available
		const maxBatchIDs = 10000
		if len(ids) > maxBatchIDs {
			return fmt.Errorf("%w: too many IDs (max %d)", model.ErrInvalidInput, maxBatchIDs)
		}

		stdinIDs, err := readOptionalIDsFromJSONLStdin(os.Stdin, maxBatchIDs-len(ids))
		if err != nil {
			return err
		}
		if len(stdinIDs) > 0 {
			ids = append(ids, stdinIDs...)
		}

		if len(ids) == 0 {
			return fmt.Errorf("%w: at least one ID is required", model.ErrInvalidInput)
		}

		ops := make([]model.ProposalOperation, 0, len(ids))
		ctx := cmd.Context()

		for _, id := range ids {
			// Auto-detect entity type: try node, edge, then tag_edge
			action := model.OpDeleteNode
			entity := "node"
			if _, err := svc.graph.GetNode(ctx, id); err != nil {
				if _, err := svc.graph.GetEdge(ctx, id); err != nil {
					if _, err := svc.tag.GetTagEdge(ctx, id); err != nil {
						return model.WithHint(
							fmt.Errorf("%w: entity %s not found", model.ErrNotFound, id),
							"Use 'gm ls node', 'gm ls edge', or 'gm ls tag_edge' to find valid IDs, or 'gm grep <keyword>' to search.",
						)
					}
					action = model.OpDeleteTagEdge
					entity = "tag_edge"
				} else {
					action = model.OpDeleteEdge
					entity = "edge"
				}
			}

			ops = append(ops, model.ProposalOperation{
				Action:  action,
				Entity:  entity,
				Data:    map[string]any{"id": id}, // proposal operation data uses any
				Summary: fmt.Sprintf("delete %s: %s", entity, truncate(id)),
			})
		}

		p, err := svc.proposal.Create(cmd.Context(), ops)
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Created pending proposal %s: delete %d %s.",
				truncate(p.ID), len(ops), pluralize("entity", "entities", len(ops))),
			proposalNextSteps(p.ID),
		)
		return nil
	},
}
