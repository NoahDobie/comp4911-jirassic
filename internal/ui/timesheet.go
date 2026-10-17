package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/worklog"
)

// Sent when a week of worklogs finishes loading
type weekLoadedMsg struct {
	offset int
	logs   []jira.Worklog
	err    error
}

// One row of the timesheet: time logged to an issue on each day
type issueRow struct {
	key   string
	days  [7]time.Duration
	total time.Duration
}

// Panel 2 layout - rows are Jira issue keys, columns are the days of the week.
type timesheetPanel struct {
	client  *jira.Client
	user    jira.User
	offset  int // weeks from this week, e.g. -1 is last week
	rows    []issueRow
	loading bool
	spinner spinner.Model
	err     string
}

func newTimesheet() timesheetPanel {
	return timesheetPanel{spinner: spinner.New()}
}

// Starts fetching the selected week in the background
func (p timesheetPanel) load() (timesheetPanel, tea.Cmd) {
	p.loading, p.err = true, ""
	return p, tea.Batch(p.spinner.Tick, fetchWeek(p.client, p.user, p.offset))
}

func fetchWeek(client *jira.Client, user jira.User, offset int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		from, to := worklog.WeekRange(time.Now(), offset)
		logs, err := client.GetWorklogs(ctx, user, from, to)
		return weekLoadedMsg{offset: offset, logs: logs, err: err}
	}
}

func (p timesheetPanel) Update(msg tea.Msg) (timesheetPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case weekLoadedMsg:
		// Ignore a slow result for a week the user already moved away from
		if msg.offset != p.offset {
			return p, nil
		}
		p.loading = false
		if msg.err != nil {
			p.err = msg.err.Error()
			return p, nil
		}
		p.rows = groupByIssue(msg.logs)
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
		case "left":
			p.offset--
			return p.load()
		case "right":
			p.offset++
			return p.load()
		case "t":
			p.offset = 0
			return p.load()
		case "r":
			return p.load()
		}
	}
	return p, nil
}

// Totals worklogs per issue and day, with issues in the order first logged
// Sometimes the same issue is logged to multiple times in a day, so we sum those
func groupByIssue(logs []jira.Worklog) []issueRow {
	var rows []issueRow
	for _, l := range logs {
		i := 0
		for i < len(rows) && rows[i].key != l.IssueKey {
			i++
		}
		if i == len(rows) {
			rows = append(rows, issueRow{key: l.IssueKey})
		}
		day := (int(l.Started.Local().Weekday()) + 6) % 7
		rows[i].days[day] += l.Duration
		rows[i].total += l.Duration
	}
	return rows
}

const (
	shownDays  = 5 // Mon to Fri
	cellWidth  = 5
	totalWidth = 6
	minKeyW    = 9
)

// Renders the panel body to fit inside width x height
func (p timesheetPanel) View(width, height int) string {
	now := time.Now()
	start, end := worklog.WeekRange(now, p.offset)

	// Sum each day across issues, and the whole week
	var dayTotals [7]time.Duration
	var week time.Duration
	for _, r := range p.rows {
		for d, dur := range r.days {
			dayTotals[d] += dur
		}
		week += r.total
	}

	label := start.Format("Jan 2") + " – " + end.AddDate(0, 0, -1).Format("Jan 2")
	if p.offset == 0 {
		label += " (this week)"
	}
	right := formatHours(week) + "h"
	if p.loading {
		right = p.spinner.View()
	}
	gap := max(width-len([]rune(label))-len([]rune(right)), 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render(label) + strings.Repeat(" ", gap) + right)
	if p.err != "" {
		b.WriteString("\n" + errorStyle.Render(truncate(p.err, width)))
		return b.String()
	}

	// The key column gets the space left over; the total column only shows when there's room
	showTotal := width-cellWidth*shownDays-totalWidth >= minKeyW
	keyW := width - cellWidth*shownDays
	if showTotal {
		keyW -= totalWidth
	}

	// Column headings and today column highlighted
	header := fmt.Sprintf("%-*s", keyW, "Issue")
	for d := range shownDays {
		cell := fmt.Sprintf("%*s", cellWidth, start.AddDate(0, 0, d).Format("Mon"))
		if p.offset == 0 && d == (int(now.Weekday())+6)%7 {
			cell = todayStyle.Render(cell)
		}
		header += cell
	}
	if showTotal {
		header += fmt.Sprintf("%*s", totalWidth, "Total")
	}
	b.WriteString("\n" + helpStyle.Render(header))

	if len(p.rows) == 0 {
		if !p.loading {
			b.WriteString("\n" + helpStyle.Render("Nothing logged this week"))
		}
		return b.String()
	}

	// Leave room for the label, headings, and day totals
	rows := p.rows
	if fit := max(height-3, 1); len(rows) > fit {
		rows = rows[:fit-1]
	}
	for _, r := range rows {
		b.WriteString("\n" + p.rowView(r, keyW, showTotal))
	}
	if hidden := len(p.rows) - len(rows); hidden > 0 {
		b.WriteString("\n" + helpStyle.Render(fmt.Sprintf("+%d more", hidden)))
	}

	totals := issueRow{key: "Total", days: dayTotals, total: week}
	b.WriteString("\n" + titleStyle.Render(p.rowView(totals, keyW, showTotal)))
	return b.String()
}

// One row of the grid - issue key, then day of week sums
func (p timesheetPanel) rowView(r issueRow, keyW int, showTotal bool) string {
	row := fmt.Sprintf("%-*s", keyW, truncate(r.key, keyW-1))
	for d := range shownDays {
		row += fmt.Sprintf("%*s", cellWidth, formatHours(r.days[d]))
	}
	if showTotal {
		row += fmt.Sprintf("%*s", totalWidth, formatHours(r.total))
	}
	return row
}

// Formats a duration as hours to one decimal, ex. 1.5, or a - for nothing logged
func formatHours(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", math.Round(d.Hours()*10)/10)
}

// Cuts s to fit in width characters, ending with … if shortened
func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:max(width, 0)])
	}
	return string(r[:width-1]) + "…"
}

// Keys shown in the status bar while this panel is focused
func (p timesheetPanel) help() string {
	return "←/→: week • t: this week • r: refresh"
}
