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
	addType        string
	addTitle       string
	addDescription string
	addStatus      string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a node → proposal",
	Long: `Create a new node in the project graph. Returns a pending proposal.

The node is NOT created immediately. A proposal is returned with status
"pending". Call "gm commit <proposal-id>" to apply it.

Accepts input via flags OR via JSON on stdin (stdin takes priority).

FLAGS
  --type         Node type (required). One of: task, epic, decision, risk, release, discussion
  --title        Node title (required)
  --description  Optional description text
  --status       Optional initial status string

STDIN FORMAT
  When piping JSON, provide a single object with the same field names:
  {"type":"task","title":"My task","description":"...","status":"open"}`,
	Example: `  # Create via flags
  gm add --type task --title "Build auth module"

  # Create via flags with all fields
  gm add --type epic --title "Q3 Roadmap" --description "All Q3 deliverables" --status "active"

  # Create via stdin (useful for AI-generated content)
  echo '{"type":"task","title":"Fix bug #42","description":"NPE in login flow"}' | gm add

  # Output (proposal with status "pending"):
  # {
  #   "ok": true,
  #   "data": {
  #     "id": "019...",
  #     "status": "pending",
  #     "operations": [
  #       {
  #         "action": "create_node",
  #         "entity": "node",
  #         "data": {"type":"task","title":"Build auth module","description":"","status":""},
  #         "summary": "task: Build auth module"
  #       }
  #     ],
  #     "created_at": "2025-01-15T10:30:00.000Z",
  #     "updated_at": "2025-01-15T10:30:00.000Z"
  #   }
  # }

  # Error — missing type:
  # {"ok":false,"error":{"code":"INVALID_INPUT","message":"invalid input: node type required"}}`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := wireAndMigrate(cmd.Context()); err != nil {
			return err
		}

		data := map[string]any{ // proposal operation data uses any
			"type":        addType,
			"title":       addTitle,
			"description": addDescription,
			"status":      addStatus,
		}

		// If stdin has data, use it instead of flags
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)
			}
			if len(input) > 0 {
				var stdinData map[string]any // JSON input is untyped
				if err := json.Unmarshal(input, &stdinData); err != nil {
					return fmt.Errorf("%w: invalid JSON: %s", model.ErrInvalidInput, err)
				}
				data = stdinData
			}
		}

		title, _ := data["title"].(string)
		nodeType, _ := data["type"].(string)
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

		output(p)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addType, "type", "", "Node type (required unless piped via stdin)")
	addCmd.Flags().StringVar(&addTitle, "title", "", "Node title (required unless piped via stdin)")
	addCmd.Flags().StringVar(&addDescription, "description", "", "Node description")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status")
}
