package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var (
	mvTitle       string
	mvDescription string
	mvStatus      string
	mvType        string
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
  --type          New node type (must be a valid type)

STDIN JSON (alternative to flags, takes priority)

  Provide a partial JSON object. Only included fields are updated:

  {"status": "done", "properties": {"completed_at": "2026-04-13"}}

EXAMPLES

  # Update status via flag:
  $ gm mv 019abc... --status done

  # Update multiple fields:
  $ gm mv 019abc... --title "Renamed task" --status in_progress

  # Update via stdin JSON (partial — only status changes):
  $ echo '{"status":"done"}' | gm mv 019abc...

  # Update with properties merge:
  $ echo '{"properties":{"priority":"high","estimate":"2h"}}' | gm mv 019abc...

OUTPUT

  Success (pending proposal):
  {"ok":true,"data":{"id":"<proposal-id>","status":"pending",
    "operations":[{"action":"update_node","entity":"node",
    "data":{...},"summary":"update: <title>"}],...}}

  Error — node not found:
  {"ok":false,"error":{"code":"NOT_FOUND","message":"not found: node"}}`,
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
			if !model.IsValidNodeType(mvType) {
				return fmt.Errorf("%w: invalid node type %q", model.ErrInvalidInput, mvType)
			}
			data["type"] = mvType
		}

		// If stdin has data, use it (merged with id)
		stat, err := os.Stdin.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)
			}
			if len(input) > 0 {
				var stdinData map[string]any // JSON input is untyped
				if err := json.Unmarshal(input, &stdinData); err != nil {
					return fmt.Errorf("%w: invalid JSON: %s", model.ErrInvalidInput, err)
				}
				stdinData["id"] = id
				data = stdinData
			}
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

		output(p)
		return nil
	},
}

func init() {
	mvCmd.Flags().StringVar(&mvTitle, "title", "", "New title")
	mvCmd.Flags().StringVar(&mvDescription, "description", "", "New description")
	mvCmd.Flags().StringVar(&mvStatus, "status", "", "New status")
	mvCmd.Flags().StringVar(&mvType, "type", "", "New node type")
}
