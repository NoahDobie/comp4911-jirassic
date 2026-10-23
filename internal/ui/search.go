package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/jira"
	"jirassic-capstone/internal/matcha"
)

// Max issues shown for one search
const searchLimit = 50

// Sent when a search finishes
type searchDoneMsg struct {
	seq    int
	issues []jira.Issue
	err    error
}

// Sent when Matcha finishes turning a request into JQL
type translateDoneMsg struct {
	seq int
	jql string
	err error
}

const searchPlaceholder = "JQL or plain English, ex. my open bugs"

// Sent when the user picks a result to log time against
type issueSelectedMsg struct {
	key string
}

// Panel 3 - runs JQL, or asks Matcha to write JQL when the input isn't valid JQL
type searchPanel struct {
	client       *jira.Client
	matcha       *matcha.Client // nil when Matcha isn't set up
	input        textinput.Model
	query        string // text of the latest search
	generated    bool   // true while Matcha's JQL is waiting to be checked and run
	runGenerated bool   // true if the latest search ran Matcha's JQL, so it isn't sent back to Matcha
	loadText     string
	results      []jira.Issue
	onResults    bool // true when the results list is selected instead of the input
	selected     int  // selected result
	seq          int  // increases with each search so old results can be ignored
	focused      bool
	loading      bool
	searched     bool
	spinner      spinner.Model
	err          string
}

func newSearchPanel() searchPanel {
	in := textinput.New()
	in.Placeholder = searchPlaceholder
	return searchPanel{input: in, spinner: spinner.New()}
}

// Focuses or blurs the panel so only the active panel shows a cursor
func (p searchPanel) setFocused(focused bool) searchPanel {
	p.focused = focused
	if focused && !p.onResults {
		p.input.Focus()
	} else {
		p.input.Blur()
	}
	return p
}

func (p searchPanel) Update(msg tea.Msg) (searchPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchDoneMsg:
		// Ignore results from an older search
		if msg.seq != p.seq {
			return p, nil
		}
		// Jira couldn't read it as JQL, so treat it as plain English
		if jira.IsBadRequest(msg.err) && p.matcha != nil && !p.runGenerated {
			return p.translate(p.query)
		}
		p.loading, p.searched = false, true
		p.results, p.err = msg.issues, ""
		if msg.err != nil {
			p.results, p.err = nil, msg.err.Error()
		}
		p.selected = 0
		return p, nil
	case translateDoneMsg:
		if msg.seq != p.seq {
			return p, nil
		}
		p.loading = false
		if msg.err != nil {
			p.err = msg.err.Error()
			return p, nil
		}
		// Show the JQL in the input so the user can check or edit it before running
		p.input.SetValue(msg.jql)
		p.input.CursorEnd()
		p.generated = true
		return p, nil
	case spinner.TickMsg:
		if !p.loading {
			return p, nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	case tea.KeyMsg:
		if p.onResults {
			return p.handleResultKey(msg)
		}
		return p.handleInputKey(msg)
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p searchPanel) handleInputKey(key tea.KeyMsg) (searchPanel, tea.Cmd) {
	switch key.String() {
	case "enter":
		return p.search()
	case "down", "tab":
		if len(p.results) > 0 {
			p.onResults = true
			return p.setFocused(p.focused), nil
		}
		return p, nil
	case "esc":
		p.input.Reset()
		p.generated, p.err = false, ""
		return p, nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(key)
	return p, cmd
}

func (p searchPanel) handleResultKey(key tea.KeyMsg) (searchPanel, tea.Cmd) {
	switch key.String() {
	case "up":
		if p.selected > 0 {
			p.selected--
			return p, nil
		}
		p.onResults = false
		return p.setFocused(p.focused), nil
	case "down":
		if p.selected < len(p.results)-1 {
			p.selected++
		}
	case "tab", "esc":
		p.onResults = false
		return p.setFocused(p.focused), nil
	case "enter":
		key := p.results[p.selected].Key
		return p, func() tea.Msg { return issueSelectedMsg{key: key} }
	}
	return p, nil
}

// Starts the search in the background
func (p searchPanel) search() (searchPanel, tea.Cmd) {
	query := strings.TrimSpace(p.input.Value())
	if query == "" {
		p.err = "Enter JQL or describe the issues you want"
		return p, nil
	}
	p.seq++
	p.query, p.runGenerated = query, p.generated
	p.loading, p.loadText, p.err, p.generated = true, "Searching...", "", false
	return p, tea.Batch(p.spinner.Tick, runSearch(p.client, query, p.seq))
}

// Asks Matcha for JQL in the background - nothing is searched until the user runs it
func (p searchPanel) translate(request string) (searchPanel, tea.Cmd) {
	p.seq++
	p.loading, p.loadText, p.err = true, "Not JQL, asking Matcha...", ""
	return p, tea.Batch(p.spinner.Tick, runTranslate(p.matcha, request, p.seq))
}

func runTranslate(client *matcha.Client, request string, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		jql, err := client.TranslateToJQL(ctx, request, time.Now())
		return translateDoneMsg{seq: seq, jql: jql, err: err}
	}
}

func runSearch(client *jira.Client, jql string, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		issues, err := client.Search(ctx, jql, searchLimit)
		return searchDoneMsg{seq: seq, issues: issues, err: err}
	}
}

