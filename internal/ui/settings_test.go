package ui

import (
	"errors"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
)

var testCfg = config.Config{
	JiraURL:      "https://jira.example.com",
	JiraToken:    "test-token",
	QuickActions: []config.QuickAction{{Label: "Standup", IssueKey: "DEMO-1", Duration: "15m"}},
}

// Ready model with testCfg, as if the Jira check already passed
func readyModel(t *testing.T) Model {
	t.Helper()
	m := New(testCfg, filepath.Join(t.TempDir(), "config.json"))
	m.state = stateReady
	return m
}

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestSettingsConfig(t *testing.T) {
	tests := []struct {
		name    string
		values  map[int]string // field index to value
		wantErr bool
	}{
		{"unchanged", nil, false},
		{"bad Jira URL", map[int]string{setJiraURL: "jira"}, true},
		{"empty token", map[int]string{setJiraToken: " "}, true},
		{"all Matcha fields", map[int]string{setMatchaURL: "https://matcha.example.com", setMatchaKey: "k", setMatchaMission: "12"}, false},
		{"half Matcha", map[int]string{setMatchaURL: "https://matcha.example.com"}, true},
		{"mission not a number", map[int]string{setMatchaURL: "https://matcha.example.com", setMatchaKey: "k", setMatchaMission: "abc"}, true},
		{"action without label", map[int]string{connFieldCount: ""}, true},
		{"action with bad time", map[int]string{connFieldCount + 2: "soon"}, true},
	}
	for _, tt := range tests {
		s := newSettings(testCfg)
		for i, v := range tt.values {
			s.inputs[i].SetValue(v)
		}
		_, err := s.config()
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, want error %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestSettingsReadsValues(t *testing.T) {
	cfg := testCfg
	cfg.MatchaURL, cfg.MatchaKey, cfg.MatchaMission = "https://matcha.example.com", "k", 12

	got, err := newSettings(cfg).config()
	if err != nil {
		t.Fatal(err)
	}
	if got.JiraURL != cfg.JiraURL || got.MatchaMission != 12 || len(got.QuickActions) != 1 || got.QuickActions[0] != cfg.QuickActions[0] {
		t.Errorf("got %+v, want %+v", got, cfg)
	}
}

func TestSettingsAddAndDeleteActions(t *testing.T) {
	s := newSettings(testCfg)

	s, _ = s.Update(key(tea.KeyCtrlN))
	if s.actionCount() != 2 || s.cursor != connFieldCount+actionFieldCount {
		t.Fatalf("after add: %d actions, cursor %d", s.actionCount(), s.cursor)
	}
	// A new row left empty is dropped on save
	if cfg, err := s.config(); err != nil || len(cfg.QuickActions) != 1 {
		t.Errorf("empty row: got %v actions, err %v", len(cfg.QuickActions), err)
	}

	// Delete from the comment field of the first row
	s.cursor = connFieldCount + 3
	s, _ = s.Update(key(tea.KeyCtrlD))
	if s.actionCount() != 1 || s.inputs[connFieldCount].Value() != "" {
		t.Errorf("after delete: %d actions, first label %q", s.actionCount(), s.inputs[connFieldCount].Value())
	}

	// Deleting on a connection field does nothing
	s.cursor = setJiraURL
	s, _ = s.Update(key(tea.KeyCtrlD))
	if s.actionCount() != 1 {
		t.Errorf("delete on a Jira field removed a row")
	}
}

func TestSettingsOpenAndClose(t *testing.T) {
	m := readyModel(t)

	m = update(t, m, key(tea.KeyCtrlX))
	if !m.showSettings {
		t.Fatal("ctrl+x should open settings")
	}
	m = update(t, m, key(tea.KeyRunes)) // keys go to settings, not panels
	m = update(t, m, settingsCloseMsg{})
	if m.showSettings {
		t.Error("settings should close")
	}
}

func TestSettingsSaveWithoutJiraChange(t *testing.T) {
	m := readyModel(t)
	m = update(t, m, key(tea.KeyCtrlX))

	cfg := testCfg
	cfg.QuickActions = append(cfg.QuickActions, config.QuickAction{Label: "Review", IssueKey: "DEMO-2", Duration: "30m"})
	m = update(t, m, settingsSaveMsg{cfg: cfg})

	if m.showSettings || m.settings.checking {
		t.Fatal("settings should close without a Jira check")
	}
	if len(m.worklog.actions) != 2 {
		t.Errorf("log panel has %d quick actions, want 2", len(m.worklog.actions))
	}
	if saved, _ := config.Load(m.cfgPath); len(saved.QuickActions) != 2 {
		t.Errorf("saved %d quick actions, want 2", len(saved.QuickActions))
	}
}

func TestSettingsJiraChangeFailsCheck(t *testing.T) {
	m := readyModel(t)
	m = update(t, m, key(tea.KeyCtrlX))

	cfg := testCfg
	cfg.JiraToken = "bad-token"
	m = update(t, m, settingsSaveMsg{cfg: cfg})
	if !m.settings.checking {
		t.Fatal("changed Jira details should be checked first")
	}

	m = update(t, m, verifyMsg{err: errors.New("Authentication failed")})
	if !m.showSettings || m.settings.err != "Authentication failed" || m.state != stateReady {
		t.Errorf("want settings open with error, got open=%v err=%q state=%v", m.showSettings, m.settings.err, m.state)
	}
	if m.cfg.JiraToken != "test-token" {
		t.Error("failed details should not replace the current config")
	}
	if saved, _ := config.Load(m.cfgPath); saved.HasJira() {
		t.Error("failed details should not be saved")
	}
}

func TestSettingsJiraChangePassesCheck(t *testing.T) {
	m := readyModel(t)
	m = update(t, m, key(tea.KeyCtrlX))

	cfg := testCfg
	cfg.JiraToken = "new-token"
	m = update(t, m, settingsSaveMsg{cfg: cfg})
	m = update(t, m, verifyMsg{user: jira.User{DisplayName: "Jane Doe"}})

	if m.showSettings || m.cfg.JiraToken != "new-token" || m.user.DisplayName != "Jane Doe" {
		t.Errorf("want settings closed with new token, got open=%v token=%q", m.showSettings, m.cfg.JiraToken)
	}
	if saved, _ := config.Load(m.cfgPath); saved.JiraToken != "new-token" {
		t.Error("verified details should be saved")
	}
}

func TestSettingsViewFitsScreen(t *testing.T) {
	cfg := testCfg
	for range 20 {
		cfg.QuickActions = append(cfg.QuickActions, testCfg.QuickActions[0])
	}
	for _, size := range [][2]int{{60, 24}, {120, 40}} {
		m := New(cfg, "")
		m.state = stateReady
		m = update(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = update(t, m, key(tea.KeyCtrlX))

		view := m.View()
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
			t.Errorf("%dx%d terminal rendered settings as %dx%d", size[0], size[1], w, h)
		}
	}
}
