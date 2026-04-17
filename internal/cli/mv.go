package cli

import (
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var (
	mvTitle       string
	mvDescription string
	mvStatus      string
	mvType        string
	mvWho         string
	mvWhere       string
	mvEventTime   string
)

var mvCmd = &cobra.Command{
	Use:   "mv <id>",
	Short: "Update a node → proposal",
	Long: `Update an existing node's fields. Returns a pending proposal.

Only the provided fields are updated (partial update). Omitted fields keep
their current values. Properties are merged: new keys are added, existing
keys are overwritten, unmentioned keys are preserved.

ACCEPTED FLAGS

  --title         New title
  --description   New description
  --status        New status
  --type          New node type (open string)
  --who           Who was involved
  --where         Where it happened
  --event-time    When it happened (free-form)

STDIN JSON (alternative to flags, takes priority)

  Provide a partial JSON object. Only included fields are updated:

  {"status": "done", "who": "Alice", "event_time": "2025-01-15"}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		data := map[string]any{"id": id} // proposal operation data uses any

		// Collect flags that were explicitly set
		if cmd.Flags().Changed("title") {
			data["title"] = mvTitle
		}
		if cmd.Flags().Changed("description") {
			data["description"] = mvDescription
		}
		if cmd.Flags().Changed("status") {
			data["status"] = mvStatus
		}
		if cmd.Flags().Changed("type") {
			data["type"] = mvType
		}
		if cmd.Flags().Changed("who") {
			data["who"] = mvWho
		}
		if cmd.Flags().Changed("where") {
			data["where"] = mvWhere
		}
		if cmd.Flags().Changed("event-time") {
			data["event_time"] = mvEventTime
		}

		// If stdin has data, use it (merged with id)
		stdinData, hasInput, err := readOptionalJSONObjectFromStdin(os.Stdin)
		if err != nil {
			return err
		}
		if hasInput {
			stdinData["id"] = id
			data = stdinData
		}

		summary := "update: " + id[:min(8, len(id))]
		if t, ok := data["title"].(string); ok && t != "" {
			summary = "update: " + t
		}

		op := model.ProposalOperation{
			Action:  model.OpUpdateNode,
			Entity:  "node",
			Data:    data,
			Summary: summary,
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Created pending proposal %s: update node %s.", truncate(p.ID), truncate(id)),
			proposalNextSteps(p.ID),
		)
		return nil
	},
}

func init() {
	mvCmd.Flags().StringVar(&mvTitle, "title", "", "New title")
	mvCmd.Flags().StringVar(&mvDescription, "description", "", "New description")
	mvCmd.Flags().StringVar(&mvStatus, "status", "", "New status")
	mvCmd.Flags().StringVar(&mvType, "type", "", "New node type")
	mvCmd.Flags().StringVar(&mvWho, "who", "", "Who was involved")
	mvCmd.Flags().StringVar(&mvWhere, "where", "", "Where it happened")
	mvCmd.Flags().StringVar(&mvEventTime, "event-time", "", "When it happened (free-form)")
}
