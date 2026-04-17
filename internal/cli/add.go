package cli

import (
	"fmt"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var (
	addType        string
	addTitle       string
	addDescription string
	addStatus      string
	addWho         string
	addWhere       string
	addEventTime   string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a node → proposal",
	Long: `Create a new node in the graph. Returns a pending proposal.

The node is NOT created immediately. A proposal is returned with status
"pending". Call "gm commit <proposal-id>" to apply it.

Accepts input via flags OR via JSON on stdin (stdin takes priority).

FLAGS
  --type         Node type (required). Open string — common types: event, person, place
  --title        Node title (required)
  --description  Optional description text
  --status       Optional initial status string
  --who          Who was involved (free-form)
  --where        Where it happened (free-form)
  --event-time   When it happened (free-form: "summer 2024", "last Tuesday", ISO 8601)

STDIN FORMAT
  When piping JSON, provide a single object with the same field names:
  {"type":"event","title":"Team lunch","who":"Alice, Bob","event_time":"2025-01-15"}`,
	Example: `  # Create via flags
  gm add --type event --title "Team lunch" --who "Alice" --event-time "last Friday"

  # Create via stdin (useful for AI-generated content)
  echo '{"type":"event","title":"Sprint planning","who":"team","where":"Room A"}' | gm add`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		data := map[string]any{ // proposal operation data uses any
			"type":        addType,
			"title":       addTitle,
			"description": addDescription,
			"status":      addStatus,
			"who":         addWho,
			"where":       addWhere,
			"event_time":  addEventTime,
		}

		// If stdin has data, use it instead of flags
		stdinData, hasInput, err := readOptionalJSONObjectFromStdin(os.Stdin)
		if err != nil {
			return err
		}
		if hasInput {
			data = stdinData
		}

		nodeType, _ := data["type"].(string)
		if nodeType == "" {
			return model.WithHint(
				fmt.Errorf("%w: type is required", model.ErrInvalidInput),
				"Provide --type <type>. Common types: event, person, place. Example: gm add --type event --title \"...\"",
			)
		}

		title, _ := data["title"].(string)
		op := model.ProposalOperation{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    data,
			Summary: fmt.Sprintf("%s: %s", nodeType, title),
		}

		p, err := svc.proposal.Create(cmd.Context(), []model.ProposalOperation{op})
		if err != nil {
			return err
		}

		outputSuccess(p,
			fmt.Sprintf("Created pending proposal %s: create %s node %q.", truncate(p.ID), nodeType, title),
			proposalNextSteps(p.ID),
		)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addType, "type", "", "Node type (required unless piped via stdin)")
	addCmd.Flags().StringVar(&addTitle, "title", "", "Node title (required unless piped via stdin)")
	addCmd.Flags().StringVar(&addDescription, "description", "", "Node description")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status")
	addCmd.Flags().StringVar(&addWho, "who", "", "Who was involved")
	addCmd.Flags().StringVar(&addWhere, "where", "", "Where it happened")
	addCmd.Flags().StringVar(&addEventTime, "event-time", "", "When it happened (free-form)")
}
