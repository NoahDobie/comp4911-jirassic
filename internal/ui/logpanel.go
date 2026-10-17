package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

// Form fields, in order
const (
	fieldKey = iota
	fieldDuration
	fieldStarted
	fieldComment
	fieldCount
)

const labelWidth = 8

var fieldLabels = [fieldCount]string{"Issue", "Time", "Started", "Comment"}

// Sent when a worklog finishes posting to Jira
type worklogSubmittedMsg struct {
	entry worklog.Entry
	err   error
}

// Panel 1: quick action buttons above a manual entry form
type logPanel struct {
	client     *jira.Client
	inputs     [fieldCount]textinput.Model
	actions    []config.QuickAction
	onButtons  bool // true when the button row is selected instead of a field
	button     int  // selected quick action button
	cursor     int  // selected form field
	focused    bool
	submitting bool
	spinner    spinner.Model
	status     string
	err        string
}

func newLogPanel(client *jira.Client, actions []config.QuickAction) logPanel {
	p := logPanel{client: client, actions: actions, spinner: spinner.New()}
	placeholders := [fieldCount]string{"DEMO-123", "1h 30m", "now, or e.g. mon 9:30am", "optional"}
	for i := range p.inputs {
		p.inputs[i] = textinput.New()
		p.inputs[i].Placeholder = placeholders[i]
	}
	return p
}

// Focuses or blurs the panel so only the active panel shows a cursor
// Fills the issue field (e.g. from a search result) and moves on to the time field
// Comes from the search panel when the user picks an issue to log time against
func (p logPanel) setIssueKey(key string) logPanel {
	p.inputs[fieldKey].SetValue(key)
	p.onButtons, p.cursor = false, fieldDuration
	p.err, p.status = "", ""
	return p.focusCursor()
}

func (p logPanel) setFocused(focused bool) logPanel {
	p.focused = focused
	return p.focusCursor()
}

// Puts the text cursor on the selected field, if one is selected
func (p logPanel) focusCursor() logPanel {
	for i := range p.inputs {
		if p.focused && !p.onButtons && i == p.cursor {
			p.inputs[i].Focus()
		} else {
			p.inputs[i].Blur()
		}
	}
	return p
}

func (p logPanel) Update(msg tea.Msg) (logPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case worklogSubmittedMsg:
		return p.handleSubmitted(msg), nil
	case spinner.TickMsg:
		if !p.submitting {
			return p, nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	case tea.KeyMsg:
		// Ignore input while a worklog is posting
		if p.submitting {
			return p, nil
		}
		return p.handleKey(msg)
	}

	// Other messages (e.g. cursor blink) go to the selected field
	if p.onButtons {
		return p, nil
	}
	var cmd tea.Cmd
	p.inputs[p.cursor], cmd = p.inputs[p.cursor].Update(msg)
	return p, cmd
}

func (p logPanel) handleKey(key tea.KeyMsg) (logPanel, tea.Cmd) {
	switch key.String() {
	case "esc":
		return p.clear(), nil
	case "tab":
		return p.next().focusCursor(), nil
	}
	if p.onButtons {
		return p.handleButtonKey(key)
	}

	switch key.String() {
	case "up":
		// Moving up from the first field selects the buttons
		if p.cursor == fieldKey && len(p.actions) > 0 {
			p.onButtons = true
		} else if p.cursor > 0 {
			p.cursor--
		}
		return p.focusCursor(), nil
	case "down":
		if p.cursor < fieldComment {
			p.cursor++
		}
		return p.focusCursor(), nil
	case "enter":
		if p.cursor < fieldComment {
			p.cursor++
			return p.focusCursor(), nil
		}
		return p.submit(p.inputs[fieldKey].Value(), p.inputs[fieldDuration].Value(), p.inputs[fieldStarted].Value(), p.inputs[fieldComment].Value())
	}

	var cmd tea.Cmd
	p.inputs[p.cursor], cmd = p.inputs[p.cursor].Update(key)
	return p, cmd
}

// Moves to the next element in screen order: each button, then each field, then wraps
func (p logPanel) next() logPanel {
	switch {
	case p.onButtons && p.button < len(p.actions)-1:
		p.button++
	case p.onButtons:
		p.onButtons, p.cursor = false, fieldKey
	case p.cursor < fieldComment:
		p.cursor++
	case len(p.actions) > 0:
		p.onButtons, p.button = true, 0
	default:
		p.cursor = fieldKey
	}
	return p
}

