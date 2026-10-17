package ui

import "github.com/charmbracelet/lipgloss"

var (
	borderColor = lipgloss.Color("240") // Gray
	focusColor  = lipgloss.Color("196") // Red

	titleStyle = lipgloss.NewStyle().Bold(true)
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	todayStyle   = lipgloss.NewStyle().Foreground(focusColor).Bold(true) // Bold red

	buttonStyle         = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("238"))                                   // Dark gray
	selectedButtonStyle = lipgloss.NewStyle().Padding(0, 1).Background(focusColor).Foreground(lipgloss.Color("231")).Bold(true) // White on red
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
