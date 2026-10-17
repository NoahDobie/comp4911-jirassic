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
)

// Max issues shown for one search
const searchLimit = 50

// Sent when a search finishes
// Sends to the main program so it can update the search panel and the issue list
type searchDoneMsg struct {
	seq    int
	issues []jira.Issue
	err    error
}

// Sent when the user picks a result to log time against
type issueSelectedMsg struct {
	key string
}

// Panel 3 - runs raw JQL and lists matching issues
type searchPanel struct {
	client    *jira.Client
	input     textinput.Model
	results   []jira.Issue
	onResults bool // true when the results list is selected instead of the input
	selected  int  // selected result
	seq       int  // increases with each search so old results can be ignored
	focused   bool
	loading   bool
	searched  bool
	spinner   spinner.Model
	err       string
}

func newSearchPanel() searchPanel {
	in := textinput.New()
	in.Placeholder = "assignee = currentUser() AND resolution = Unresolved"
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
		p.loading, p.searched = false, true
		p.results, p.err = msg.issues, ""
		if msg.err != nil {
			p.results, p.err = nil, msg.err.Error()
		}
		p.selected = 0
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
	jql := strings.TrimSpace(p.input.Value())
	if jql == "" {
		p.err = "Enter a JQL query"
		return p, nil
	}
	p.seq++
	p.loading, p.err = true, ""
	return p, tea.Batch(p.spinner.Tick, runSearch(p.client, jql, p.seq))
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
	in.Width = max(width-3, 1) // 2 for the prompt, 1 for the cursor

	var b strings.Builder
	b.WriteString(in.View() + "\n")

	switch {
	case p.loading:
		b.WriteString(p.spinner.View() + " Searching...")
		return b.String()
	case p.err != "":
		b.WriteString(errorStyle.Render(truncate(p.err, width)))
		return b.String()
	case !p.searched:
		b.WriteString(helpStyle.Render("enter search • ↓ results"))
		return b.String()
	case len(p.results) == 0:
		b.WriteString(helpStyle.Render("No matching issues"))
		return b.String()
	}

	count := fmt.Sprintf("%d issues", len(p.results))
	if len(p.results) == searchLimit {
		count = fmt.Sprintf("First %d issues", searchLimit)
	}
	b.WriteString(helpStyle.Render(count))

	// Scroll the list so the selected result stays visible
	visible := max(height-2, 1)
	start := max(p.selected-visible+1, 0)
	end := min(start+visible, len(p.results))
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
	if width < 50 {
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
	return "enter: search • ↓: results • esc: clear"
}
