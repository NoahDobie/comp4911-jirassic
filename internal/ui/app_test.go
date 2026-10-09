package ui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
)

// Sends msg to the model and returns the updated Model
func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestGridSizes(t *testing.T) {
	tests := []struct {
		w, h                         int
		leftW, rightW, topH, bottomH int
	}{
		{100, 41, 50, 50, 20, 20},
		{101, 40, 50, 51, 19, 20},
	}
	for _, tt := range tests {
		l, r, top, bottom := gridSizes(tt.w, tt.h)
		if l != tt.leftW || r != tt.rightW || top != tt.topH || bottom != tt.bottomH {
			t.Errorf("gridSizes(%d, %d) = %d %d %d %d", tt.w, tt.h, l, r, top, bottom)
		}
		if l+r != tt.w || top+bottom != tt.h-1 {
			t.Errorf("gridSizes(%d, %d) does not fill the screen", tt.w, tt.h)
		}
	}
}

// Tests that the initial state of the UI is correct based on the config provided
// If the config is empty, the UI should start in setup state, otherwise it should start in verifying state
func TestStartState(t *testing.T) {
	if m := New(config.Config{}, ""); m.state != stateSetup {
		t.Error("empty config should start in setup")
	}
	cfg := config.Config{JiraURL: "https://jira.example.com", JiraToken: "test-token"}
	if m := New(cfg, ""); m.state != stateVerifying {
		t.Error("saved config should start verifying")
	}
}

// Tests that the focus of the UI changes correctly when the user presses tab, shift+tab, or alt+1-4
func TestFocusKeys(t *testing.T) {
	m := New(config.Config{}, "")
	m.state = stateReady

	steps := []struct {
		key  tea.KeyMsg
		want int
	}{
		{tea.KeyMsg{Type: tea.KeyTab}, 1},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, 0},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, 3},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true}, 1},
	}
	for _, s := range steps {
		m = update(t, m, s.key)
		if m.focus != s.want {
			t.Errorf("after %q focus = %d, want %d", s.key.String(), m.focus, s.want)
		}
	}
}

// Tests that a successful verification saves the config to the specified path
func TestVerifySuccessSavesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(config.Config{}, path)

	m = update(t, m, setupSubmitMsg{url: "https://jira.example.com", token: "test-token"})
	if m.state != stateVerifying {
		t.Fatalf("state = %v, want verifying", m.state)
	}

	m = update(t, m, verifyMsg{user: jira.User{DisplayName: "Jane Doe"}})
	if m.state != stateReady || m.user.DisplayName != "Jane Doe" {
		t.Fatalf("expected ready state with user, got %v %+v", m.state, m.user)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.JiraURL != "https://jira.example.com" || saved.JiraToken != "test-token" {
		t.Errorf("config not saved: %+v", saved)
	}
}

func TestVerifyFailureReturnsToSetup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(config.Config{}, path)

	m = update(t, m, setupSubmitMsg{url: "https://jira.example.com", token: "bad-token"})
	m = update(t, m, verifyMsg{err: errors.New("Authentication failed")})

	if m.state != stateSetup || m.setup.err != "Authentication failed" {
		t.Errorf("expected setup with error, got %v %q", m.state, m.setup.err)
	}
	if saved, _ := config.Load(path); saved.HasJira() {
		t.Error("unverified details should not be saved")
	}
}

func TestVerifyCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"jdoe","displayName":"Jane Doe"}`))
	}))
	defer srv.Close()

	msg := verify(srv.URL, "test-token")().(verifyMsg)
	if msg.err != nil || msg.user.DisplayName != "Jane Doe" || msg.client == nil {
		t.Errorf("unexpected result: %+v", msg)
	}
}

// Ensures that the validateSetup function correctly validates the Jira URL and PAT
func TestValidateSetup(t *testing.T) {
	tests := []struct {
		url, token string
		ok         bool
	}{
		{"https://jira.example.com", "test-token", true},
		{"http://localhost:8080", "test-token", true},
		{"", "test-token", false},
		{"jira.example.com", "test-token", false},
		{"https://jira.example.com", "", false},
	}
	for _, tt := range tests {
		err := validateSetup(tt.url, tt.token)
		if (err == nil) != tt.ok {
			t.Errorf("validateSetup(%q, %q) error = %v", tt.url, tt.token, err)
		}
	}
}

// Tests that the UI renders correctly for different terminal sizes
func TestViewFitsScreen(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {101, 40}, {200, 60}} {
		m := New(config.Config{}, "")
		m.state = stateReady
		m = update(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})

		view := m.View()
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
			t.Errorf("%dx%d terminal rendered as %dx%d", size[0], size[1], w, h)
		}
	}
}
