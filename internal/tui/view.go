package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) errorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(m.theme.Error).
		Bold(true)
}

func (m *Model) helpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(m.theme.HelpText).
		Padding(1, 0, 0, 0)
}

func (m *Model) focusedPaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder)
}

func (m *Model) unfocusedPaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.UnfocusedBorder)
}

// View implements tea.Model
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.showThemeSelector {
		return m.renderThemeSelector()
	}

	if m.showDashboard {
		return m.renderDashboard()
	}

	if m.loading {
		return m.renderBanner()
	}

	// Show error banner if present
	errorBanner := ""
	if m.err != nil {
		errorBanner = m.errorStyle().Render(fmt.Sprintf("⚠ Error: %s", m.err)) + "\n"
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
	leftStyle := m.unfocusedPaneStyle()
	if m.focusedPane == LeftPane {
		leftStyle = m.focusedPaneStyle()
	}
	leftPane := leftStyle.
		Width(leftContentWidth).
		Height(leftContentHeight).
		Render(m.list.View())

	// Build right pane (open PRs)
	rightContentWidth := max(rightWidth-2, 0)
	rightContentHeight := max(listHeight-2, 0)
	m.prList.SetSize(rightContentWidth, rightContentHeight)
	rightStyle := m.unfocusedPaneStyle()
	if m.focusedPane == RightPane {
		rightStyle = m.focusedPaneStyle()
	}
	rightPane := rightStyle.
		Width(rightContentWidth).
		Height(rightContentHeight).
		Render(m.prList.View())

	// Combine panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Help text
	help := m.helpStyle().Render(fmt.Sprintf("tab: switch pane | enter: open | r: mark read | f: filter [%s] | d: dashboard | t: theme | q: quit | /: search", m.filterMode))

	return errorBanner + panes + "\n" + help
}

// renderBanner renders the banner.txt centered in the terminal with a pulsing color
func (m *Model) renderBanner() string {
	banner := strings.TrimRight(bannerText, "\n")

	t := math.Sin(2 * math.Pi * float64(m.bannerFrame) / 40)
	frac := (t + 1) / 2

	dark := m.theme.BannerDark
	bright := m.theme.BannerBright
	r := dark[0] + int(frac*float64(bright[0]-dark[0]))
	g := dark[1] + int(frac*float64(bright[1]-dark[1]))
	b := dark[2] + int(frac*float64(bright[2]-dark[2]))

	color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))

	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(color)

	return style.Render(banner)
}

// renderThemeSelector draws the theme picker overlay centered on screen.
func (m *Model) renderThemeSelector() string {
	m.themeList.SetSize(30, len(themeOrder)*3+4)

	box := m.focusedPaneStyle().
		Width(32).
		Height(len(themeOrder)*3 + 6).
		Render(m.themeList.View())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
