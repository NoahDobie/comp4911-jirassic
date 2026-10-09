package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jirassic-capstone/internal/worklog"
)

// QuickAction is a saved worklog the user can submit in one click
type QuickAction struct {
	Label    string `json:"label"`
	IssueKey string `json:"issueKey"`
	Duration string `json:"duration"`
	Comment  string `json:"comment,omitempty"`
}

// Config is stored locally only
type Config struct {
	JiraURL       string        `json:"jiraUrl"`
	JiraToken     string        `json:"jiraToken"`
	MatchaURL     string        `json:"matchaUrl,omitempty"`
	MatchaKey     string        `json:"matchaKey,omitempty"`
	MatchaMission string        `json:"matchaMission,omitempty"`
	QuickActions  []QuickAction `json:"quickActions"`
}

// Path returns the default config file location
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config dir: %w", err)
	}
	return filepath.Join(dir, "jirassic", "config.json"), nil
}

// Load reads the config at path
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.validateQuickActions(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes the config with owner-only permissions
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Rreports whether Jira connection details are filled in
func (c Config) HasJira() bool {
	return strings.TrimSpace(c.JiraURL) != "" && strings.TrimSpace(c.JiraToken) != ""
}

func (c Config) validateQuickActions() error {
	for i, a := range c.QuickActions {
		if strings.TrimSpace(a.Label) == "" {
			return fmt.Errorf("quick action %d: label is required", i+1)
		}
		if _, err := worklog.NewEntry(a.IssueKey, a.Duration, a.Comment, time.Time{}); err != nil {
			return fmt.Errorf("quick action %q: %w", a.Label, err)
		}
	}
	return nil
}
