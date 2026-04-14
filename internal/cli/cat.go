package cli

import (
	"errors"
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var catCmd = &cobra.Command{
	Use:   "cat <id>",
	Short: "Show full detail of one entity",
	Long: `Show the full detail of a single entity by its UUID.

Auto-detects the entity type by trying lookups in order:
node → edge → tag → proposal. Returns the first match.

Use this to inspect any entity after listing with "gm ls".`,
	Example: `  # Show a node
  gm cat 019abc...

  # Show with pretty-printed JSON
  gm cat 019abc... --pretty

  # Output (node example):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019abc...",
  #     "type": "task",
  #     "title": "Build auth module",
  #     "description": "Implement JWT-based authentication",
  #     "status": "active",
  #     "properties": {},
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:30:00.000Z"
  #   }
  # }

  # Output (edge example):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019def...",
  #     "type": "depends_on",
  #     "from_id": "019abc...",
  #     "to_id": "019ghi...",
  #     "properties": {},
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:30:00.000Z"
  #   }
  # }

  # Output (proposal example):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019jkl...",
  #     "status": "committed",
  #     "operations": [
  #       {"action":"create_node","entity":"node","data":{...},"summary":"task: Build auth module"}
  #     ],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:31:00.000Z"
  #   }
  # }

  # Error — not found:
  # {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: no entity with id 019..."}}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		ctx := cmd.Context()

		// Try node first, then edge, then tag, then proposal.
		// Only fall through on ErrNotFound; surface all other errors.
		node, err := svc.graph.GetNode(ctx, id)
		if err == nil {
			outputSuccess(node,
				fmt.Sprintf("Retrieved %s node %q (id: %s).", node.Type, node.Title, truncate(node.ID)),
				[]string{
					fmt.Sprintf("gm mv %s --status <new>  — update this node", id),
					fmt.Sprintf("gm ln %s <other-id> --type <type>  — link to another node", id),
					fmt.Sprintf("gm tag %s <tag-name>  — attach a tag", id),
					fmt.Sprintf("gm rm %s  — delete this node", id),
					fmt.Sprintf("gm log --entity-id %s  — view this node's event history", id),
				},
			)
			return nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}

		edge, err := svc.graph.GetEdge(ctx, id)
		if err == nil {
			outputSuccess(edge,
				fmt.Sprintf("Retrieved %s edge (id: %s, from: %s → to: %s).",
					edge.Type, truncate(edge.ID), truncate(edge.FromID), truncate(edge.ToID)),
				[]string{
					fmt.Sprintf("gm cat %s  — inspect the source node", edge.FromID),
					fmt.Sprintf("gm cat %s  — inspect the target node", edge.ToID),
					fmt.Sprintf("gm rm %s  — delete this edge", id),
				},
			)
			return nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}

		t, err := svc.tag.GetTag(ctx, id)
		if err == nil {
			outputSuccess(t,
				fmt.Sprintf("Retrieved tag %q (id: %s).", t.Name, truncate(t.ID)),
				[]string{
					"gm ls node  — list nodes (filter by tag coming soon)",
					fmt.Sprintf("gm log --entity-id %s  — view tag event history", id),
				},
			)
			return nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}

		p, err := svc.proposal.Get(ctx, id)
		if err == nil {
			next := []string{
				fmt.Sprintf("gm log --entity-id %s  — view proposal events", id),
			}
			if p.Status == "pending" {
				next = append([]string{
					fmt.Sprintf("gm commit %s  — apply this proposal", id),
					fmt.Sprintf("gm reject %s  — discard this proposal", id),
				}, next...)
			}
			outputSuccess(p,
				fmt.Sprintf("Retrieved %s proposal %s with %d %s.",
					p.Status, truncate(p.ID), len(p.Operations),
					pluralize("operation", "operations", len(p.Operations))),
				next,
			)
			return nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}

		return model.WithHint(
			fmt.Errorf("%w: no entity with id %s", model.ErrNotFound, id),
			"Use 'gm ls node', 'gm ls edge', 'gm ls tag', or 'gm ls proposal' to find valid IDs.",
		)
	},
}
