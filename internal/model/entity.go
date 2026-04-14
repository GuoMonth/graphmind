package model

import (
	"time"
)

// Node represents a vertex in the graph — an event, person, place, or any recorded entity.
type Node struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Who         string         `json:"who,omitempty"`
	Where       string         `json:"where,omitempty"`
	EventTime   string         `json:"event_time,omitempty"`
	Properties  map[string]any `json:"properties"` // JSON flexible properties
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	FromID     string         `json:"from_id"`
	ToID       string         `json:"to_id"`
	Properties map[string]any `json:"properties"` // JSON flexible properties
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Tag represents an AI-extracted semantic label.
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Event represents an immutable record of a past mutation.
type Event struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Action     string    `json:"action"`
	Payload    string    `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// Proposal represents a staged batch of changes awaiting confirmation.
type Proposal struct {
	ID         string              `json:"id"`
	Status     string              `json:"status"`
	Operations []ProposalOperation `json:"operations"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// ProposalOperation represents a single operation within a proposal.
type ProposalOperation struct {
	Action  string         `json:"action"`
	Entity  string         `json:"entity"`
	Data    map[string]any `json:"data"` // operation-specific payload
	Summary string         `json:"summary"`
}

// NodeTag represents the association between a node and a tag.
type NodeTag struct {
	NodeID string `json:"node_id"`
	TagID  string `json:"tag_id"`
}

// TagEdge represents a directed relationship between two tags.
type TagEdge struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	FromID     string         `json:"from_id"`
	ToID       string         `json:"to_id"`
	Properties map[string]any `json:"properties"` // JSON flexible properties
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
