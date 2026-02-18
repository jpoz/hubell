package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jpoz/hubell/internal/github"
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

	if m.showEngineerDetail {
		return m.renderEngineerDetail()
	}

	if m.showOrgDashboard {
		return m.renderOrgDashboard()
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

	// Calculate pane widths: 30% timeline / 35% notifications / 35% PRs
	tlWidth := m.width * 30 / 100
	notiWidth := m.width * 35 / 100
	prWidth := m.width - tlWidth - notiWidth

	// Height for list content (minus error banner, help, borders)
	listHeight := m.height - 5

	// Build timeline pane (left)
	tlContentWidth := max(tlWidth-2, 0)
	tlContentHeight := max(listHeight-2, 0)
	m.timelineList.SetSize(tlContentWidth, tlContentHeight)
	tlStyle := m.unfocusedPaneStyle()
	if m.focusedPane == TimelinePane {
		tlStyle = m.focusedPaneStyle()
	}
	timelinePane := tlStyle.
		Width(tlContentWidth).
		Height(tlContentHeight).
		Render(m.timelineList.View())

	// Build notifications pane (middle)
	notiContentWidth := max(notiWidth-2, 0)
	notiContentHeight := max(listHeight-2, 0)
	m.list.SetSize(notiContentWidth, notiContentHeight)
	notiStyle := m.unfocusedPaneStyle()
	if m.focusedPane == LeftPane {
		notiStyle = m.focusedPaneStyle()
	}
	notiPane := notiStyle.
		Width(notiContentWidth).
		Height(notiContentHeight).
		Render(m.list.View())

	// Build PRs pane (right)
	prContentWidth := max(prWidth-2, 0)
	prContentHeight := max(listHeight-2, 0)
	m.prList.SetSize(prContentWidth, prContentHeight)
	prStyle := m.unfocusedPaneStyle()
	if m.focusedPane == RightPane {
		prStyle = m.focusedPaneStyle()
	}
	prPane := prStyle.
		Width(prContentWidth).
		Height(prContentHeight).
		Render(m.prList.View())

	// Combine panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, timelinePane, notiPane, prPane)

	// Help text
	help := m.helpStyle().Render(fmt.Sprintf("tab: switch pane | enter: open | r: mark read | f: filter [%s] | d: dashboard | o: org | t: theme | q: quit | /: search", m.filterMode))

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

	bannerStyle := lipgloss.NewStyle().Foreground(color)
	checklist := m.renderLoadingChecklist()

	content := bannerStyle.Render(banner) + "\n\n" + checklist

	container := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return container.Render(content)
}

// spinnerFrames are braille-dot characters used as a text spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderLoadingChecklist renders the loading progress checklist
func (m *Model) renderLoadingChecklist() string {
	type step struct {
		key   github.LoadingStep
		label string
	}
	steps := []step{
		{github.StepNotifications, "Fetching notifications"},
		{github.StepPullRequests, "Loading pull requests"},
		{github.StepMergedPRs, "Checking merged PRs"},
		{github.StepWeeklyStats, "Loading weekly stats"},
	}

	doneStyle := lipgloss.NewStyle().Foreground(m.theme.StatusSuccess)
	activeStyle := lipgloss.NewStyle().Foreground(m.theme.FocusedBorder)
	pendingStyle := lipgloss.NewStyle().Foreground(m.theme.HelpText)
	barFillStyle := lipgloss.NewStyle().Foreground(m.theme.StatusSuccess)
	barEmptyStyle := lipgloss.NewStyle().Foreground(m.theme.HelpText)

	spinner := spinnerFrames[m.bannerFrame%len(spinnerFrames)]

	// Find the first non-done step (the active one)
	activeStep := github.LoadingStep(-1)
	for _, s := range steps {
		if !m.loadingSteps[s.key] {
			activeStep = s.key
			break
		}
	}

	// Pad all labels to the same width so columns stay aligned
	maxLabel := 0
	for _, s := range steps {
		if len(s.label) > maxLabel {
			maxLabel = len(s.label)
		}
	}

	const barWidth = 20
	// Fixed line width: " x  label  bar (NN/NN)" — wide enough for the progress bar line
	lineWidth := 4 + maxLabel + 1 + barWidth + 8 // prefix + label + space + bar + " (NN/NN)"

	var lines []string
	for _, s := range steps {
		paddedLabel := fmt.Sprintf("%-*s", maxLabel, s.label)
		if m.loadingSteps[s.key] {
			line := fmt.Sprintf(" ✓  %s", paddedLabel)
			line = fmt.Sprintf("%-*s", lineWidth, line)
			lines = append(lines, doneStyle.Render(line))
		} else if s.key == activeStep && s.key == github.StepPullRequests && m.prProgress.Total > 0 {
			prefix := activeStyle.Render(fmt.Sprintf(" %s  %s ", spinner, paddedLabel))
			bar := renderProgressBar(m.prProgress.Current, m.prProgress.Total, barWidth, barFillStyle, barEmptyStyle)
			counter := activeStyle.Render(fmt.Sprintf(" (%*d/%d)", digitWidth(m.prProgress.Total), m.prProgress.Current, m.prProgress.Total))
			lines = append(lines, prefix+bar+counter)
		} else if s.key == activeStep {
			line := fmt.Sprintf(" %s  %s", spinner, paddedLabel)
			line = fmt.Sprintf("%-*s", lineWidth, line)
			lines = append(lines, activeStyle.Render(line))
		} else {
			line := fmt.Sprintf("    %s", paddedLabel)
			line = fmt.Sprintf("%-*s", lineWidth, line)
			lines = append(lines, pendingStyle.Render(line))
		}
	}

	// Wrap in a fixed-width left-aligned block so centering doesn't shift
	block := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(lineWidth).Align(lipgloss.Left).Render(block)
}

// digitWidth returns the number of decimal digits in n.
func digitWidth(n int) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}

// renderProgressBar renders a block-style progress bar
func renderProgressBar(current, total, width int, fillStyle, emptyStyle lipgloss.Style) string {
	if total <= 0 {
		return emptyStyle.Render(strings.Repeat("░", width))
	}
	filled := current * width / total
	if filled > width {
		filled = width
	}
	return fillStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", width-filled))
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
