package ui

import "github.com/charmbracelet/lipgloss"

var (
	borderColor = lipgloss.Color("240") // Gray color for the border
	focusColor  = lipgloss.Color("196") // Red color for the focused border

	titleStyle = lipgloss.NewStyle().Bold(true)
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

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
		Width(w - 2). // Subtract 2 for the border
		Height(h - 2).
		MaxHeight(h)
}
