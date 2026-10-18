package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/jira"
)

// How far back the history panel looks
const historyDays = 14

// Sent when recent worklogs finish loading
type historyLoadedMsg struct {
	seq  int
	logs []jira.Worklog
	err  error
}

// Panel 4: the user's most recent worklogs, newest first
type historyPanel struct {
	client   *jira.Client
	user     jira.User
	logs     []jira.Worklog
	selected int
	seq      int // increases with each load so old results can be ignored
	focused  bool
	loading  bool
	spinner  spinner.Model
	err      string
}

func newHistoryPanel() historyPanel {
	return historyPanel{spinner: spinner.New()}
}

func (p historyPanel) setFocused(focused bool) historyPanel {
	p.focused = focused
	return p
}

// Starts fetching recent worklogs in the background
func (p historyPanel) load() (historyPanel, tea.Cmd) {
	p.seq++
	p.loading, p.err = true, ""
	return p, tea.Batch(p.spinner.Tick, fetchHistory(p.client, p.user, p.seq))
}

func fetchHistory(client *jira.Client, user jira.User, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// From midnight historyDays ago up to the end of today
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		logs, err := client.GetWorklogs(ctx, user, today.AddDate(0, 0, -historyDays), today.AddDate(0, 0, 1))
		return historyLoadedMsg{seq: seq, logs: logs, err: err}
	}
}

func (p historyPanel) Update(msg tea.Msg) (historyPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		if msg.seq != p.seq {
			return p, nil
		}
		p.loading = false
		if msg.err != nil {
			p.err = msg.err.Error()
			return p, nil
		}
		p.logs = newestFirst(msg.logs)
		p.selected = min(p.selected, max(len(p.logs)-1, 0))
		return p, nil
	case spinner.TickMsg:
		if !p.loading {
			return p, nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			p.selected = max(p.selected-1, 0)
		case "down":
			p.selected = min(p.selected+1, max(len(p.logs)-1, 0))
		case "r":
			return p.load()
		}
	}
	return p, nil
}

// Reverses worklogs that arrive oldest first
func newestFirst(logs []jira.Worklog) []jira.Worklog {
	out := make([]jira.Worklog, len(logs))
	for i, l := range logs {
		out[len(logs)-1-i] = l
	}
	return out
}

// Renders the panel body to fit inside width x height
func (p historyPanel) View(width, height int) string {
	var total time.Duration
	for _, l := range p.logs {
		total += l.Duration
	}

	label := fmt.Sprintf("Last %d days", historyDays)
	right := fmt.Sprintf("%d logs · %sh", len(p.logs), formatHours(total))
	if p.loading {
		right = p.spinner.View()
	}
	gap := max(width-len([]rune(label))-len([]rune(right)), 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render(label) + strings.Repeat(" ", gap) + right)

	switch {
	case p.err != "":
		b.WriteString("\n" + errorStyle.Render(truncate(p.err, width)))
		return b.String()
	case len(p.logs) == 0 && !p.loading:
		b.WriteString("\n" + helpStyle.Render("Nothing logged recently"))
		return b.String()
	}

	// Scroll the list so the selected worklog stays visible
	visible := max(height-1, 1)
	start := max(p.selected-visible+1, 0)
	end := min(start+visible, len(p.logs))
	keyW := 0
	for _, l := range p.logs {
		keyW = max(keyW, len(l.IssueKey))
	}

	for i := start; i < end; i++ {
		row := historyRow(p.logs[i], width, keyW)
		if p.focused && i == p.selected {
			row = selectedRowStyle.Render(row)
		}
		b.WriteString("\n" + row)
	}
	return b.String()
}

// One worklog: date (and time when there's room), key, hours, then as much of the title as fits
func historyRow(l jira.Worklog, width, keyW int) string {
	layout := "Jan 02"
	if width >= 50 {
		layout = "Jan 02 15:04"
	}
	hours := formatHours(l.Duration) + "h"
	row := fmt.Sprintf("%s  %-*s %5s  %s", l.Started.Local().Format(layout), keyW, l.IssueKey, hours, l.IssueSummary)
	return truncate(row, width)
}

// Keys shown in the status bar while this panel is focused
func (p historyPanel) help() string {
	return "↑/↓: move • r: refresh"
}
