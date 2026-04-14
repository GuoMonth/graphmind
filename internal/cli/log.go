package cli

import (
	"fmt"
	"time"

	"github.com/senguoyun-guosheng/graphmind/internal/event"
	"github.com/senguoyun-guosheng/graphmind/internal/model"
	"github.com/spf13/cobra"
)

var (
	logEntityID string
	logAction   string
	logSince    string
	logLimit    int
	logAfter    string
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View event history",
	Long: `View the event history (audit log) of the graph.

Every mutation (node created, edge deleted, proposal committed, etc.)
is recorded as an immutable event. This command queries that event stream.

FILTERS

  --entity-id <id>    Events for a specific entity
  --type <action>     Filter by event type (e.g. node_created, proposal_committed)
  --since <duration>  Events within a time window (e.g. 1h, 24h, 7d, 30d)
  --limit <n>         Max results (default 50, max 1000)
  --after <id>        Cursor for pagination

EVENT TYPES

  node_created, node_updated, node_deleted,
  edge_created, edge_deleted,
  tag_created, tag_updated, tag_deleted,
  node_tagged, node_untagged,
  proposal_created, proposal_committed, proposal_rejected

EXAMPLES

  # Recent events (default limit 50):
  $ gm log

  # Events for a specific entity:
  $ gm log --entity-id 019abc...

  # Only node creation events in the last 24 hours:
  $ gm log --type node_created --since 24h

  # Paginate through events:
  $ gm log --limit 10
  $ gm log --limit 10 --after 019abc...

OUTPUT

  Success (array of events, newest first):
  {"ok":true,"data":[
    {"id":"019...","entity_type":"node","entity_id":"019...",
     "action":"node_created","payload":"{...}",
     "created_at":"2026-04-13T..."},
    ...
  ]}
  No events:
  {"ok":true,"data":[]}`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		const maxLimit = 1000
		if logLimit > maxLimit {
			logLimit = maxLimit
		}

		filter := event.ListFilter{
			EntityID: logEntityID,
			Action:   logAction,
			Limit:    logLimit,
			After:    logAfter,
		}

		// Parse --since as Go duration and convert to ISO 8601 timestamp
		if logSince != "" {
			d, err := parseDuration(logSince)
			if err != nil {
				return fmt.Errorf("%w: invalid --since duration %q (use e.g. 1h, 24h, 7d)", model.ErrInvalidInput, logSince)
			}
			filter.Since = time.Now().UTC().Add(-d).Format("2006-01-02T15:04:05.000Z")
		}

		events, err := svc.event.List(cmd.Context(), &filter)
		if err != nil {
			return err
		}

		summary := fmt.Sprintf("Retrieved %d %s.", len(events), pluralize("event", "events", len(events)))
		next := []string{
			"gm cat <entity-id>  — inspect an entity referenced in events",
			"gm log --entity-id <id>  — filter events for a specific entity",
			"gm log --type <action> --since <duration>  — narrow by action and time window",
		}
		if len(events) == logLimit {
			next = append(next,
				fmt.Sprintf("gm log --limit %d --after %s  — next page", logLimit, events[len(events)-1].ID))
		}
		outputSuccess(events, summary, next)
		return nil
	},
}

// parseDuration extends time.ParseDuration to support "d" (days) suffix.
func parseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		// Parse number of days
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func init() {
	logCmd.Flags().StringVar(&logEntityID, "entity-id", "", "Filter events by entity ID")
	logCmd.Flags().StringVar(&logAction, "type", "", "Filter by event type (e.g. node_created)")
	logCmd.Flags().StringVar(&logSince, "since", "", "Events within duration (e.g. 1h, 24h, 7d)")
	logCmd.Flags().IntVar(&logLimit, "limit", 50, "Max results (default 50, max 1000)")
	logCmd.Flags().StringVar(&logAfter, "after", "", "Cursor for pagination")
}
