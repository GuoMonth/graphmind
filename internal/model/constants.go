package model

// Envelope is the standard JSON output wrapper.
type Envelope struct {
	OK    bool        `json:"ok"`
	Data  any         `json:"data,omitempty"`  // present on success
	Error *ErrorBody  `json:"error,omitempty"` // present on failure
}

// ErrorBody is the structured error in the envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Node types
const (
	NodeTypeTask       = "task"
	NodeTypeEpic       = "epic"
	NodeTypeDecision   = "decision"
	NodeTypeRisk       = "risk"
	NodeTypeRelease    = "release"
	NodeTypeDiscussion = "discussion"
)

// Edge types
const (
	EdgeTypeDependsOn  = "depends_on"
	EdgeTypeBlocks     = "blocks"
	EdgeTypeDecompose  = "decompose"
	EdgeTypeCausedBy   = "caused_by"
	EdgeTypeRelatedTo  = "related_to"
	EdgeTypeSupersedes = "supersedes"
)

// Proposal statuses
const (
	ProposalStatusPending   = "pending"
	ProposalStatusCommitted = "committed"
	ProposalStatusRejected  = "rejected"
)

// Event actions
const (
	ActionNodeCreated    = "node_created"
	ActionNodeUpdated    = "node_updated"
	ActionNodeDeleted    = "node_deleted"
	ActionEdgeCreated    = "edge_created"
	ActionEdgeDeleted    = "edge_deleted"
	ActionTagCreated     = "tag_created"
	ActionTagUpdated     = "tag_updated"
	ActionTagDeleted     = "tag_deleted"
	ActionNodeTagged     = "node_tagged"
	ActionNodeUntagged   = "node_untagged"
	ActionProposalCreated   = "proposal_created"
	ActionProposalCommitted = "proposal_committed"
	ActionProposalRejected  = "proposal_rejected"
)

// Proposal operation actions (used in ProposalOperation.Action)
const (
	OpCreateNode = "create_node"
	OpCreateEdge = "create_edge"
	OpTagNode    = "tag_node"
)

// ValidNodeTypes is the set of allowed node types.
var ValidNodeTypes = map[string]bool{
	NodeTypeTask:       true,
	NodeTypeEpic:       true,
	NodeTypeDecision:   true,
	NodeTypeRisk:       true,
	NodeTypeRelease:    true,
	NodeTypeDiscussion: true,
}

// ValidEdgeTypes is the set of allowed edge types.
var ValidEdgeTypes = map[string]bool{
	EdgeTypeDependsOn:  true,
	EdgeTypeBlocks:     true,
	EdgeTypeDecompose:  true,
	EdgeTypeCausedBy:   true,
	EdgeTypeRelatedTo:  true,
	EdgeTypeSupersedes: true,
}

// DirectionalEdgeTypes are edge types that should be checked for cycles.
var DirectionalEdgeTypes = map[string]bool{
	EdgeTypeDependsOn: true,
	EdgeTypeBlocks:    true,
	EdgeTypeDecompose: true,
	EdgeTypeCausedBy:  true,
	EdgeTypeSupersedes: true,
}