// Left/right picks a button, enter logs it, down goes back to the form
func (p logPanel) handleButtonKey(key tea.KeyMsg) (logPanel, tea.Cmd) {
	switch key.String() {
	case "left":
		if p.button > 0 {
			p.button--
		}
	case "right":
		if p.button < len(p.actions)-1 {
			p.button++
		}
	case "down":
		p.onButtons = false
		return p.focusCursor(), nil
	case "enter":
		a := p.actions[p.button]
		return p.submit(a.IssueKey, a.Duration, "", a.Comment)
	}
	return p, nil
}

// Validates the input and starts posting the worklog in the background
func (p logPanel) submit(key, duration, started, comment string) (logPanel, tea.Cmd) {
	start, err := worklog.ParseStartTime(started, time.Now())
	if err != nil {
		p.err, p.status = err.Error(), ""
		return p, nil
	}
	entry, err := worklog.NewEntry(key, duration, comment, start)
	if err != nil {
		p.err, p.status = err.Error(), ""
		return p, nil
	}
	p.submitting = true
	p.err = ""
	p.status = fmt.Sprintf("Logging %s to %s...", worklog.FormatDuration(entry.Duration), entry.IssueKey)
	return p, tea.Batch(p.spinner.Tick, addWorklog(p.client, entry))
}

// Posts the worklog without blocking the UI
func addWorklog(client *jira.Client, entry worklog.Entry) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // 20 second timeout for Jira connection
		defer cancel()
		_, err := client.AddWorklog(ctx, entry)
		return worklogSubmittedMsg{entry: entry, err: err}
	}
}

// Shows the result and gets ready for the next entry
func (p logPanel) handleSubmitted(msg worklogSubmittedMsg) logPanel {
	p.submitting = false
	if msg.err != nil {
		p.err, p.status = msg.err.Error(), ""
		return p
	}
	// Keep the form as-is if a quick action was used
	if !p.onButtons {
		p = p.clear()
	}
	p.err = ""
	p.status = fmt.Sprintf("Logged %s to %s", worklog.FormatDuration(msg.entry.Duration), msg.entry.IssueKey)
	return p
}

// Empties the form and moves the cursor back to the first field
func (p logPanel) clear() logPanel {
	for i := range p.inputs {
		p.inputs[i].Reset()
	}
	p.cursor = fieldKey
	p.onButtons = false
	p.err, p.status = "", ""
	return p.focusCursor()
}

// Renders the panel body to fit inside the given width
func (p logPanel) View(width int) string {
	var b strings.Builder
	b.WriteString(p.buttonsView(width) + "\n")

	for i, in := range p.inputs {
		in.Width = max(width-labelWidth-3, 1) // 2 for the prompt, 1 for the cursor
		b.WriteString(fmt.Sprintf("%-*s%s\n", labelWidth, fieldLabels[i], in.View()))
	}

	// switch the status line based on the current state
	switch {
	case p.submitting:
		b.WriteString(p.spinner.View() + " " + p.status)
	case p.err != "":
		b.WriteString(errorStyle.Render(p.err))
	case p.status != "":
		b.WriteString(successStyle.Render("✓ " + p.status))
	case p.focused && p.onButtons:
		// Show what the selected button will log
		a := p.actions[p.button]
		b.WriteString(helpStyle.Render(fmt.Sprintf("%s · %s · enter to log", a.IssueKey, a.Duration)))
	default:
		b.WriteString(helpStyle.Render("tab next • enter log • esc clear"))
	}
	return b.String()
}

// Quick actions as a row of buttons, wrapping onto more lines if needed
func (p logPanel) buttonsView(width int) string {
	if len(p.actions) == 0 {
		return helpStyle.Render("No quick actions yet - add some!")
	}

	var lines []string
	line, lineW := "", 0
	for i, a := range p.actions {
		style := buttonStyle
		if p.focused && p.onButtons && i == p.button {
			style = selectedButtonStyle
		}
		btn := style.Render(a.Label)
		w := lipgloss.Width(btn)

		if lineW > 0 && lineW+1+w > width {
			lines = append(lines, line)
			line, lineW = "", 0
		}
		if lineW > 0 {
			line += " "
			lineW++
		}
		line += btn
		lineW += w
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}