// Renders the panel body to fit inside width x height
func (p searchPanel) View(width, height int) string {
	in := p.input
	in.Width = max(width-len([]rune(in.Prompt))-1, 1) // 1 for the cursor

	var b strings.Builder
	b.WriteString(in.View() + "\n")

	// A status line replaces the results until there are some to show
	status, hasStatus := "", true
	switch {
	case p.loading:
		status = p.spinner.View() + " " + p.loadText
	case p.err != "":
		status = errorStyle.Render(truncate(p.err, width))
	case p.generated:
		status = askStyle.Render(truncate("Check Matcha's JQL, enter to run", width))
	case !p.searched:
		status = helpStyle.Render(truncate("enter search • ↓ results", width))
	case len(p.results) == 0:
		status = helpStyle.Render("No matching issues")
	default:
		hasStatus = false
	}
	if hasStatus {
		b.WriteString(status)
		return b.String()
	}

	count := fmt.Sprintf("%d issues", len(p.results))
	if len(p.results) == searchLimit {
		count = fmt.Sprintf("First %d issues", searchLimit)
	}
	b.WriteString(helpStyle.Render(count))

	start, end := visibleRange(p.selected, max(height-2, 1), len(p.results))
	keyW, statusW := p.columnWidths(width)
	for i := start; i < end; i++ {
		row := resultRow(p.results[i], width, keyW, statusW)
		if p.focused && p.onResults && i == p.selected {
			row = selectedRowStyle.Render(row)
		}
		b.WriteString("\n" + row)
	}
	return b.String()
}

// Key and status columns fit the longest value; status is hidden when narrow
func (p searchPanel) columnWidths(width int) (keyW, statusW int) {
	for _, is := range p.results {
		keyW = max(keyW, len(is.Key))
		statusW = max(statusW, len([]rune(is.Status)))
	}
	statusW = min(statusW, 14)
	if width < narrowWidth {
		statusW = 0
	}
	return keyW, statusW
}

// One result: key, status, then as much of the summary as fits
func resultRow(is jira.Issue, width, keyW, statusW int) string {
	row := fmt.Sprintf("%-*s ", keyW, is.Key)
	if statusW > 0 {
		row += fmt.Sprintf("%-*s ", statusW, truncate(is.Status, statusW))
	}
	return truncate(row+is.Summary, width)
}

// Keys shown in the status bar while this panel is focused
func (p searchPanel) help() string {
	if p.onResults {
		return "↑/↓: move • enter: log time to issue • esc: back to query"
	}
	if p.generated {
		return "enter: run Matcha's JQL • edit it first if needed • esc: discard"
	}
	return "enter: search (JQL or plain English) • ↓: results • esc: clear"
}
