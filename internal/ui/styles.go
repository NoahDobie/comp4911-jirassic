package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	borderColor       = lipgloss.Color("240") // Gray
	focusColor        = lipgloss.Color("196") // Red
	selectedTextColor = lipgloss.Color("231") // White

	titleStyle = lipgloss.NewStyle().Bold(true)
	helpStyle  = lipgloss.NewStyle().Foreground(borderColor)
	errorStyle = lipgloss.NewStyle().Foreground(focusColor)

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	todayStyle   = lipgloss.NewStyle().Foreground(focusColor).Bold(true) // Bold red

	askStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true) // Bold orange
	selectedRowStyle = lipgloss.NewStyle().Background(focusColor).Foreground(selectedTextColor)

	buttonStyle         = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("238")) // Dark gray
	selectedButtonStyle = selectedRowStyle.Padding(0, 1).Bold(true)
)

// Below this panel width, list rows drop their less important columns
const narrowWidth = 50

// Bordered panel with an outer size of w x h
func panelStyle(focused bool, w, h int) lipgloss.Style {
	color := borderColor
	if focused {
		color = focusColor
	}
	// Width/Height include padding but not the border, so subtract 2 for the border
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Width(w - 2).
		Height(h - 2).
		MaxHeight(h)
}

// Bold label on the left and right pushed to the right edge of width
func headerLine(label, right string, width int) string {
	gap := max(width-len([]rune(label))-len([]rune(right)), 1)
	return titleStyle.Render(label) + strings.Repeat(" ", gap) + right
}

// First and last+1 rows to show so the selected row stays visible
func visibleRange(selected, visible, n int) (start, end int) {
	start = max(selected-visible+1, 0)
	return start, min(start+visible, n)
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
