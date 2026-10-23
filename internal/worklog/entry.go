package worklog

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+-\d+$`)

// Worklog entry ready to send to Jira
type Entry struct {
	IssueKey string
	Duration time.Duration
	Comment  string
	Started  time.Time
}

// Validates user input and builds an Entry
func NewEntry(issueKey, duration, comment string, started time.Time) (Entry, error) {
	key := strings.ToUpper(strings.TrimSpace(issueKey))
	if !issueKeyPattern.MatchString(key) {
		return Entry{}, fmt.Errorf("invalid issue key %q (expected ex. DEMO-123)", issueKey)
	}

	d, err := ParseDuration(duration)
	if err != nil {
		return Entry{}, err
	}

	return Entry{
		IssueKey: key,
		Duration: d,
		Comment:  strings.TrimSpace(comment),
		Started:  started,
	}, nil
}
