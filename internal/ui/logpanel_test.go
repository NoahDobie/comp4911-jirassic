package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keyUp    = tea.KeyMsg{Type: tea.KeyUp}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
)

// Key message for typing text into a field
func typed(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// Sends each message to the panel in order and returns the last command
func send(p logPanel, msgs ...tea.Msg) (logPanel, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		p, cmd = p.Update(msg)
	}
	return p, cmd
}

func testActions() []config.QuickAction {
	return []config.QuickAction{
		{Label: "Standup", IssueKey: "DEMO-1", Duration: "15m", Comment: "Daily standup"},
		{Label: "Review", IssueKey: "DEMO-2", Duration: "1h"},
	}
}

// Tests that the log panel submits a worklog entry correctly and handles the response.
func TestLogPanelSubmitForm(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, cmd := send(p, typed("demo-5"), keyEnter, typed("1h 30m"), keyEnter, keyEnter, typed("Code review"), keyEnter)

	if !p.submitting || cmd == nil {
		t.Fatal("expected panel to be submitting")
	}
	if p.status != "Logging 1h 30m to DEMO-5..." {
		t.Errorf("status = %q", p.status)
	}

	// Keys are ignored while posting
	p, _ = send(p, typed("x"))
	if p.inputs[fieldComment].Value() != "Code review" {
		t.Error("input changed while submitting")
	}

	entry := worklog.Entry{IssueKey: "DEMO-5", Duration: 90 * time.Minute}
	p, _ = send(p, worklogSubmittedMsg{entry: entry})
	if p.submitting || p.status != "Logged 1h 30m to DEMO-5" {
		t.Errorf("unexpected state after success: submitting=%v status=%q", p.submitting, p.status)
	}
	if p.inputs[fieldKey].Value() != "" || p.cursor != fieldKey {
		t.Error("form should be cleared and ready for the next entry")
	}
}

func TestLogPanelInvalidInput(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, cmd := send(p, typed("DEMO-5"), keyEnter, typed("soon"), keyEnter, keyEnter, keyEnter)

	if p.submitting || cmd != nil {
		t.Error("invalid input should not submit")
	}
	if !strings.Contains(p.err, "invalid duration") {
		t.Errorf("err = %q", p.err)
	}
}

func TestLogPanelSubmitError(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, _ = send(p, typed("DEMO-5"), keyEnter, typed("1h"), keyEnter, keyEnter, keyEnter)
	p, _ = send(p, worklogSubmittedMsg{err: errors.New("Issue does not exist")})

	if p.submitting || p.err != "Issue does not exist" {
		t.Errorf("unexpected state: submitting=%v err=%q", p.submitting, p.err)
	}
	if p.inputs[fieldKey].Value() != "DEMO-5" {
		t.Error("form should be kept so the user can fix it")
	}
}

// Tests that a quick action fills the form instead of submitting, so it can be checked first
func TestLogPanelQuickAction(t *testing.T) {
	p := newLogPanel(nil, testActions()).setFocused(true)

	// Type into the form, go up to the Standup button, and pick it - it replaces what was typed
	p, _ = send(p, typed("DEMO-9"), keyEnter, typed("2h"), keyUp, keyUp, keyEnter)
	if p.submitting {
		t.Fatal("a quick action should not submit by itself")
	}
	if p.inputs[fieldKey].Value() != "DEMO-1" || p.inputs[fieldDuration].Value() != "15m" || p.inputs[fieldComment].Value() != "Daily standup" {
		t.Errorf("form not filled: %q %q %q", p.inputs[fieldKey].Value(), p.inputs[fieldDuration].Value(), p.inputs[fieldComment].Value())
	}
	if p.cursor != fieldStarted {
		t.Errorf("cursor = %d, want the start time field", p.cursor)
	}

	// Enter past the comment submits as normal
	p, cmd := send(p, keyEnter, keyEnter)
	if !p.submitting || cmd == nil || p.status != "Logging 15m to DEMO-1..." {
		t.Errorf("expected submit, submitting=%v status=%q err=%q", p.submitting, p.status, p.err)
	}
}

func TestLogPanelQuickActionButton(t *testing.T) {
	p := newLogPanel(nil, testActions()).setFocused(true)

	// Up from the first field selects the buttons, right moves past the last one
	p, _ = send(p, keyUp, keyRight, keyRight)
	if !p.onButtons || p.button != 1 {
		t.Fatalf("onButtons=%v button=%d, want true 1", p.onButtons, p.button)
	}
	p, cmd := send(p, keyEnter)

	if p.submitting || p.onButtons || p.inputs[fieldKey].Value() != "DEMO-2" || p.inputs[fieldDuration].Value() != "1h" {
		t.Errorf("enter on Review should fill the form, got submitting=%v onButtons=%v key=%q", p.submitting, p.onButtons, p.inputs[fieldKey].Value())
	}
	if cmd == nil {
		t.Error("expected the cursor blink to start")
	}
}

