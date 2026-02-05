package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(1, 0, 0, 0)
)

// View implements tea.Model
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Show error banner if present
	errorBanner := ""
	if m.err != nil {
		errorBanner = errorStyle.Render(fmt.Sprintf("⚠ Error: %s", m.err)) + "\n\n"
	}

	// Render list
	listView := m.list.View()

	// Help text
	help := helpStyle.Render(fmt.Sprintf("enter: open | r: mark read | f: filter [%s] | q: quit | /: search", m.filterMode))

	return errorBanner + listView + "\n" + help
}
