package cli

import (
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit <proposal-id>",
	Short: "Commit a pending proposal",
	Long:  "Apply all operations in a pending proposal atomically.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(); err != nil {
			return err
		}

		p, err := svc.proposal.Commit(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		output(p)
		return nil
	},
}

var rejectCmd = &cobra.Command{
	Use:   "reject <proposal-id>",
	Short: "Reject a pending proposal",
	Long:  "Discard all operations in a pending proposal.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(); err != nil {
			return err
		}

		p, err := svc.proposal.Reject(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		output(p)
		return nil
	},
}
