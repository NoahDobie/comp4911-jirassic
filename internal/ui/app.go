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
	"jirassic-capstone/internal/matcha"
	"jirassic-capstone/internal/worklog"
)

type state int

// Three states of the UI- setup (settings form, not connected yet), verifying, and ready
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

// Root: connection check or settings form, then the four-panel grid
type Model struct {
	cfg     config.Config
	cfgPath string
	state   state
	spinner spinner.Model

	user      jira.User
	worklog   logPanel
	timesheet timesheetPanel
	search    searchPanel
	history   historyPanel

	settings     settingsModel
	showSettings bool // open over the panels with ctrl+x

	focus         int
	width, height int
}

// Root model, starting at the settings form or the connection check
func New(cfg config.Config, cfgPath string) Model {
	m := Model{
		cfg:       cfg,
		cfgPath:   cfgPath,
		state:     stateSetup,
		settings:  newSettings(cfg),
		spinner:   spinner.New(),
		worklog:   newLogPanel(nil, cfg.QuickActions),
		timesheet: newTimesheet(),
		search:    newSearchPanel(),
		history:   newHistoryPanel(),
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
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		client := jira.NewClient(jiraURL, token)
		user, err := client.CurrentUser(ctx)
		return verifyMsg{client: client, user: user, err: err}
	}
}

// Handles messages from the UI and updates the model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case verifyMsg:
		return m.handleVerify(msg)
	case settingsSaveMsg:
		return m.saveSettings(msg.cfg)
	case spinner.TickMsg:
		// Ticks for other spinners fall through to the panels
		if m.state == stateVerifying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	switch m.state {
	case stateSetup:
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		return m, cmd
	case stateReady:
		return m.updateReady(msg)
	}
	return m, nil
}

// Connects on success, saving details that came from the settings form
// On failure the settings form shows the error so the user can fix their details
func (m Model) handleVerify(msg verifyMsg) (tea.Model, tea.Cmd) {
	m.settings.checking = false
	if msg.err != nil {
		if m.state == stateVerifying {
			// The saved details failed at launch
			m.state = stateSetup
		}
		m.settings.err = msg.err.Error()
		return m, textinput.Blink
	}
	// The launch check uses details that are already saved
	if m.state != stateVerifying {
		if err := config.Save(m.cfgPath, m.settings.pending); err != nil {
			m.settings.err = err.Error()
			return m, nil
		}
		m.cfg = m.settings.pending
	}
	m.state, m.showSettings = stateReady, false
	return m.connect(msg)
}

// Hands the verified client and user to the panels and loads their data
func (m Model) connect(msg verifyMsg) (tea.Model, tea.Cmd) {
	m.user = msg.user
	m.worklog.client = msg.client
	m.worklog = m.worklog.setFocused(m.focus == 0)
	m.search.client = msg.client
	m = m.applyConfig(m.cfg)

	m.timesheet.client, m.timesheet.user = msg.client, msg.user
	m.history.client, m.history.user = msg.client, msg.user
	m, loadCmd := m.reloadWorklogs()
	return m, tea.Batch(textinput.Blink, loadCmd)
}

// Sent when one fetch for both the timesheet and history finishes
type sharedWorklogsMsg struct {
	week, recent [2]time.Time // each panel's range, so each only gets its own logs
	seq          int          // history's load number, so an old result can be ignored
	logs         []jira.Worklog
	err          error
}

// Reloads the timesheet and history
// When the timesheet shows this week, history's last 14 days already cover most of it,
// so one fetch serves both instead of downloading busy shared issues twice
func (m Model) reloadWorklogs() (Model, tea.Cmd) {
	var sheetCmd, historyCmd tea.Cmd
	if m.timesheet.offset != 0 {
		m.timesheet, sheetCmd = m.timesheet.load()
		m.history, historyCmd = m.history.load()
		return m, tea.Batch(sheetCmd, historyCmd)
	}

	m.timesheet, sheetCmd = m.timesheet.startLoading()
	m.history, historyCmd = m.history.startLoading()
	now := time.Now()
	msg := sharedWorklogsMsg{seq: m.history.seq}
	msg.week[0], msg.week[1] = worklog.WeekRange(now, 0)
	msg.recent[0], msg.recent[1] = worklog.RecentRange(now, historyDays)
	return m, tea.Batch(sheetCmd, historyCmd, fetchShared(m.history.client, m.history.user, msg))
}

// Fetches the worklogs covering both ranges in msg
func fetchShared(client *jira.Client, user jira.User, msg sharedWorklogsMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), worklogLoadTimeout)
		defer cancel()
		from, to := msg.recent[0], msg.recent[1]
		if msg.week[0].Before(from) {
			from = msg.week[0]
		}
		if msg.week[1].After(to) {
			to = msg.week[1]
		}
		msg.logs, msg.err = client.GetWorklogs(ctx, user, from, to)
		return msg
	}
}

// Worklogs started in (from, to)
func within(logs []jira.Worklog, from, to time.Time) []jira.Worklog {
	var out []jira.Worklog
	for _, l := range logs {
		if !l.Started.Before(from) && l.Started.Before(to) {
			out = append(out, l)
		}
	}
	return out
}

