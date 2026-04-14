package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit <proposal-id>",
	Short: "Commit a pending proposal",
	Long: `Apply all operations in a pending proposal atomically.

All operations in the proposal are executed in a single SQLite transaction.
If any operation fails (e.g., node type invalid, edge creates cycle),
the entire proposal is rolled back and nothing changes.

After commit, the proposal status changes to "committed".
Only proposals with status "pending" can be committed.`,
	Example: `  # Commit a proposal
  gm commit 019abc...

  # Typical workflow: add then commit
  PROPOSAL=$(gm add --type task --title "New task" | jq -r '.data.id')
  gm commit "$PROPOSAL"

  # Output on success:
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019abc...",
  #     "status": "committed",
  #     "operations": [...],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:31:00.000Z"
  #   }
  # }

  # Error — already committed:
  # {"ok":false,"error":{"code":"CONFLICT",
  #   "message":"invalid state: proposal is committed, not pending"}}

  # Error — proposal not found:
  # {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: proposal"}}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := svc.proposal.Commit(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Committed proposal %s: %d %s applied atomically.",
				truncate(p.ID), len(p.Operations), pluralize("operation", "operations", len(p.Operations))),
			[]string{
				"gm ls node  — list nodes to see the changes",
				"gm log --since 1h  — view recent event history",
				fmt.Sprintf("gm cat %s  — review the committed proposal", p.ID),
			},
		)
		return nil
	},
}

var rejectCmd = &cobra.Command{
	Use:   "reject <proposal-id>",
	Short: "Reject a pending proposal",
	Long: `Discard all operations in a pending proposal.

The proposal status changes to "rejected". No graph mutations occur.
Only proposals with status "pending" can be rejected.`,
	Example: `  # Reject a proposal
  gm reject 019abc...

  # Output on success:
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019abc...",
  #     "status": "rejected",
  #     "operations": [...],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:31:00.000Z"
  #   }
  # }

  # Error — already committed/rejected:
  # {"ok":false,"error":{"code":"CONFLICT",
  #   "message":"invalid state: proposal is committed, not pending"}}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := svc.proposal.Reject(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Rejected proposal %s: all %d %s discarded.",
				truncate(p.ID), len(p.Operations), pluralize("operation", "operations", len(p.Operations))),
			[]string{
				"gm ls proposal --status pending  — check remaining pending proposals",
				"gm add / gm ln / gm tag / gm batch  — create new proposals",
			},
		)
		return nil
	},
}
