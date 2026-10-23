package ui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

// Worklog started d days after this week's Monday, at hour h
func logAt(key string, d, h int, dur time.Duration) jira.Worklog {
	mon, _ := worklog.WeekRange(time.Now(), 0)
	return jira.Worklog{IssueKey: key, Started: mon.AddDate(0, 0, d).Add(time.Duration(h) * time.Hour), Duration: dur}
}

func TestGroupByIssue(t *testing.T) {
	rows := groupByIssue([]jira.Worklog{
		logAt("DEMO-1", 0, 9, 15*time.Minute),
		logAt("DEMO-2", 0, 10, 2*time.Hour),
		logAt("DEMO-1", 0, 16, 15*time.Minute),
		logAt("DEMO-1", 2, 9, 15*time.Minute),
		logAt("DEMO-3", 6, 12, time.Hour),
	})

	if len(rows) != 3 || rows[0].key != "DEMO-1" || rows[1].key != "DEMO-2" || rows[2].key != "DEMO-3" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].days[0] != 30*time.Minute || rows[0].days[2] != 15*time.Minute || rows[0].total() != 45*time.Minute {
		t.Errorf("DEMO-1 = %+v", rows[0])
	}
	if rows[2].days[6] != time.Hour {
		t.Errorf("DEMO-3 sunday = %v", rows[2].days[6])
	}
}

func TestFormatHours(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "-"},
		{15 * time.Minute, "0.3"},
		{30 * time.Minute, "0.5"},
		{90 * time.Minute, "1.5"},
		{7*time.Hour + 45*time.Minute, "7.8"},
		{8 * time.Hour, "8.0"},
		{40 * time.Hour, "40.0"},
	}
	for _, tt := range tests {
		if got := formatHours(tt.in); got != tt.want {
			t.Errorf("formatHours(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTimesheetWeekKeys(t *testing.T) {
	keyLeft := tea.KeyMsg{Type: tea.KeyLeft}
	steps := []struct {
		key  tea.KeyMsg
		want int
	}{
		{keyLeft, -1},
		{keyLeft, -2},
		{keyRight, -1},
		{typed("t"), 0},
		{typed("r"), 0},
	}

	p := newTimesheet()
	for _, s := range steps {
		var cmd tea.Cmd
		p, cmd = p.Update(s.key)
		if p.offset != s.want || !p.loading || cmd == nil {
			t.Errorf("after %q: offset=%d loading=%v, want offset %d and a reload", s.key.String(), p.offset, p.loading, s.want)
		}
	}
}

func TestTimesheetIgnoresStaleWeek(t *testing.T) {
	p := newTimesheet()
	p.offset, p.loading = -1, true

	// Result for this week arrives after the user moved to last week
	p, _ = p.Update(weekLoadedMsg{offset: 0, logs: []jira.Worklog{logAt("DEMO-1", 0, 9, time.Hour)}})
	if !p.loading || len(p.rows) != 0 {
		t.Error("stale result should be ignored")
	}

	p, _ = p.Update(weekLoadedMsg{offset: -1, logs: []jira.Worklog{logAt("DEMO-1", 0, 9, time.Hour)}})
	if p.loading || len(p.rows) != 1 {
		t.Error("current result should be shown")
	}
}

func TestTimesheetError(t *testing.T) {
	p := newTimesheet()
	p.loading = true
	p, _ = p.Update(weekLoadedMsg{err: errors.New("Authentication failed")})

	if p.loading || !strings.Contains(p.View(40, 8), "Authentication failed") {
		t.Errorf("error not shown: %q", p.View(40, 8))
	}
}

func TestTimesheetView(t *testing.T) {
	p := newTimesheet()
	p, _ = p.Update(weekLoadedMsg{logs: []jira.Worklog{
		logAt("DEMO-1", 0, 9, 15*time.Minute),
		logAt("DEMO-2", 0, 10, 7*time.Hour+45*time.Minute),
		logAt("DEMO-2", 1, 10, 8*time.Hour),
	}})

	lines := strings.Split(p.View(50, 8), "\n")
	want := []string{
		"Issue                Mon  Tue  Wed  Thu  Fri Total",
		"DEMO-1               0.3    -    -    -    -   0.3",
		"DEMO-2               7.8  8.0    -    -    -  15.8",
		"Total                8.0  8.0    -    -    -  16.0",
	}
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want label + 4:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, w := range want {
		if got := stripStyle(lines[i+1]); got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got, w)
		}
	}
}

func TestTimesheetNarrowHidesTotal(t *testing.T) {
	p := newTimesheet()
	p, _ = p.Update(weekLoadedMsg{logs: []jira.Worklog{logAt("DEMO-1", 0, 9, time.Hour)}})

	lines := strings.Split(p.View(36, 8), "\n")
	if got := stripStyle(lines[1]); got != "Issue        Mon  Tue  Wed  Thu  Fri" {
		t.Errorf("header = %q", got)
	}
}

func TestTimesheetTooManyRows(t *testing.T) {
	var logs []jira.Worklog
	for i := 1; i <= 8; i++ {
		logs = append(logs, logAt(fmt.Sprintf("DEMO-%d", i), 0, 9, time.Hour))
	}
	p := newTimesheet()
	p, _ = p.Update(weekLoadedMsg{logs: logs})

	view := p.View(50, 8)
	if lines := strings.Split(view, "\n"); len(lines) != 8 {
		t.Errorf("got %d lines, want 8:\n%s", len(lines), view)
	}
	if !strings.Contains(view, "+4 more") || !strings.Contains(view, "8.0") {
		t.Errorf("expected 4 hidden rows and the day total:\n%s", view)
	}
}

// Removes colours and bold so rendered text can be compared
func stripStyle(s string) string {
	return ansi.Strip(s)
}

func TestFetchWeekCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"total":0,"issues":[]}`))
	}))
	defer srv.Close()

	msg := fetchWeek(jira.NewClient(srv.URL, "test-token"), jira.User{Name: "jdoe"}, -1)().(weekLoadedMsg)
	if msg.err != nil || msg.offset != -1 || len(msg.logs) != 0 {
		t.Errorf("unexpected result: %+v", msg)
	}
}

func TestTimesheetRefreshAfterLog(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady

	m = update(t, m, worklogSubmittedMsg{entry: worklog.Entry{IssueKey: "DEMO-1", Duration: time.Hour}})
	if !m.timesheet.loading {
		t.Error("timesheet should reload after a worklog is added")
	}

	m.timesheet.loading = false
	m = update(t, m, worklogSubmittedMsg{err: errors.New("failed")})
	if m.timesheet.loading {
		t.Error("timesheet should not reload after a failed worklog")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"DEMO-1", 10, "DEMO-1"},
		{"DEMO-1", 6, "DEMO-1"},
		{"DEMO-12", 6, "DEMO-…"},
		{"DEMO", 1, "D"},
		{"DEMO", 0, ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.width); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}
