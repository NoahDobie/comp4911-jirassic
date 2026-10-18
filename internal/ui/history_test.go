package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

// Worklogs oldest first, as GetWorklogs returns them
func testHistory() []jira.Worklog {
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 30, 0, 0, time.Local) }
	return []jira.Worklog{
		{IssueKey: "DEMO-1", IssueSummary: "Daily standup", Started: at(14, 9), Duration: 15 * time.Minute},
		{IssueKey: "DEMO-12", IssueSummary: "Build the timesheet panel", Started: at(14, 10), Duration: 7*time.Hour + 30*time.Minute},
		{IssueKey: "DEMO-30", IssueSummary: "Fix week rollover bug", Started: at(15, 13), Duration: 3 * time.Hour},
	}
}

// History panel that has finished loading logs
func loadedHistory(logs []jira.Worklog) historyPanel {
	p := newHistoryPanel().setFocused(true)
	p.seq = 1
	p, _ = p.Update(historyLoadedMsg{seq: 1, logs: logs})
	return p
}

func TestHistoryNewestFirst(t *testing.T) {
	p := loadedHistory(testHistory())
	if len(p.logs) != 3 || p.logs[0].IssueKey != "DEMO-30" || p.logs[2].IssueKey != "DEMO-1" {
		t.Errorf("logs not newest first: %+v", p.logs)
	}
}

func TestHistoryIgnoresOldLoad(t *testing.T) {
	p := newHistoryPanel()
	p.seq, p.loading = 2, true
	p, _ = p.Update(historyLoadedMsg{seq: 1, logs: testHistory()})
	if !p.loading || len(p.logs) != 0 {
		t.Error("results from an older load should be ignored")
	}
}

func TestHistoryError(t *testing.T) {
	p := newHistoryPanel()
	p.seq, p.loading = 1, true
	p, _ = p.Update(historyLoadedMsg{seq: 1, err: errors.New("Authentication failed: check your Jira PAT")})
	if p.loading || !strings.Contains(p.View(40, 8), "Authentication failed") {
		t.Errorf("error not shown:\n%s", p.View(40, 8))
	}
}

func TestHistoryKeys(t *testing.T) {
	p := loadedHistory(testHistory())

	steps := []struct {
		name string
		key  any
		want int
	}{
		{"up at top stays", keyUp, 0},
		{"down", keyDown, 1},
		{"down", keyDown, 2},
		{"down at bottom stays", keyDown, 2},
		{"up", keyUp, 1},
	}
	for _, s := range steps {
		p, _ = p.Update(s.key)
		if p.selected != s.want {
			t.Errorf("%s: selected = %d, want %d", s.name, p.selected, s.want)
		}
	}

	p, cmd := p.Update(typed("r"))
	if !p.loading || cmd == nil || p.seq != 2 {
		t.Error("r should reload")
	}
}

func TestHistoryView(t *testing.T) {
	p := loadedHistory(testHistory())

	lines := strings.Split(p.View(60, 8), "\n")
	if got := stripStyle(lines[0]); !strings.HasPrefix(got, "Last 14 days") || !strings.HasSuffix(got, "3 logs · 10.8h") {
		t.Errorf("header = %q", got)
	}
	want := []string{
		"Oct 15 13:30  DEMO-30  3.0h  Fix week rollover bug",
		"Oct 14 10:30  DEMO-12  7.5h  Build the timesheet panel",
		"Oct 14 09:30  DEMO-1   0.3h  Daily standup",
	}
	for i, w := range want {
		if got := stripStyle(lines[i+1]); got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got, w)
		}
	}

	// Narrow panels drop the time and cut the title to fit
	lines = strings.Split(p.View(36, 8), "\n")
	if got := stripStyle(lines[2]); got != "Oct 14  DEMO-12  7.5h  Build the ti…" {
		t.Errorf("narrow line = %q", got)
	}
}

func TestHistoryScrolls(t *testing.T) {
	var logs []jira.Worklog
	for i := range 20 {
		logs = append(logs, jira.Worklog{IssueKey: fmt.Sprintf("DEMO-%d", i), Started: time.Now(), Duration: time.Hour})
	}
	p := loadedHistory(logs)
	for range 15 {
		p, _ = p.Update(keyDown)
	}

	view := p.View(40, 8)
	if lines := strings.Split(view, "\n"); len(lines) != 8 {
		t.Errorf("got %d lines, want 8", len(lines))
	}
	if !strings.Contains(view, p.logs[15].IssueKey+" ") {
		t.Errorf("selected worklog should stay visible:\n%s", view)
	}
}

func TestFetchHistoryRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// GetWorklogs widens the search by a day on each side
		now := time.Now()
		from := now.AddDate(0, 0, -historyDays-1).Format("2006-01-02")
		to := now.AddDate(0, 0, 2).Format("2006-01-02")
		jql, _ := body["jql"].(string)
		if !strings.Contains(jql, from) || !strings.Contains(jql, to) {
			t.Errorf("jql = %q, want dates %s to %s", jql, from, to)
		}
		w.Write([]byte(`{"total":0,"issues":[]}`))
	}))
	defer srv.Close()

	msg := fetchHistory(jira.NewClient(srv.URL, "test-token"), jira.User{Name: "jdoe"}, 4)().(historyLoadedMsg)
	if msg.err != nil || msg.seq != 4 {
		t.Errorf("unexpected result: %+v", msg)
	}
}

func TestHistoryRefreshAfterLog(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady

	m = update(t, m, worklogSubmittedMsg{entry: worklog.Entry{IssueKey: "DEMO-1", Duration: time.Hour}})
	if !m.history.loading {
		t.Error("history should reload after a worklog is added")
	}
}
