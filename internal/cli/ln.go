package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var lnEdgeType string

var lnCmd = &cobra.Command{
	Use:   "ln <from-id> <to-id>",
	Short: "Create an edge → proposal",
	Long: `Create a directed edge between two nodes. Returns a pending proposal.

The edge is NOT created immediately. A proposal is returned with status
"pending". Call "gm commit <proposal-id>" to apply it.

Both <from-id> and <to-id> must be valid existing node IDs. The --type
flag specifies the relationship kind.

EDGE TYPES
  depends_on   from depends on to (directional, cycle-checked)
  blocks       from blocks to (directional, cycle-checked)
  decompose    from decomposes into to (directional, cycle-checked)
  caused_by    from is caused by to (directional, cycle-checked)
  supersedes   from supersedes to (directional, cycle-checked)
  related_to   bidirectional association (NO cycle check)

CYCLE DETECTION
  Directional edges are checked for same-type cycles using a recursive
  traversal. If adding the edge would form a cycle of the same edge type,
  the command fails with exit code 3 (CONFLICT).`,
	Example: `  # Create a dependency edge
  gm ln 019abc... 019def... --type depends_on

  # Create a decomposition (parent → child)
  gm ln 019epic... 019task... --type decompose

  # Create a bidirectional relation (no cycle check)
  gm ln 019a... 019b... --type related_to

  # Output (proposal with status "pending"):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019...",
  #     "status": "pending",
  #     "operations": [
  #       {
  #         "action": "create_edge",
  #         "entity": "edge",
  #         "data": {"type":"depends_on","from_id":"019abc...","to_id":"019def..."},
  #         "summary": "depends_on: 019abc... → 019def..."
  #       }
  #     ],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:30:00.000Z"
  #   }
  # }

  # Error — cycle detected:
  # {"ok":false,"error":{"code":"CONFLICT","message":"conflict: edge would create a cycle"}}

  # Error — node not found:
  # {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: from_id node does not exist"}}`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		toID := args[1]

		if !model.IsValidEdgeType(lnEdgeType) {
			return model.WithHint(
				fmt.Errorf("%w: invalid edge type %q", model.ErrInvalidInput, lnEdgeType),
				fmt.Sprintf("Valid edge types: %v. Example: gm ln <from> <to> --type depends_on", model.AllEdgeTypes()),
			)
		}

		op := model.ProposalOperation{
			Action: model.OpCreateEdge,
			Entity: "edge",
			Data: map[string]any{ // proposal operation data uses any
				"type":    lnEdgeType,
				"from_id": fromID,
				"to_id":   toID,
			},
			Summary: fmt.Sprintf("%s: %s → %s", lnEdgeType, truncate(fromID), truncate(toID)),
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Created pending proposal %s: create %s edge (%s → %s).",
				truncate(p.ID), lnEdgeType, truncate(fromID), truncate(toID)),
			proposalNextSteps(p.ID),
		)
		return nil
	},
}

func init() {
	lnCmd.Flags().StringVar(&lnEdgeType, "type", "", "Edge type (required)")
	_ = lnCmd.MarkFlagRequired("type")
}
