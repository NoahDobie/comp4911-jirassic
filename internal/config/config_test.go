package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HasJira() {
		t.Error("empty config should not have Jira details")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jirassic", "config.json")
	want := Config{
		JiraURL:   "https://jira.example.com",
		JiraToken: "test-token",
		QuickActions: []QuickAction{
			{Label: "Standup", IssueKey: "DEMO-1", Duration: "15m", Comment: "Daily standup"},
		},
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got.HasJira() || got.HasMatcha() {
		t.Error("expected Jira details but no Matcha details")
	}

	got.MatchaURL, got.MatchaKey, got.MatchaMission = "https://matcha.example.com", "test-key", 42
	if !got.HasMatcha() {
		t.Error("expected HasMatcha to be true")
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := map[string]string{
		"bad json":      `{`,
		"missing label": `{"quickActions":[{"issueKey":"DEMO-1","duration":"1h"}]}`,
		"bad issue key": `{"quickActions":[{"label":"x","issueKey":"demo","duration":"1h"}]}`,
		"bad duration":  `{"quickActions":[{"label":"x","issueKey":"DEMO-1","duration":"lots"}]}`,
	}
	for name, body := range tests {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