func TestLogPanelTabOrder(t *testing.T) {
	keyTab := tea.KeyMsg{Type: tea.KeyTab}

	// Issue -> Time -> Started -> Comment -> Standup -> Review -> Issue
	p := newLogPanel(nil, testActions()).setFocused(true)
	want := []struct {
		onButtons     bool
		button, field int
	}{
		{false, 0, fieldDuration},
		{false, 0, fieldStarted},
		{false, 0, fieldComment},
		{true, 0, fieldComment},
		{true, 1, fieldComment},
		{false, 1, fieldKey},
	}
	for i, w := range want {
		p, _ = send(p, keyTab)
		if p.onButtons != w.onButtons || (w.onButtons && p.button != w.button) || (!w.onButtons && p.cursor != w.field) {
			t.Errorf("tab %d: onButtons=%v button=%d cursor=%d", i+1, p.onButtons, p.button, p.cursor)
		}
	}

	// Without buttons, tab wraps from Comment to Issue
	p = newLogPanel(nil, nil).setFocused(true)
	p, _ = send(p, keyTab, keyTab, keyTab, keyTab)
	if p.onButtons || p.cursor != fieldKey {
		t.Errorf("onButtons=%v cursor=%d, want field %d", p.onButtons, p.cursor, fieldKey)
	}
}

func TestLogPanelNoButtons(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, _ = send(p, keyUp)
	if p.onButtons {
		t.Error("buttons should not be selectable when there are no quick actions")
	}
}

func TestLogPanelClear(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, _ = send(p, typed("DEMO-5"), keyEnter, typed("1h"), keyEsc)

	if p.inputs[fieldKey].Value() != "" || p.inputs[fieldDuration].Value() != "" || p.cursor != fieldKey {
		t.Error("esc should clear the form")
	}
}

func TestAddWorklogCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/rest/api/2/issue/DEMO-1/worklog" || body["timeSpent"] != "15m" {
			t.Errorf("unexpected request %s %v", r.URL.Path, body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"10001"}`))
	}))
	defer srv.Close()

	entry := worklog.Entry{IssueKey: "DEMO-1", Duration: 15 * time.Minute, Started: time.Now()}
	msg := addWorklog(jira.NewClient(srv.URL, "test-token"), entry)().(worklogSubmittedMsg)
	if msg.err != nil {
		t.Errorf("unexpected error: %v", msg.err)
	}
}

// Tests that key presses only reach the focused panel
func TestKeysOnlyReachFocusedPanel(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady
	m = m.setFocus(1)

	m = update(t, m, typed("DEMO-1"))
	if m.worklog.inputs[fieldKey].Value() != "" {
		t.Error("typing in another panel should not reach the log panel")
	}

	m = m.setFocus(0)
	m = update(t, m, typed("DEMO-1"))
	if m.worklog.inputs[fieldKey].Value() != "DEMO-1" {
		t.Error("typing should reach the focused log panel")
	}
}

func TestLogPanelStartTime(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body["started"], "T09:30:00.000") {
			t.Errorf("started = %q, want 09:30 today", body["started"])
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"10001"}`))
	}))
	defer srv.Close()

	p := newLogPanel(jira.NewClient(srv.URL, "test-token"), nil).setFocused(true)
	p, cmd := send(p, typed("DEMO-5"), keyEnter, typed("1h"), keyEnter, typed("9:30am"), keyEnter, keyEnter)
	if !p.submitting || cmd == nil {
		t.Fatalf("expected submit, err = %q", p.err)
	}

	// The batch holds the spinner tick and the post, run each one
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(worklogSubmittedMsg); ok && msg.err != nil {
			t.Errorf("unexpected error: %v", msg.err)
		}
	}
	if !called {
		t.Error("worklog was not posted")
	}
}

func TestLogPanelInvalidStartTime(t *testing.T) {
	p := newLogPanel(nil, nil).setFocused(true)
	p, cmd := send(p, typed("DEMO-5"), keyEnter, typed("1h"), keyEnter, typed("later"), keyEnter, keyEnter)

	if p.submitting || cmd != nil {
		t.Error("invalid start time should not submit")
	}
	if !strings.Contains(p.err, "invalid start") {
		t.Errorf("err = %q", p.err)
	}
}
