package cli

import (
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var lnEdgeType string

var lnCmd = &cobra.Command{
	Use:   "ln <from-id> <to-id>",
	Short: "Create an edge → proposal",
	Long:  "Create a directed edge between two nodes. Returns a pending proposal.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(); err != nil {
			os.Exit(outputError(err))
			return nil
		}
		defer svc.db.Close()

		fromID := args[0]
		toID := args[1]

		op := model.ProposalOperation{
			Action: model.OpCreateEdge,
			Entity: "edge",
			Data: map[string]any{ // proposal operation data uses any
				"type":    lnEdgeType,
				"from_id": fromID,
				"to_id":   toID,
			},
			Summary: fmt.Sprintf("%s: %s → %s", lnEdgeType, fromID[:8], toID[:8]),
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			os.Exit(outputError(err))
			return nil
		}

		output(p)
		return nil
	},
}

func init() {
	lnCmd.Flags().StringVar(&lnEdgeType, "type", "", "Edge type (required)")
	lnCmd.MarkFlagRequired("type")
}