// Gives each panel the part of a shared fetch that falls in its own range
// They then handle it like their own load, including ignoring it if they've moved on
func (m Model) splitShared(msg sharedWorklogsMsg) Model {
	week := weekLoadedMsg{offset: 0, logs: within(msg.logs, msg.week[0], msg.week[1]), err: msg.err}
	recent := historyLoadedMsg{seq: msg.seq, logs: within(msg.logs, msg.recent[0], msg.recent[1]), err: msg.err}
	m.timesheet, _ = m.timesheet.Update(week)
	m.history, _ = m.history.Update(recent)
	return m
}

// Uses cfg from now on: its quick actions and the optional Matcha client
func (m Model) applyConfig(cfg config.Config) Model {
	m.cfg = cfg
	m.worklog = m.worklog.setActions(cfg.QuickActions)
	m.search.matcha = nil
	if cfg.HasMatcha() {
		m.search.matcha = matcha.NewClient(cfg.MatchaURL, cfg.MatchaKey, cfg.MatchaMission)
	}
	return m
}

// Saves new settings, checking the Jira connection first if not connected yet or those details changed
func (m Model) saveSettings(cfg config.Config) (tea.Model, tea.Cmd) {
	if m.state == stateSetup || cfg.JiraURL != m.cfg.JiraURL || cfg.JiraToken != m.cfg.JiraToken {
		m.settings.checking, m.settings.pending = true, cfg
		return m, tea.Batch(m.settings.spinner.Tick, verify(cfg.JiraURL, cfg.JiraToken))
	}
	if err := config.Save(m.cfgPath, cfg); err != nil {
		m.settings.err = err.Error()
		return m, nil
	}
	m.showSettings = false
	return m.applyConfig(cfg), textinput.Blink
}

// Handles key presses when the UI is in the ready state, allowing the user to switch between panels
func (m Model) updateReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// The settings pop-up takes every key while it's open
		if m.showSettings {
			var cmd tea.Cmd
			m.settings, cmd = m.settings.Update(key)
			return m, cmd
		}
		// Tab is left for the panels to move between their own fields
		switch key.String() {
		case "ctrl+x":
			m.settings, m.showSettings = newSettings(m.cfg), true
			m.settings.canClose = true
			return m, textinput.Blink
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
		case 2:
			m.search, cmd = m.search.Update(key)
		case 3:
			m.history, cmd = m.history.Update(key)
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case issueSelectedMsg:
		// A picked search result fills in the quick log form
		m.worklog = m.worklog.setIssueKey(msg.key)
		return m.setFocus(0), textinput.Blink
	case settingsCloseMsg:
		m.showSettings = false
		return m, textinput.Blink
	case sharedWorklogsMsg:
		return m.splitShared(msg), nil
	}

	// Non-key messages (results, spinner ticks) reach every panel, and settings when open
	var settingsCmd, logCmd, sheetCmd, searchCmd, historyCmd tea.Cmd
	if m.showSettings {
		m.settings, settingsCmd = m.settings.Update(msg)
	}
	m.worklog, logCmd = m.worklog.Update(msg)
	m.timesheet, sheetCmd = m.timesheet.Update(msg)
	m.search, searchCmd = m.search.Update(msg)
	m.history, historyCmd = m.history.Update(msg)

	// Refresh the timesheet and history after a worklog is added
	if sub, ok := msg.(worklogSubmittedMsg); ok && sub.err == nil {
		var loadCmd tea.Cmd
		m, loadCmd = m.reloadWorklogs()
		return m, tea.Batch(settingsCmd, logCmd, searchCmd, loadCmd)
	}
	return m, tea.Batch(settingsCmd, logCmd, sheetCmd, searchCmd, historyCmd)
}

// Moves focus to panel i
func (m Model) setFocus(i int) Model {
	m.focus = i
	m.worklog = m.worklog.setFocused(i == 0)
	m.search = m.search.setFocused(i == 2)
	m.history = m.history.setFocused(i == 3)
	return m
}

// Renders the UI based on the current state and size of the terminal
func (m Model) View() string {
	if m.state == stateVerifying {
		return m.spinner.View() + " Connecting to Jira..."
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small (%dx%d), need a minimum size of: %dx%d", m.width, m.height, minWidth, minHeight)
	}
	if m.state == stateSetup || m.showSettings {
		return m.settings.View(m.width, m.height)
	}

	// 2x2 grid of panels with a status bar at the bottom
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
	// Inner size: minus border and padding, then the title line and the gap below it
	bodyW, bodyH := w-4, h-4

	var body string
	switch i {
	case 0:
		body = m.worklog.View(bodyW)
	case 1:
		body = m.timesheet.View(bodyW, bodyH)
	case 2:
		body = m.search.View(bodyW, bodyH)
	case 3:
		body = m.history.View(bodyW, bodyH)
	}
	return panelStyle(i == m.focus, w, h).Render(titleStyle.Render(panelTitles[i]) + "\n\n" + body)
}

// Renders the status bar at the bottom of the UI with the current user's name and keyboard shortcuts available
func (m Model) statusBar() string {
	text := fmt.Sprintf("Connected as %s • shift+tab: next panel • alt+1-4: jump • ctrl+x: settings • ctrl+c: quit", m.user.DisplayName)
	switch m.focus {
	case 1:
		text = m.timesheet.help() + " • " + text
	case 2:
		text = m.search.help() + " • " + text
	case 3:
		text = m.history.help() + " • " + text
	}
	return helpStyle.MaxWidth(m.width).Render(text)
}
