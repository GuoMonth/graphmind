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
	Long:  "Create a new node in the project graph. Returns a pending proposal.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := wireAndMigrate(); err != nil {
			os.Exit(outputError(err))
			return nil
		}
		defer svc.db.Close()

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
				os.Exit(outputError(fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)))
				return nil
			}
			if len(input) > 0 {
				var stdinData map[string]any // JSON input is untyped
				if err := json.Unmarshal(input, &stdinData); err != nil {
					os.Exit(outputError(fmt.Errorf("%w: invalid JSON: %s", model.ErrInvalidInput, err)))
					return nil
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
			os.Exit(outputError(err))
			return nil
		}

		output(p)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addType, "type", "", "Node type (required)")
	addCmd.Flags().StringVar(&addTitle, "title", "", "Node title (required)")
	addCmd.Flags().StringVar(&addDescription, "description", "", "Node description")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status")
}
