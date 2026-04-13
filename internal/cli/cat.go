package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var catCmd = &cobra.Command{
	Use:   "cat <id>",
	Short: "Show full detail of one entity",
	Long:  "Show full detail of one entity by ID. Auto-detects entity type.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(cmd.Context()); err != nil {
			return err
		}

		id := args[0]
		ctx := cmd.Context()

		// Try node first, then edge, then tag, then proposal
		node, err := svc.graph.GetNode(ctx, id)
		if err == nil {
			output(node)
			return nil
		}

		edge, err := svc.graph.GetEdge(ctx, id)
		if err == nil {
			output(edge)
			return nil
		}

		t, err := svc.tag.GetTag(ctx, id)
		if err == nil {
			output(t)
			return nil
		}

		p, err := svc.proposal.Get(ctx, id)
		if err == nil {
			output(p)
			return nil
		}

		return fmt.Errorf("%w: no entity with id %s", model.ErrNotFound, id)
	},
}
