package worklog

import (
	"testing"
	"time"
)

func TestNewEntry(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	e, err := NewEntry(" demo-12 ", "1h 30m", " standup ", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.IssueKey != "DEMO-12" || e.Duration != 90*time.Minute || e.Comment != "standup" || !e.Started.Equal(now) {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestNewEntryInvalid(t *testing.T) {
	now := time.Now()
	tests := []struct{ key, dur string }{
		{"", "1h"},
		{"DEMO", "1h"},
		{"123-4", "1h"},
		{"DEMO-1", ""},
		{"DEMO-1", "soon"},
	}
	for _, tt := range tests {
		if _, err := NewEntry(tt.key, tt.dur, "", now); err == nil {
			t.Errorf("NewEntry(%q, %q) expected error", tt.key, tt.dur)
		}
	}
}
