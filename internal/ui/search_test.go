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
	"jirassic-capstone/internal/matcha"
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
	if cmd != nil || p.loading || p.err != "Enter JQL or describe the issues you want" {
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

// What Jira returns when the input isn't valid JQL
var badJQL = &jira.StatusError{Code: http.StatusBadRequest, Message: "jira: Error in the JQL Query"}

func TestPlainEnglishFallsBackToMatcha(t *testing.T) {
	p := newSearchPanel().setFocused(true)
	p.matcha = matcha.NewClient("http://127.0.0.1:1", "test-key", 42)

	// Tried as JQL first
	p, _ = p.Update(typed("my open bugs"))
	p, _ = p.Update(keyEnter)
	if !p.loading || p.query != "my open bugs" {
		t.Fatalf("expected a JQL search first: loading=%v query=%q", p.loading, p.query)
	}

	// Jira can't read it, so the same text goes to Matcha
	p, cmd := p.Update(searchDoneMsg{seq: p.seq, err: badJQL})
	if !p.loading || cmd == nil || p.loadText != "Not JQL, asking Matcha..." || p.err != "" {
		t.Fatalf("expected Matcha to be asked: loading=%v text=%q err=%q", p.loading, p.loadText, p.err)
	}

	// Matcha's JQL is put in the input, but nothing is searched yet (NFR-04)
	p, cmd = p.Update(translateDoneMsg{seq: p.seq, jql: "assignee = currentUser() AND type = Bug"})
	if cmd != nil || p.loading || p.searched {
		t.Fatal("generated JQL must not run on its own")
	}
	if !p.generated || p.input.Value() != "assignee = currentUser() AND type = Bug" {
		t.Fatalf("generated=%v input=%q", p.generated, p.input.Value())
	}
	if !strings.Contains(p.View(60, 9), "Check Matcha's JQL, enter to run") {
		t.Errorf("confirm hint not shown:\n%s", p.View(60, 9))
	}

	// The user can edit it, then enter runs it
	p, _ = p.Update(typed(" AND priority = High"))
	p, cmd = p.Update(keyEnter)
	if !p.loading || cmd == nil || p.generated || !p.runGenerated {
		t.Fatal("enter should run the confirmed JQL")
	}

	// If Matcha's JQL is also invalid, show Jira's error instead of asking Matcha again
	p, cmd = p.Update(searchDoneMsg{seq: p.seq, err: badJQL})
	if p.loading || cmd != nil || !strings.Contains(p.err, "Error in the JQL Query") {
		t.Errorf("loading=%v err=%q, want Jira's error", p.loading, p.err)
	}
}

func TestValidJQLDoesNotUseMatcha(t *testing.T) {
	p := newSearchPanel().setFocused(true)
	p.matcha = matcha.NewClient("http://127.0.0.1:1", "test-key", 42)
	p, _ = p.Update(typed("project = DEMO"))
	p, _ = p.Update(keyEnter)

	p, cmd := p.Update(searchDoneMsg{seq: p.seq, issues: testIssues()})
	if cmd != nil || p.loading || len(p.results) != 2 {
		t.Error("valid JQL should show results without asking Matcha")
	}
}

func TestOnlyBadJQLGoesToMatcha(t *testing.T) {
	tests := []struct {
		name   string
		matcha bool
		err    error
	}{
		{"no matcha set up", false, badJQL},
		{"auth error", true, &jira.StatusError{Code: http.StatusUnauthorized, Message: "Authentication failed: check your Jira PAT"}},
		{"connection error", true, errors.New("connect to Jira: timeout")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newSearchPanel().setFocused(true)
			if tt.matcha {
				p.matcha = matcha.NewClient("http://127.0.0.1:1", "test-key", 42)
			}
			p, _ = p.Update(typed("my open bugs"))
			p, _ = p.Update(keyEnter)

			p, cmd := p.Update(searchDoneMsg{seq: p.seq, err: tt.err})
			if p.loading || cmd != nil || p.err != tt.err.Error() {
				t.Errorf("loading=%v err=%q, want the error shown", p.loading, p.err)
			}
		})
	}
}

func TestMatchaError(t *testing.T) {
	p := newSearchPanel()
	p.seq, p.loading = 1, true
	p, _ = p.Update(translateDoneMsg{seq: 1, err: errors.New("Matcha couldn't make JQL: not a Jira search")})

	if p.loading || !strings.Contains(p.err, "not a Jira search") {
		t.Errorf("loading=%v err=%q", p.loading, p.err)
	}
}

func TestRunTranslateCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"jql\":\"project = DEMO\"}"}]}]}`))
	}))
	defer srv.Close()

	msg := runTranslate(matcha.NewClient(srv.URL, "test-key", 42), "everything in DEMO", 5)().(translateDoneMsg)
	if msg.err != nil || msg.seq != 5 || msg.jql != "project = DEMO" {
		t.Errorf("unexpected result: %+v", msg)
	}
}
