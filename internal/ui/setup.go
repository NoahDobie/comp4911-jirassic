package ui

import (
	"errors"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Sent when the user submits the setup form
type setupSubmitMsg struct {
	url, token string
}

// Screen that collects the Jira URL and PAT
type setupModel struct {
	inputs [2]textinput.Model
	focus  int
	err    string
}

// Setup dialog with placeholder values for user to fill Jira information
func newSetup(jiraURL, token string) setupModel {
	u := textinput.New()
	u.Placeholder = "https://jira.example.com"
	u.SetValue(jiraURL)
	u.Focus()

	t := textinput.New()
	t.Placeholder = "Personal access token"
	t.EchoMode = textinput.EchoPassword
	t.SetValue(token)

	return setupModel{inputs: [2]textinput.Model{u, t}}
}

func (s setupModel) Update(msg tea.Msg) (setupModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", "shift+tab", "up", "down":
			return s.switchField(), textinput.Blink
		case "enter":
			if s.focus == 0 {
				return s.switchField(), textinput.Blink
			}
			return s.submit()
		}
	}

	var cmd tea.Cmd
	s.inputs[s.focus], cmd = s.inputs[s.focus].Update(msg)
	return s, cmd
}

func (s setupModel) switchField() setupModel {
	s.inputs[s.focus].Blur()
	s.focus = 1 - s.focus
	s.inputs[s.focus].Focus()
	return s
}

func (s setupModel) submit() (setupModel, tea.Cmd) {
	jiraURL := strings.TrimSpace(s.inputs[0].Value())
	token := strings.TrimSpace(s.inputs[1].Value())
	if err := validateSetup(jiraURL, token); err != nil {
		s.err = err.Error()
		return s, nil
	}
	s.err = ""
	return s, func() tea.Msg { return setupSubmitMsg{url: jiraURL, token: token} }
}

// Checks the Jira URL and PAT are filled in properly
func validateSetup(jiraURL, token string) error {
	u, err := url.Parse(jiraURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("Enter a full Jira URL, e.g. https://jira.example.com")
	}
	if token == "" {
		return errors.New("Enter your Jira personal access token")
	}
	return nil
}

// Setup form with any error and key help
func (s setupModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Jirassic setup") + "\n\n")
	b.WriteString("Jira URL\n" + s.inputs[0].View() + "\n\n")
	b.WriteString("Personal access token\n" + s.inputs[1].View() + "\n\n")
	if s.err != "" {
		b.WriteString(errorStyle.Render(s.err) + "\n\n")
	}
	b.WriteString(helpStyle.Render("enter: next/connect • tab: switch field • ctrl+c: quit"))
	return b.String()
}
