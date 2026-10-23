package ui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jirassic-capstone/internal/config"
)

// Connection fields at the top of the settings form, in order
const (
	setJiraURL = iota
	setJiraToken
	setMatchaURL
	setMatchaKey
	setMatchaMission
	connFieldCount
)

// Each quick action row has a label, issue, time, and comment
const actionFieldCount = 4

const (
	settingsMaxWidth = 92 // room for a 72 character Matcha key
	settingsLabelW   = 11
	settingsChrome   = 19 // lines in the pop-up besides the quick action rows
)

var (
	connLabels    = [connFieldCount]string{"URL", "Token", "URL", "API key", "Mission"}
	actionColumns = [actionFieldCount]string{"Label", "Issue", "Time", "Comment"}
	actionWidths  = [actionFieldCount - 1]int{16, 15, 9} // comment gets the rest
)

// Sent when the user saves settings that passed validation
type settingsSaveMsg struct {
	cfg config.Config
}

// Sent when the user closes settings without saving
type settingsCloseMsg struct{}

// Pop-up form for editing the config: Jira and Matcha details, then quick actions
type settingsModel struct {
	inputs   []textinput.Model // connection fields, then actionFieldCount per quick action
	cursor   int
	checking bool          // true while new Jira details are being verified
	pending  config.Config // settings waiting on that check
	spinner  spinner.Model
	err      string
}

func newSettings(cfg config.Config) settingsModel {
	s := settingsModel{spinner: spinner.New()}
	values := [connFieldCount]string{cfg.JiraURL, cfg.JiraToken, cfg.MatchaURL, cfg.MatchaKey, ""}
	if cfg.MatchaMission > 0 {
		values[setMatchaMission] = strconv.Itoa(cfg.MatchaMission)
	}
	placeholders := [connFieldCount]string{"https://jira.example.com", "Personal access token", "optional", "optional", "optional, ex. 12"}
	for i := range values {
		in := newInput(placeholders[i], values[i])
		if i == setJiraToken || i == setMatchaKey {
			in.EchoMode = textinput.EchoPassword
		}
		s.inputs = append(s.inputs, in)
	}
	for _, a := range cfg.QuickActions {
		s = s.addAction(a)
	}
	return s.focusCursor()
}

func newInput(placeholder, value string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.SetValue(value)
	return in
}

// Adds a row of fields for a quick action at the end of the form
func (s settingsModel) addAction(a config.QuickAction) settingsModel {
	placeholders := [actionFieldCount]string{"Standup", "DEMO-123", "15m", "optional"}
	values := [actionFieldCount]string{a.Label, a.IssueKey, a.Duration, a.Comment}
	for i := range values {
		s.inputs = append(s.inputs, newInput(placeholders[i], values[i]))
	}
	return s
}

// Removes the quick action row the cursor is on
func (s settingsModel) deleteAction() settingsModel {
	if s.cursor < connFieldCount {
		return s
	}
	start := s.cursor - (s.cursor-connFieldCount)%actionFieldCount
	s.inputs = slices.Delete(s.inputs, start, start+actionFieldCount)
	s.cursor = min(start, len(s.inputs)-1)
	return s
}

func (s settingsModel) actionCount() int {
	return (len(s.inputs) - connFieldCount) / actionFieldCount
}

// Puts the text cursor on the selected field only
func (s settingsModel) focusCursor() settingsModel {
	for i := range s.inputs {
		if i == s.cursor {
			s.inputs[i].Focus()
		} else {
			s.inputs[i].Blur()
		}
	}
	return s
}

func (s settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !s.checking {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	case tea.KeyMsg:
		// Ignore input while the Jira check runs
		if s.checking {
			return s, nil
		}
		return s.handleKey(msg)
	}

	var cmd tea.Cmd
	s.inputs[s.cursor], cmd = s.inputs[s.cursor].Update(msg)
	return s, cmd
}

func (s settingsModel) handleKey(key tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch key.String() {
	case "esc":
		return s, func() tea.Msg { return settingsCloseMsg{} }
	case "tab", "down":
		s.cursor = (s.cursor + 1) % len(s.inputs)
		return s.focusCursor(), textinput.Blink
	case "shift+tab", "up":
		s.cursor = (s.cursor - 1 + len(s.inputs)) % len(s.inputs)
		return s.focusCursor(), textinput.Blink
	case "ctrl+n":
		s = s.addAction(config.QuickAction{})
		s.cursor = len(s.inputs) - actionFieldCount
		return s.focusCursor(), textinput.Blink
	case "ctrl+d":
		return s.deleteAction().focusCursor(), textinput.Blink
	case "enter":
		cfg, err := s.config()
		if err != nil {
			s.err = err.Error()
			return s, nil
		}
		s.err = ""
		return s, func() tea.Msg { return settingsSaveMsg{cfg: cfg} }
	}

	var cmd tea.Cmd
	s.inputs[s.cursor], cmd = s.inputs[s.cursor].Update(key)
	return s, cmd
}

