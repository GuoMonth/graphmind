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
	Long:  "Associate a tag with a node. Creates the tag if it doesn't exist (upsert). Returns a pending proposal.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(cmd.Context()); err != nil {
			return err
		}

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
