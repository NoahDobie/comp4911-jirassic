package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/jira"
)

type state int

// Three states of the UI- setup, verifying, and ready
const (
	stateSetup state = iota
	stateVerifying
	stateReady
)

// Four panels of the UI and sizes
const (
	panelCount = 4
	minWidth   = 60
	minHeight  = 24
)

var panelTitles = [panelCount]string{"1 Quick Log", "2 Timesheet", "3 Search", "4 History"}

// Result of checking the Jira connection
type verifyMsg struct {
	client *jira.Client
	user   jira.User
	err    error
}

// Root: setup screen, connection check, then the four-panel grid
type Model struct {
	cfg     config.Config
	cfgPath string
	state   state
	setup   setupModel
	spinner spinner.Model

	client    *jira.Client
	user      jira.User
	worklog   logPanel
	timesheet timesheetPanel

	focus         int
	width, height int
	saveOnVerify  bool
}

// New creates a new Model with the given config and config path
func New(cfg config.Config, cfgPath string) Model {
	m := Model{
		cfg:       cfg,
		cfgPath:   cfgPath,
		state:     stateSetup,
		setup:     newSetup(cfg.JiraURL, cfg.JiraToken),
		spinner:   spinner.New(),
		worklog:   newLogPanel(nil, cfg.QuickActions),
		timesheet: newTimesheet(),
	}
	if cfg.HasJira() {
		m.state = stateVerifying
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.state == stateVerifying {
		return tea.Batch(m.spinner.Tick, verify(m.cfg.JiraURL, m.cfg.JiraToken))
	}
	return textinput.Blink
}

// Checks the connection in the background so the UI is never blocked
func verify(jiraURL, token string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // 20 second timeout for Jira connection
		defer cancel()
		client := jira.NewClient(jiraURL, token) // create a new Jira client
		user, err := client.CurrentUser(ctx)     // get the current user from Jira
		return verifyMsg{client: client, user: user, err: err}
	}
}

// Handles messages from the UI and updates the model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // update the model with the new window size
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case setupSubmitMsg: // when the user submits the setup for Jira, update the model with the new Jira URL and token, and start verifying the connection
		m.cfg.JiraURL, m.cfg.JiraToken = msg.url, msg.token
		m.state = stateVerifying
		m.saveOnVerify = true
		return m, tea.Batch(m.spinner.Tick, verify(msg.url, msg.token))
	case verifyMsg:
		return m.handleVerify(msg)
	case spinner.TickMsg: // update the spinner if we are in the verifying state
		// Ticks for other spinners fall through to the panels
		if m.state == stateVerifying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	switch m.state {
	case stateSetup: // update the setup model if we are in the setup state
		var cmd tea.Cmd
		m.setup, cmd = m.setup.Update(msg)
		return m, cmd
	case stateReady: // update the ready model if we are in the ready state
		return m.updateReady(msg)
	}
	return m, nil
}

// On failure, return to setup with the error so the user can fix their details
func (m Model) handleVerify(msg verifyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state = stateSetup
		m.setup.err = msg.err.Error()
		return m, textinput.Blink
	}
	// Only save details that have been verified
	if m.saveOnVerify {
		if err := config.Save(m.cfgPath, m.cfg); err != nil {
			m.state = stateSetup
			m.setup.err = err.Error()
			return m, nil
		}
		m.saveOnVerify = false
	}
	m.client, m.user, m.state = msg.client, msg.user, stateReady
	m.worklog.client = msg.client
	m.worklog = m.worklog.setFocused(m.focus == 0)

	m.timesheet.client, m.timesheet.user = msg.client, msg.user
	var cmd tea.Cmd
	m.timesheet, cmd = m.timesheet.load()
	return m, tea.Batch(textinput.Blink, cmd)
}

// Handles key presses when the UI is in the ready state, allowing the user to switch between panels
func (m Model) updateReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Tab is left for the panels to move between their own fields
		switch key.String() {
		case "shift+tab":
			return m.setFocus((m.focus + 1) % panelCount), nil
		case "alt+1", "alt+2", "alt+3", "alt+4":
			return m.setFocus(int(key.String()[4] - '1')), nil // jumps to panels 0-3
		}
		// Other keys only go to the focused panel
		var cmd tea.Cmd
		switch m.focus {
		case 0:
			m.worklog, cmd = m.worklog.Update(key)
		case 1:
			m.timesheet, cmd = m.timesheet.Update(key)
		}
		return m, cmd
	}

	// Non-key messages (results, spinner ticks) reach every panel
	var logCmd, sheetCmd tea.Cmd
	m.worklog, logCmd = m.worklog.Update(msg)
	m.timesheet, sheetCmd = m.timesheet.Update(msg)

	// Refresh the timesheet after a worklog is added
	if sub, ok := msg.(worklogSubmittedMsg); ok && sub.err == nil {
		m.timesheet, sheetCmd = m.timesheet.load()
	}
	return m, tea.Batch(logCmd, sheetCmd)
}

// Moves focus to panel i
func (m Model) setFocus(i int) Model {
	m.focus = i
	m.worklog = m.worklog.setFocused(i == 0)
	return m
}

// Renders the UI based on the current state and size of the terminal
func (m Model) View() string {
	switch m.state {
	case stateSetup:
		return m.setup.View()
	case stateVerifying:
		return m.spinner.View() + " Connecting to Jira..."
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small (%dx%d), need a minimum size of: %dx%d", m.width, m.height, minWidth, minHeight)
	}

	// Render the 2x2 grid of panels with a status bar at the bottom
	// Calculate the sizes of the panels based on the terminal size
	leftW, rightW, topH, bottomH := gridSizes(m.width, m.height)
	top := lipgloss.JoinHorizontal(lipgloss.Top, m.panel(0, leftW, topH), m.panel(1, rightW, topH))
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, m.panel(2, leftW, bottomH), m.panel(3, rightW, bottomH))
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom, m.statusBar())
}

// Outer panel sizes for the 2x2 grid, leaving the last row for the status bar
func gridSizes(width, height int) (leftW, rightW, topH, bottomH int) {
	leftW = width / 2
	rightW = width - leftW
	h := height - 1
	topH = h / 2
	bottomH = h - topH
	return leftW, rightW, topH, bottomH
}

// Renders panel i with an outer size of w x h
func (m Model) panel(i, w, h int) string {
	// Inner size: minus border and padding, then the title line
	bodyW, bodyH := w-4, h-3

	body := helpStyle.Render("Coming soon")
	switch i {
	case 0:
		body = m.worklog.View(bodyW)
	case 1:
		body = m.timesheet.View(bodyW, bodyH)
	}
	return panelStyle(i == m.focus, w, h).Render(titleStyle.Render(panelTitles[i]) + "\n" + body)
}

// Renders the status bar at the bottom of the UI with the current user's name and keyboard shortcuts available
func (m Model) statusBar() string {
	text := fmt.Sprintf("Connected as %s • shift+tab: next panel • alt+1-4: jump • ctrl+c: quit", m.user.DisplayName)
	if m.focus == 1 {
		text = m.timesheet.help() + " • " + text
	}
	return helpStyle.MaxWidth(m.width).Render(text)
}