// Reads the form into a config, checking each value
func (s settingsModel) config() (config.Config, error) {
	val := func(i int) string { return strings.TrimSpace(s.inputs[i].Value()) }
	cfg := config.Config{
		JiraURL:   val(setJiraURL),
		JiraToken: val(setJiraToken),
		MatchaURL: val(setMatchaURL),
		MatchaKey: val(setMatchaKey),
	}
	if err := validateSetup(cfg.JiraURL, cfg.JiraToken); err != nil {
		return cfg, err
	}

	mission := val(setMatchaMission)
	if mission != "" {
		n, err := strconv.Atoi(mission)
		if err != nil || n <= 0 {
			return cfg, errors.New("Matcha mission must be a positive number")
		}
		cfg.MatchaMission = n
	}
	// Matcha is optional, but it only works with all three filled in
	if (cfg.MatchaURL != "" || cfg.MatchaKey != "" || mission != "") && !cfg.HasMatcha() {
		return cfg, errors.New("Fill in all three Matcha fields, or leave them all empty")
	}

	for i := range s.actionCount() {
		first := connFieldCount + i*actionFieldCount
		a := config.QuickAction{Label: val(first), IssueKey: val(first + 1), Duration: val(first + 2), Comment: val(first + 3)}
		// Skip rows left completely empty
		if a == (config.QuickAction{}) {
			continue
		}
		cfg.QuickActions = append(cfg.QuickActions, a)
	}
	return cfg, cfg.ValidateQuickActions()
}

// Pop-up box centered in a width x height screen
func (s settingsModel) View(width, height int) string {
	boxW := min(width, settingsMaxWidth)
	w := boxW - 4 // border and padding

	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings") + "\n\n")
	b.WriteString(titleStyle.Render("Jira") + "\n")
	b.WriteString(s.fieldView(setJiraURL, w) + "\n" + s.fieldView(setJiraToken, w) + "\n\n")
	b.WriteString(titleStyle.Render("Matcha") + helpStyle.Render(" (optional, for plain English search)") + "\n")
	for i := setMatchaURL; i <= setMatchaMission; i++ {
		b.WriteString(s.fieldView(i, w) + "\n")
	}
	b.WriteString("\n" + titleStyle.Render(fmt.Sprintf("Quick actions (%d)", s.actionCount())) + "\n")
	b.WriteString(s.actionsView(w, height-settingsChrome) + "\n\n")
	b.WriteString(s.statusView(w))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(focusColor).
		Padding(0, 1).
		Width(boxW - 2).
		Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// One labelled connection field
func (s settingsModel) fieldView(i, width int) string {
	in := s.inputs[i]
	in.Width = max(width-settingsLabelW-len([]rune(in.Prompt))-1, 1) // 1 for the cursor
	return fmt.Sprintf("%-*s%s", settingsLabelW, "  "+connLabels[i], in.View())
}

// Quick actions as a table, scrolled so the selected row stays visible
func (s settingsModel) actionsView(width, rows int) string {
	n := s.actionCount()
	if n == 0 {
		return helpStyle.Render("None yet, ctrl+n to add one")
	}

	widths := [actionFieldCount]int{actionWidths[0], actionWidths[1], actionWidths[2], 0}
	widths[actionFieldCount-1] = max(width-actionWidths[0]-actionWidths[1]-actionWidths[2], 4)

	header := ""
	for j, col := range actionColumns {
		header += fmt.Sprintf("  %-*s", widths[j]-2, col) // lines up after the "> " prompt
	}
	lines := []string{helpStyle.Render(header)}

	selected := max(s.cursor-connFieldCount, 0) / actionFieldCount
	start, end := visibleRange(selected, max(rows, 1), n)
	for i := start; i < end; i++ {
		row := ""
		for j := range actionFieldCount {
			in := s.inputs[connFieldCount+i*actionFieldCount+j]
			in.Width = max(widths[j]-len([]rune(in.Prompt))-2, 1) // cursor and a space between columns
			row += lipgloss.NewStyle().Width(widths[j]).Render(in.View())
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// Check progress or error, then key help
func (s settingsModel) statusView(width int) string {
	status := ""
	switch {
	case s.checking:
		status = s.spinner.View() + " Checking the new Jira details..."
	case s.err != "":
		status = errorStyle.Render(truncate(s.err, width))
	}
	help := helpStyle.Render(truncate("enter: save • esc: close without saving", width)) + "\n" +
		helpStyle.Render(truncate("↑/↓/tab: move • ctrl+n: add action • ctrl+d: delete action", width))
	return status + "\n" + help
}
