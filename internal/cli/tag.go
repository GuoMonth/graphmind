package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var tagDescription string

var tagCmd = &cobra.Command{
	Use:   "tag <node-id> <tag-name>",
	Short: "Tag a node → proposal",
	Long: `Associate a tag with a node. Creates the tag if it doesn't exist (upsert).
Returns a pending proposal.

The tag is NOT applied immediately. A proposal is returned with status
"pending". Call "gm commit <proposal-id>" to apply it.

If the tag name does not exist yet, it is auto-created when the proposal
is committed. If it already exists, the existing tag is reused.`,
	Example: `  # Tag a node
  gm tag 019abc... "high-priority"

  # Tag with a description for the tag
  gm tag 019abc... "backend" --description "Backend services and APIs"

  # Output (proposal with status "pending"):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019...",
  #     "status": "pending",
  #     "operations": [
  #       {
  #         "action": "tag_node",
  #         "entity": "tag",
  #         "data": {"node_id":"019abc...","tag_name":"backend","description":"Backend services and APIs"},
  #         "summary": "tag 019abc.. with \"backend\""
  #       }
  #     ],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:30:00.000Z"
  #   }
  # }

  # Error — node not found (surfaces when proposal is committed, not at tag time):
  # {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: node does not exist"}}`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := args[0]
		tagName := args[1]

		op := model.ProposalOperation{
			Action: model.OpTagNode,
			Entity: "tag",
			Data: map[string]any{ // proposal operation data uses any
				"node_id":     nodeID,
				"tag_name":    tagName,
				"description": tagDescription,
			},
			Summary: fmt.Sprintf("tag %s with %q", truncate(nodeID, 8), tagName),
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			return err
		}

		output(p)
		return nil
	},
}

func init() {
	tagCmd.Flags().StringVar(&tagDescription, "description", "", "Tag description")
}
