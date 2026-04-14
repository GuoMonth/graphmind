package cli

import (
	"errors"
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var lnEdgeType string

var lnCmd = &cobra.Command{
	Use:   "ln <from-id> <to-id>",
	Short: "Create an edge → proposal",
	Long: `Create a directed edge between two entities. Returns a pending proposal.

Auto-detects whether the IDs belong to nodes or tags:
  - If both IDs are nodes → creates a node edge
  - If both IDs are tags → creates a tag edge
  - Mixed (one node, one tag) → error

The edge is NOT created immediately. A proposal is returned with status
"pending". Call "gm commit <proposal-id>" to apply it.

CYCLE DETECTION
  Node edges are checked for same-type cycles using a recursive
  traversal. If adding the edge would form a cycle of the same edge type,
  the command fails with exit code 3 (CONFLICT).`,
	Example: `  # Create a node edge (auto-detected)
  gm ln 019abc... 019def... --type caused_by

  # Create a tag edge (auto-detected)
  gm ln <tag-id> <tag-id> --type parent_of

  # Output (proposal with status "pending"):
  # {"ok":true,"data":{"id":"019...","status":"pending","operations":[...],...}}

  # Error — mixed types:
  # {"ok":false,"error":{"code":"INVALID_INPUT","message":"invalid input: cannot link a node to a tag"}}`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		toID := args[1]

		if lnEdgeType == "" {
			return model.WithHint(
				fmt.Errorf("%w: edge type is required", model.ErrInvalidInput),
				"Provide --type <type>. Common types: caused_by, followed_by, related_to, parent_of, synonym_of.",
			)
		}

		ctx := cmd.Context()

		// Auto-detect entity type: try nodes first, then tags
		_, fromNodeErr := svc.graph.GetNode(ctx, fromID)
		_, fromTagErr := svc.tag.GetTag(ctx, fromID)
		_, toNodeErr := svc.graph.GetNode(ctx, toID)
		_, toTagErr := svc.tag.GetTag(ctx, toID)

		fromIsNode := fromNodeErr == nil
		fromIsTag := fromTagErr == nil
		toIsNode := toNodeErr == nil
		toIsTag := toTagErr == nil

		var op model.ProposalOperation

		switch {
		case fromIsNode && toIsNode:
			op = model.ProposalOperation{
				Action: model.OpCreateEdge,
				Entity: "edge",
				Data: map[string]any{
					"type":    lnEdgeType,
					"from_id": fromID,
					"to_id":   toID,
				},
				Summary: fmt.Sprintf("%s: %s → %s", lnEdgeType, truncate(fromID), truncate(toID)),
			}
		case fromIsTag && toIsTag:
			op = model.ProposalOperation{
				Action: model.OpCreateTagEdge,
				Entity: "tag_edge",
				Data: map[string]any{
					"type":    lnEdgeType,
					"from_id": fromID,
					"to_id":   toID,
				},
				Summary: fmt.Sprintf("tag %s: %s → %s", lnEdgeType, truncate(fromID), truncate(toID)),
			}
		case (fromIsNode && toIsTag) || (fromIsTag && toIsNode):
			return model.WithHint(
				fmt.Errorf("%w: cannot link a node to a tag — both IDs must be the same entity type", model.ErrInvalidInput),
				"Use 'gm tag <node-id> <tag-name>' to associate a tag with a node, or ensure both IDs are nodes or both are tags.",
			)
		default:
			// Neither found — return the most useful error
			if !fromIsNode && !fromIsTag && errors.Is(fromNodeErr, model.ErrNotFound) {
				return model.WithHint(
					fmt.Errorf("%w: entity %s not found", model.ErrNotFound, fromID),
					"Use 'gm ls node' or 'gm ls tag' to find valid IDs.",
				)
			}
			return model.WithHint(
				fmt.Errorf("%w: entity %s not found", model.ErrNotFound, toID),
				"Use 'gm ls node' or 'gm ls tag' to find valid IDs.",
			)
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			return err
		}

		entityType := "node"
		if op.Action == model.OpCreateTagEdge {
			entityType = "tag"
		}
		outputSuccess(p,
			fmt.Sprintf("Created pending proposal %s: create %s %s edge (%s → %s).",
				truncate(p.ID), entityType, lnEdgeType, truncate(fromID), truncate(toID)),
			proposalNextSteps(p.ID),
		)
		return nil
	},
}

func init() {
	lnCmd.Flags().StringVar(&lnEdgeType, "type", "", "Edge type (required)")
	_ = lnCmd.MarkFlagRequired("type")
}
