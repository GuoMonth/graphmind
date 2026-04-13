package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Multi-operation proposal from stdin",
	Long: `Create a multi-operation proposal from a JSON array on stdin.

This is the primary way to make complex atomic changes. All operations
in the batch are committed or rejected as a single unit.

STDIN FORMAT

  A JSON array of operation objects. Each object has:

  - "command": one of "add", "ln", "tag", "mv", "rm"
  - "data": the operation payload (same fields as the individual command)

CROSS-REFERENCES

  Within a batch, later operations can reference entities created by
  earlier operations using zero-based index references instead of IDs:

  - "from_reference": <index>   (for ln command, instead of from_id)
  - "to_reference": <index>     (for ln command, instead of to_id)
  - "reference": <index>        (for tag/mv/rm, instead of node_id/id)

  For "rm", set "entity": "edge" to delete an edge (default: "node").

EXAMPLES

  # Create two nodes and link them in one atomic proposal:
  $ cat <<'EOF' | gm batch
  [
    {"command": "add", "data": {"type": "task", "title": "Design API"}},
    {"command": "add", "data": {"type": "task", "title": "Implement API"}},
    {"command": "ln", "data": {"type": "depends_on", "from_reference": 1, "to_reference": 0}},
    {"command": "tag", "data": {"reference": 0, "tag_name": "api"}},
    {"command": "tag", "data": {"reference": 1, "tag_name": "api"}}
  ]
  EOF
  $ gm commit <proposal-id>

  # Update and tag in one batch:
  $ echo '[{"command":"mv","data":{"id":"019abc...","status":"done"}},
           {"command":"tag","data":{"node_id":"019abc...","tag_name":"completed"}}]' \
    | gm batch

OUTPUT

  Success (pending proposal with all operations):
  {"ok":true,"data":{"id":"<proposal-id>","status":"pending","operations":[...],...}}

  Error — invalid command:
  {"ok":false,"error":{"code":"INVALID_INPUT","message":"invalid input: unknown batch command \"foo\""}}`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		const maxStdinBytes = 10 << 20 // 10 MB
		input, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
		if err != nil {
			return fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)
		}
		if len(input) > maxStdinBytes {
			return fmt.Errorf("%w: stdin exceeds 10 MB limit", model.ErrInvalidInput)
		}
		if len(input) == 0 {
			return fmt.Errorf("%w: stdin is empty, expected JSON array", model.ErrInvalidInput)
		}

		var rawOps []struct {
			Command string         `json:"command"`
			Data    map[string]any `json:"data"` // JSON operation payload
		}
		if err := json.Unmarshal(input, &rawOps); err != nil {
			return fmt.Errorf("%w: invalid JSON: %s", model.ErrInvalidInput, err)
		}

		if len(rawOps) == 0 {
			return fmt.Errorf("%w: empty operations array", model.ErrInvalidInput)
		}

		ops := make([]model.ProposalOperation, 0, len(rawOps))
		for i, raw := range rawOps {
			op, err := batchCommandToOp(raw.Command, raw.Data, i)
			if err != nil {
				return err
			}
			ops = append(ops, op)
		}

		p, err := svc.proposal.Create(cmd.Context(), ops)
		if err != nil {
			return err
		}

		output(p)
		return nil
	},
}

// batchCommandToOp converts a batch command name to a ProposalOperation.
func batchCommandToOp(command string, data map[string]any, index int) (model.ProposalOperation, error) {
	if data == nil {
		data = map[string]any{} // ensure non-nil map
	}

	switch command {
	case "add":
		nodeType, _ := data["type"].(string)
		title, _ := data["title"].(string)
		return model.ProposalOperation{
			Action:  model.OpCreateNode,
			Entity:  "node",
			Data:    data,
			Summary: fmt.Sprintf("%s: %s", nodeType, title),
		}, nil

	case "ln":
		edgeType, _ := data["type"].(string)
		return model.ProposalOperation{
			Action:  model.OpCreateEdge,
			Entity:  "edge",
			Data:    data,
			Summary: fmt.Sprintf("link: %s", edgeType),
		}, nil

	case "tag":
		tagName, _ := data["tag_name"].(string)
		return model.ProposalOperation{
			Action:  model.OpTagNode,
			Entity:  "tag",
			Data:    data,
			Summary: fmt.Sprintf("tag: %s", tagName),
		}, nil

	case "mv":
		id, _ := data["id"].(string)
		return model.ProposalOperation{
			Action:  model.OpUpdateNode,
			Entity:  "node",
			Data:    data,
			Summary: fmt.Sprintf("update: %s", truncate(id)),
		}, nil

	case "rm":
		id, _ := data["id"].(string)
		// Require explicit entity type, or default to node
		entity, _ := data["entity"].(string)
		switch entity {
		case "edge":
			return model.ProposalOperation{
				Action:  model.OpDeleteEdge,
				Entity:  "edge",
				Data:    data,
				Summary: fmt.Sprintf("delete edge: %s", truncate(id)),
			}, nil
		case "node", "":
			return model.ProposalOperation{
				Action:  model.OpDeleteNode,
				Entity:  "node",
				Data:    data,
				Summary: fmt.Sprintf("delete node: %s", truncate(id)),
			}, nil
		default:
			return model.ProposalOperation{}, fmt.Errorf(
				"%w: rm entity must be \"node\" or \"edge\", got %q at index %d",
				model.ErrInvalidInput, entity, index,
			)
		}

	default:
		return model.ProposalOperation{}, fmt.Errorf(
			"%w: unknown batch command %q at index %d", model.ErrInvalidInput, command, index,
		)
	}
}
