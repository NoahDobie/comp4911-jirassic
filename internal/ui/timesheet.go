package ui

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	key  string
	days [7]time.Duration
}

// Time logged across the whole week
func (r issueRow) total() time.Duration {
	var sum time.Duration
	for _, d := range r.days {
		sum += d
	}
	return sum
}

// Panel 2 layout - rows are Jira issue keys, columns are the days of the week
type timesheetPanel struct {
	client  *jira.Client
	user    jira.User
	offset  int // weeks from this week, ex. -1 is last week
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
	p, tick := p.startLoading()
	return p, tea.Batch(tick, fetchWeek(p.client, p.user, p.offset))
}

// Shows the spinner while worklogs load, without starting a fetch itself
func (p timesheetPanel) startLoading() (timesheetPanel, tea.Cmd) {
	p.loading, p.err = true, ""
	return p, p.spinner.Tick
}

// Time limit for loading a range of worklogs
// Shared issues can have thousands of worklogs that Jira only sends all at once
const worklogLoadTimeout = 2 * time.Minute

func fetchWeek(client *jira.Client, user jira.User, offset int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), worklogLoadTimeout)
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
		i := slices.IndexFunc(rows, func(r issueRow) bool { return r.key == l.IssueKey })
		if i < 0 {
			rows = append(rows, issueRow{key: l.IssueKey})
			i = len(rows) - 1
		}
		rows[i].days[worklog.DayIndex(l.Started)] += l.Duration
	}
	return rows
}

const minKeyW = 9

// Grid lines between cells
var (
	cellSep = helpStyle.Render("│")
	ruleSep = helpStyle.Render("┼")
)

// Column widths for the grid, not counting the separator in front of each day and total cell
type gridLayout struct {
	keyW, dayW, totalW int
	days               int    // 7 for the whole week, 5 for Mon to Fri
	pad                string // space after each value so it doesn't touch the next separator
	showTotal          bool
}

// Picks the roomiest layout that still leaves minKeyW for issue keys
// Narrow panels drop the padding first, then the total column, then the weekend
func timesheetLayout(width int) gridLayout {
	layouts := []gridLayout{
		{days: 7, dayW: 5, totalW: 7, pad: " ", showTotal: true}, // "12.5 " and " Total "
		{days: 7, dayW: 4, totalW: 5, showTotal: true},
		{days: 7, dayW: 4},
		{days: 5, dayW: 4},
	}
	var l gridLayout
	for _, l = range layouts {
		l.keyW = width - (l.dayW+1)*l.days
		if l.showTotal {
			l.keyW -= l.totalW + 1
		}
		if l.keyW >= minKeyW {
			break
		}
	}
	return l
}

// Right-aligns s in a cell of width w, leaving room for the layout's padding
func (l gridLayout) cell(s string, w int) string {
	return fmt.Sprintf("%*s%s", w-len(l.pad), s, l.pad)
}

// Renders the panel body to fit inside width x height
func (p timesheetPanel) View(width, height int) string {
	now := time.Now()
	start, end := worklog.WeekRange(now, p.offset)

	// Sum each day across issues
	totals := issueRow{key: "Total"}
	for _, r := range p.rows {
		for d, dur := range r.days {
			totals.days[d] += dur
		}
	}

	label := start.Format("Jan 2") + " – " + end.AddDate(0, 0, -1).Format("Jan 2")
	if p.offset == 0 {
		label += " (this week)"
	}
	right := formatTotal(totals.total())
	if p.loading {
		right = p.spinner.View()
	}

	var b strings.Builder
	b.WriteString(headerLine(label, right, width))
	if p.err != "" {
		b.WriteString("\n" + errorStyle.Render(truncate(p.err, width)))
		return b.String()
	}

	// Column headings with today highlighted
	l := timesheetLayout(width)
	header := helpStyle.Render(fmt.Sprintf("%-*s", l.keyW, "Issue"))
	for d := range l.days {
		style := helpStyle
		if p.offset == 0 && d == worklog.DayIndex(now) {
			style = todayStyle
		}
		header += cellSep + style.Render(l.cell(start.AddDate(0, 0, d).Format("Mon"), l.dayW))
	}
	if l.showTotal {
		header += cellSep + helpStyle.Render(l.cell("Total", l.totalW))
	}
	rule := l.rule()
	b.WriteString("\n" + header + "\n" + rule)

	if len(p.rows) == 0 {
		if !p.loading {
			b.WriteString("\n" + helpStyle.Render("Nothing logged this week"))
		}
		return b.String()
	}

	// Leave room for the label, headings, two rules, and day totals
	rows := p.rows
	if fit := max(height-5, 1); len(rows) > fit {
		rows = rows[:fit-1]
	}
	for _, r := range rows {
		b.WriteString("\n" + l.row(r, lipgloss.NewStyle()))
	}
	if hidden := len(p.rows) - len(rows); hidden > 0 {
		b.WriteString("\n" + helpStyle.Render(fmt.Sprintf("+%d more", hidden)))
	}

	b.WriteString("\n" + rule + "\n" + l.row(totals, titleStyle))
	return b.String()
}

// One row of the grid - issue key, then day of week sums, each cell drawn with style
func (l gridLayout) row(r issueRow, style lipgloss.Style) string {
	row := style.Render(fmt.Sprintf("%-*s", l.keyW, truncate(r.key, l.keyW-1)))
	for d := range l.days {
		row += cellSep + style.Render(l.cell(formatHours(r.days[d]), l.dayW))
	}
	if l.showTotal {
		row += cellSep + style.Render(l.cell(formatHours(r.total()), l.totalW))
	}
	return row
}

// Horizontal line across the grid, crossing each column separator
func (l gridLayout) rule() string {
	line := helpStyle.Render(strings.Repeat("─", l.keyW))
	for range l.days {
		line += ruleSep + helpStyle.Render(strings.Repeat("─", l.dayW))
	}
	if l.showTotal {
		line += ruleSep + helpStyle.Render(strings.Repeat("─", l.totalW))
	}
	return line
}

// Formats a duration as hours to one decimal, ex. 1.5, or a - for nothing logged
func formatHours(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", math.Round(d.Hours()*10)/10)
}

// Formats a total with an h, showing 0.0h rather than - when nothing is logged
func formatTotal(d time.Duration) string {
	return fmt.Sprintf("%.1fh", math.Round(d.Hours()*10)/10)
}

// Keys shown in the status bar while this panel is focused
func (p timesheetPanel) help() string {
	return "←/→: week • t: this week • r: refresh"
}
