package model

import (
	"fmt"
	"time"
)

// TimeFormat is the canonical timestamp layout used in SQLite and JSON output.
const TimeFormat = "2006-01-02T15:04:05.000Z"

// ParseTime parses a timestamp string in the canonical format.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeFormat, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}
