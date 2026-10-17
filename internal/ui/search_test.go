package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
)

func testIssues() []jira.Issue {
	return []jira.Issue{
		{Key: "DEMO-1", Summary: "Daily standup", Status: "Open"},
		{Key: "DEMO-12", Summary: "Build the timesheet panel", Status: "In Progress"},
	}
}

func TestSearchRuns(t *testing.T) {
	p := newSearchPanel().setFocused(true)
	p, _ = p.Update(typed("project = DEMO"))
	p, cmd := p.Update(keyEnter)

	if !p.loading || cmd == nil || p.seq != 1 {
		t.Fatalf("expected search to start: loading=%v seq=%d", p.loading, p.seq)
	}

	p, _ = p.Update(searchDoneMsg{seq: 1, issues: testIssues()})
	if p.loading || len(p.results) != 2 {
		t.Errorf("results not shown: loading=%v results=%d", p.loading, len(p.results))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	p := newSearchPanel().setFocused(true)
	p, cmd := p.Update(keyEnter)
	if cmd != nil || p.loading || p.err != "Enter a JQL query" {
		t.Errorf("empty query should not search: err=%q", p.err)
	}
}

func TestSearchIgnoresOldResults(t *testing.T) {
	p := newSearchPanel()
	p.seq, p.loading = 2, true

	p, _ = p.Update(searchDoneMsg{seq: 1, issues: testIssues()})
	if !p.loading || len(p.results) != 0 {
		t.Error("results from an older search should be ignored")
	}
}

func TestSearchError(t *testing.T) {
	p := newSearchPanel()
	p.seq, p.results = 1, testIssues()
	p, _ = p.Update(searchDoneMsg{seq: 1, err: errors.New("jira: Field 'projct' does not exist")})

	if len(p.results) != 0 || !strings.Contains(p.View(40, 9), "projct") {
		t.Errorf("error not shown:\n%s", p.View(40, 9))
	}
}

func TestSearchSelectResult(t *testing.T) {
	p := newSearchPanel().setFocused(true)
	p.seq = 1
	p, _ = p.Update(searchDoneMsg{seq: 1, issues: testIssues()})

	// Down enters the list, down again picks the second issue, down at the end stays put
	p, _ = p.Update(keyDown)
	p, _ = p.Update(keyDown)
	p, _ = p.Update(keyDown)
	if !p.onResults || p.selected != 1 {
		t.Fatalf("onResults=%v selected=%d, want true 1", p.onResults, p.selected)
	}

	_, cmd := p.Update(keyEnter)
	if msg, ok := cmd().(issueSelectedMsg); !ok || msg.key != "DEMO-12" {
		t.Errorf("enter should select DEMO-12, got %+v", msg)
	}

	// Up from the first result goes back to the query
	p, _ = p.Update(keyUp)
	p, _ = p.Update(keyUp)
	if p.onResults {
		t.Error("up from the first result should return to the query")
	}
}

func TestSearchSelectFillsLogPanel(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady
	m = m.setFocus(2)

	m = update(t, m, issueSelectedMsg{key: "DEMO-12"})
	if m.focus != 0 {
		t.Errorf("focus = %d, want the quick log panel", m.focus)
	}
	if m.worklog.inputs[fieldKey].Value() != "DEMO-12" || m.worklog.cursor != fieldDuration {
		t.Errorf("log panel key=%q cursor=%d", m.worklog.inputs[fieldKey].Value(), m.worklog.cursor)
	}
}

func TestSearchView(t *testing.T) {
	p := newSearchPanel()
	p.seq = 1
	p, _ = p.Update(searchDoneMsg{seq: 1, issues: testIssues()})

	// Wide: key and status columns line up
	lines := strings.Split(p.View(60, 9), "\n")
	want := []string{
		"DEMO-1  Open        Daily standup",
		"DEMO-12 In Progress Build the timesheet panel",
	}
	for i, w := range want {
		if got := stripStyle(lines[i+2]); got != w {
			t.Errorf("wide line %d:\n got %q\nwant %q", i, got, w)
		}
	}

	// Narrow: status is hidden and the summary is cut to fit
	lines = strings.Split(p.View(30, 9), "\n")
	if got := stripStyle(lines[3]); got != "DEMO-12 Build the timesheet p…" {
		t.Errorf("narrow line = %q", got)
	}
}

func TestSearchScrolls(t *testing.T) {
	var issues []jira.Issue
	for i := range 20 {
		issues = append(issues, jira.Issue{Key: "DEMO-" + string(rune('A'+i)), Summary: "Task"})
	}
	p := newSearchPanel().setFocused(true)
	p.seq = 1
	p, _ = p.Update(searchDoneMsg{seq: 1, issues: issues})
	p, _ = p.Update(keyDown) // into the list
	for range 10 {
		p, _ = p.Update(keyDown)
	}

	view := p.View(40, 9)
	if lines := strings.Split(view, "\n"); len(lines) != 9 {
		t.Errorf("got %d lines, want 9", len(lines))
	}
	if !strings.Contains(view, "DEMO-K") {
		t.Errorf("selected result should stay visible:\n%s", view)
	}
}

func TestRunSearchCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["jql"] != "project = DEMO" {
			t.Errorf("jql = %v", body["jql"])
		}
		w.Write([]byte(`{"total":1,"issues":[{"key":"DEMO-1","fields":{"summary":"Daily standup","status":{"name":"Open"}}}]}`))
	}))
	defer srv.Close()

	msg := runSearch(jira.NewClient(srv.URL, "test-token"), "project = DEMO", 3)().(searchDoneMsg)
	if msg.err != nil || msg.seq != 3 || len(msg.issues) != 1 || msg.issues[0].Status != "Open" {
		t.Errorf("unexpected result: %+v", msg)
	}
}

func TestKeysReachSearchPanel(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady
	m = m.setFocus(2)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("project = DEMO")})
	if m.search.input.Value() != "project = DEMO" {
		t.Errorf("search input = %q", m.search.input.Value())
	}
}
