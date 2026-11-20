package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

func TestWithin(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 9, 0, 0, 0, time.Local) }
	logs := []jira.Worklog{{IssueKey: "DEMO-1", Started: day(11)}, {IssueKey: "DEMO-2", Started: day(12)}, {IssueKey: "DEMO-3", Started: day(19)}}

	got := within(logs, time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local), time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local))
	if len(got) != 1 || got[0].IssueKey != "DEMO-2" {
		t.Errorf("got %+v, want only DEMO-2", got)
	}
}

// Ready model whose timesheet is on this week
func sharedModel() Model {
	m := New(config.Config{}, "")
	m.state = stateReady
	return m
}

// One fetch on this week's timesheet fills both panels, each with only its own range
func TestSharedFetchSplitsBetweenPanels(t *testing.T) {
	m, _ := sharedModel().reloadWorklogs()
	if !m.timesheet.loading || !m.history.loading {
		t.Fatal("both panels should show they're loading")
	}

	now := time.Now()
	weekStart, _ := worklog.WeekRange(now, 0)
	msg := sharedWorklogsMsg{seq: m.history.seq}
	msg.week[0], msg.week[1] = worklog.WeekRange(now, 0)
	msg.recent[0], msg.recent[1] = worklog.RecentRange(now, historyDays)
	msg.logs = []jira.Worklog{
		{IssueKey: "DEMO-1", Started: weekStart.Add(time.Hour), Duration: time.Hour},           // this week
		{IssueKey: "DEMO-2", Started: weekStart.AddDate(0, 0, -5), Duration: 30 * time.Minute}, // only in history
	}

	m = update(t, m, msg)
	if m.timesheet.loading || m.history.loading {
		t.Fatal("both panels should finish loading")
	}
	if len(m.timesheet.rows) != 1 || m.timesheet.rows[0].key != "DEMO-1" {
		t.Errorf("timesheet rows = %+v, want only DEMO-1", m.timesheet.rows)
	}
	if len(m.history.logs) != 2 {
		t.Errorf("history has %d logs, want 2", len(m.history.logs))
	}
}

// A shared result arriving after the user moved to another week only updates history
func TestSharedFetchIgnoredByMovedTimesheet(t *testing.T) {
	m, _ := sharedModel().reloadWorklogs()
	m.timesheet.offset = -1

	msg := sharedWorklogsMsg{seq: m.history.seq}
	msg.recent[0], msg.recent[1] = worklog.RecentRange(time.Now(), historyDays)
	msg.logs = []jira.Worklog{{IssueKey: "DEMO-1", Started: time.Now().Add(-time.Hour), Duration: time.Hour}}

	m = update(t, m, msg)
	if !m.timesheet.loading || len(m.timesheet.rows) != 0 {
		t.Error("timesheet on another week should ignore the shared result")
	}
	if m.history.loading || len(m.history.logs) != 1 {
		t.Error("history should still take its part")
	}
}

// Another week on the timesheet means separate fetches, since the ranges don't overlap
func TestReloadOnOtherWeekLoadsSeparately(t *testing.T) {
	m := sharedModel()
	m.timesheet.offset = -3
	seq := m.history.seq

	m, cmd := m.reloadWorklogs()
	if cmd == nil || !m.timesheet.loading || !m.history.loading || m.history.seq != seq+1 {
		t.Error("both panels should start their own loads")
	}
}

// The shared fetch asks Jira once for a range covering both panels
func TestFetchSharedCoversBothRanges(t *testing.T) {
	var jqls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ JQL string }
		json.NewDecoder(r.Body).Decode(&body)
		jqls = append(jqls, body.JQL)
		w.Write([]byte(`{"total":0,"issues":[]}`))
	}))
	defer srv.Close()

	msg := sharedWorklogsMsg{}
	msg.week[0] = time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)
	msg.week[1] = time.Date(2026, 10, 26, 0, 0, 0, 0, time.Local)
	msg.recent[0] = time.Date(2026, 10, 9, 0, 0, 0, 0, time.Local)
	msg.recent[1] = time.Date(2026, 10, 24, 0, 0, 0, 0, time.Local)

	got := fetchShared(jira.NewClient(srv.URL, "test-token"), jira.User{}, msg)().(sharedWorklogsMsg)
	if got.err != nil || len(jqls) != 1 {
		t.Fatalf("want one search, got %d, err %v", len(jqls), got.err)
	}
	// From the earlier start (Oct 9) to the later end (Oct 26), each widened by a day
	if !strings.Contains(jqls[0], `worklogDate >= "2026-10-08"`) || !strings.Contains(jqls[0], `worklogDate <= "2026-10-27"`) {
		t.Errorf("jql = %s", jqls[0])
	}
}
