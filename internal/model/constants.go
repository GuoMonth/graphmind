package model

// Envelope is the standard JSON output wrapper.
type Envelope struct {
	OK        bool       `json:"ok"`
	Data      any        `json:"data,omitempty"`       // present on success
	Summary   string     `json:"summary,omitempty"`    // what was accomplished (success only)
	NextSteps []string   `json:"next_steps,omitempty"` // suggested follow-up actions
	Error     *ErrorBody `json:"error,omitempty"`      // present on failure
}

// ErrorBody is the structured error in the envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Proposal statuses
const (
	ProposalStatusPending   = "pending"
	ProposalStatusCommitted = "committed"
	ProposalStatusRejected  = "rejected"
)

// Event actions
const (
	ActionNodeCreated       = "node_created"
	ActionNodeUpdated       = "node_updated"
	ActionNodeDeleted       = "node_deleted"
	ActionEdgeCreated       = "edge_created"
	ActionEdgeDeleted       = "edge_deleted"
	ActionTagCreated        = "tag_created"
	ActionTagUpdated        = "tag_updated"
	ActionTagDeleted        = "tag_deleted"
	ActionNodeTagged        = "node_tagged"
	ActionNodeUntagged      = "node_untagged"
	ActionTagEdgeCreated    = "tag_edge_created"
	ActionTagEdgeDeleted    = "tag_edge_deleted"
	ActionProposalCreated   = "proposal_created"
	ActionProposalCommitted = "proposal_committed"
	ActionProposalRejected  = "proposal_rejected"
)

// Proposal operation actions (used in ProposalOperation.Action)
const (
	OpCreateNode    = "create_node"
	OpCreateEdge    = "create_edge"
	OpCreateTagEdge = "create_tag_edge"
	OpTagNode       = "tag_node"
	OpUpdateNode    = "update_node"
	OpDeleteNode    = "delete_node"
	OpDeleteEdge    = "delete_edge"
	OpDeleteTagEdge = "delete_tag_edge"
)
