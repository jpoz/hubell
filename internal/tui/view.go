package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(1, 0, 0, 0)

	focusedPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	unfocusedPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("241"))
)

// View implements tea.Model
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.loading {
		return m.renderBanner()
	}

	// Show error banner if present
	errorBanner := ""
	if m.err != nil {
		errorBanner = errorStyle.Render(fmt.Sprintf("⚠ Error: %s", m.err)) + "\n"
	}

	// Calculate pane widths (subtract 2 per pane for border)
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	// Height for list content (minus error banner, help, borders)
	listHeight := m.height - 5

	// Build left pane (notifications)
	leftContentWidth := max(leftWidth-2, 0)
	leftContentHeight := max(listHeight-2, 0)
	m.list.SetSize(leftContentWidth, leftContentHeight)
	leftStyle := unfocusedPaneStyle
	if m.focusedPane == LeftPane {
		leftStyle = focusedPaneStyle
	}
	leftPane := leftStyle.
		Width(leftContentWidth).
		Height(leftContentHeight).
		Render(m.list.View())

	// Build right pane (open PRs)
	rightContentWidth := max(rightWidth-2, 0)
	rightContentHeight := max(listHeight-2, 0)
	m.prList.SetSize(rightContentWidth, rightContentHeight)
	rightStyle := unfocusedPaneStyle
	if m.focusedPane == RightPane {
		rightStyle = focusedPaneStyle
	}
	rightPane := rightStyle.
		Width(rightContentWidth).
		Height(rightContentHeight).
		Render(m.prList.View())

	// Combine panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Help text
	help := helpStyle.Render(fmt.Sprintf("tab: switch pane | enter: open | r: mark read | f: filter [%s] | q: quit | /: search", m.filterMode))

	return errorBanner + panes + "\n" + help
}

// renderBanner renders the banner.txt centered in the terminal with a pulsing color
func (m *Model) renderBanner() string {
	banner := strings.TrimRight(bannerText, "\n")

	// Pulse between dim (color 238) and bright accent (color 62) using a sine wave.
	// ~2 second cycle at 50ms per frame (40 frames period).
	t := math.Sin(2 * math.Pi * float64(m.bannerFrame) / 40)
	// Map sine [-1, 1] to grayscale range [238, 255] blended toward color 62
	// We'll interpolate an RGB value for a smooth purple pulse.
	// Color 62 in 256-color is roughly (95, 135, 175) - a steel blue.
	// We'll pulse from dark gray to that color.
	frac := (t + 1) / 2 // normalize to [0, 1]
	r := 80 + int(frac*15)   // 80 -> 95
	g := 80 + int(frac*55)   // 80 -> 135
	b := 100 + int(frac*75)  // 100 -> 175

	color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))

	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(color)

	return style.Render(banner)
}
