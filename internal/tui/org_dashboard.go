package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jpoz/hubell/internal/github"
)

var orgLoadingSteps = []github.OrgLoadingStep{
	github.OrgStepMembers,
	github.OrgStepMergedPRs,
	github.OrgStepOpenPRs,
	github.OrgStepCommits,
	github.OrgStepReviews,
	github.OrgStepDiffStats,
	github.OrgStepAggregate,
}

// OrgSortColumn identifies which column the org table is sorted by.
type OrgSortColumn int

const (
	SortByCommits OrgSortColumn = iota
	SortByReviews
	SortByLOC
	SortByMerged
	SortByOpen
	SortByName
	orgSortColumnCount
)

func (s OrgSortColumn) String() string {
	switch s {
	case SortByCommits:
		return "Commits"
	case SortByReviews:
		return "Reviews"
	case SortByLOC:
		return "LOC"
	case SortByMerged:
		return "Merged"
	case SortByOpen:
		return "Open"
	case SortByName:
		return "Name"
	default:
		return ""
	}
}

// sortOrgMembers sorts the org member list by the current sort column.
func (m *Model) sortOrgMembers() {
	sort.Slice(m.orgMembers, func(i, j int) bool {
		switch m.orgSortColumn {
		case SortByCommits:
			return m.orgMembers[i].Commits > m.orgMembers[j].Commits
		case SortByReviews:
			return m.orgMembers[i].Reviews > m.orgMembers[j].Reviews
		case SortByLOC:
			return (m.orgMembers[i].Additions + m.orgMembers[i].Deletions) > (m.orgMembers[j].Additions + m.orgMembers[j].Deletions)
		case SortByMerged:
			return len(m.orgMembers[i].MergedPRs) > len(m.orgMembers[j].MergedPRs)
		case SortByOpen:
			return len(m.orgMembers[i].OpenPRs) > len(m.orgMembers[j].OpenPRs)
		case SortByName:
			return m.orgMembers[i].Login < m.orgMembers[j].Login
		default:
			return m.orgMembers[i].Commits > m.orgMembers[j].Commits
		}
	})
}

// totalMergedPRs returns the sum of merged PRs across all org members.
func totalMergedPRs(members []github.OrgMemberActivity) int {
	total := 0
	for _, m := range members {
		total += len(m.MergedPRs)
	}
	return total
}

// totalOrgStats returns total commits, reviews, and LOC across all org members.
func totalOrgStats(members []github.OrgMemberActivity) (commits, reviews, loc int) {
	for _, m := range members {
		commits += m.Commits
		reviews += m.Reviews
		loc += m.Additions + m.Deletions
	}
	return
}

// renderOrgDashboard renders the org activity overlay.
func (m *Model) renderOrgDashboard() string {
	if m.orgInputActive {
		return m.renderOrgInput()
	}

	maxWidth := max(m.width-2, 40)
	maxHeight := max(m.height-2, 10)

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Title).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(m.theme.Subtle)
	selectedStyle := lipgloss.NewStyle().Foreground(m.theme.SelectedForeground).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.NormalForeground)
	errorStyle := lipgloss.NewStyle().Foreground(m.theme.Error).Bold(true)

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s - Org Activity (last 7 days)", m.orgName)))
	b.WriteString("\n\n")

	if m.orgLoading {
		b.WriteString(m.renderOrgLoading(maxWidth-6, accentStyle, subtleStyle))
		b.WriteString("\n\n")
		b.WriteString(subtleStyle.Render("esc: close"))
	} else if m.orgError != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.orgError)))
		b.WriteString("\n\n")
		b.WriteString(subtleStyle.Render("r: retry  esc: close"))
	} else if len(m.orgMembers) == 0 {
		b.WriteString(subtleStyle.Render("No active engineers found in the last 7 days."))
		b.WriteString("\n\n")
		b.WriteString(subtleStyle.Render("r: refresh  esc: close"))
	} else {
		// Column headers
		innerWidth := maxWidth - 6 // padding
		rowPrefix := "  "
		selectedRowPrefix := "▸ "
		statsWidth := 47 // " %9s %9s %8s %8s %8s"
		tableWidth := max(innerWidth-lipgloss.Width(rowPrefix), 0)
		nameWidth := max(tableWidth-statsWidth, 16)

		headerCommits := "Commits"
		headerReviews := "Reviews"
		headerLOC := "LOC"
		headerMerged := "Merged"
		headerOpen := "Open"
		nameHeader := "Engineer"

		switch m.orgSortColumn {
		case SortByCommits:
			headerCommits = "Commits ▼"
		case SortByReviews:
			headerReviews = "Reviews ▼"
		case SortByLOC:
			headerLOC = "LOC ▼"
		case SortByMerged:
			headerMerged = "Merged ▼"
		case SortByOpen:
			headerOpen = "Open ▼"
		case SortByName:
			nameHeader = "Engineer ▼"
		}

		header := fmt.Sprintf("%s%-*s %9s %9s %8s %8s %8s", rowPrefix, nameWidth, nameHeader, headerCommits, headerReviews, headerLOC, headerMerged, headerOpen)
		b.WriteString(accentStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render(rowPrefix + strings.Repeat("─", tableWidth)))
		b.WriteString("\n")

		// Calculate visible rows
		headerLines := 4 // title + blank + header + separator
		footerLines := 4 // blank + summary + blank + help
		visibleRows := max(maxHeight-headerLines-footerLines, 3)

		// Scroll offset
		scrollOffset := 0
		if m.orgSelectedIndex >= visibleRows {
			scrollOffset = m.orgSelectedIndex - visibleRows + 1
		}

		endIdx := min(scrollOffset+visibleRows, len(m.orgMembers))

		for i := scrollOffset; i < endIdx; i++ {
			member := m.orgMembers[i]
			name := "@" + member.Login
			if len(name) > nameWidth {
				name = name[:nameWidth-1] + "…"
			}

			commits := member.Commits
			reviews := member.Reviews
			loc := member.Additions + member.Deletions
			merged := len(member.MergedPRs)
			open := len(member.OpenPRs)

			line := fmt.Sprintf("%-*s %9d %9d %8d %8d %8d", nameWidth, name, commits, reviews, loc, merged, open)

			if i == m.orgSelectedIndex {
				b.WriteString(selectedStyle.Render(selectedRowPrefix + line))
			} else {
				b.WriteString(normalStyle.Render(rowPrefix + line))
			}
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(m.orgMembers) > visibleRows {
			shown := fmt.Sprintf(" (%d-%d of %d)", scrollOffset+1, endIdx, len(m.orgMembers))
			b.WriteString(subtleStyle.Render(shown))
			b.WriteString("\n")
		}

		// Summary
		b.WriteString("\n")
		totalCommits, totalReviews, totalLOC := totalOrgStats(m.orgMembers)
		summary := fmt.Sprintf("%d engineers active  ·  %d commits  ·  %d reviews  ·  %d LOC  ·  %d PRs merged",
			len(m.orgMembers), totalCommits, totalReviews, totalLOC, totalMergedPRs(m.orgMembers))
		if m.orgLastLoadSummary.Duration > 0 {
			summary += fmt.Sprintf("  ·  loaded in %s", formatLoadDuration(m.orgLastLoadSummary.Duration))
		}
		b.WriteString(accentStyle.Render(summary))
		b.WriteString("\n\n")

		// Help
		b.WriteString(subtleStyle.Render("↑↓: navigate  ←→/s: sort  enter: details  r: refresh  esc: close"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder).
		Padding(1, 2).
		Width(maxWidth).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderOrgLoading(maxWidth int, accentStyle, subtleStyle lipgloss.Style) string {
	spinner := spinnerFrames[m.bannerFrame%len(spinnerFrames)]
	elapsed := time.Since(m.orgLoadStartedAt)
	if m.orgLoadStartedAt.IsZero() {
		elapsed = 0
	}

	completedSteps := 0
	currentStepProgress := 0.0
	for _, step := range orgLoadingSteps {
		progress, ok := m.orgLoadProgress[step]
		if !ok {
			break
		}
		if progress.Done {
			completedSteps++
			continue
		}
		if progress.Total > 0 {
			currentStepProgress = float64(progress.Current) / float64(progress.Total)
		}
		break
	}

	progressWidth := max(min(maxWidth-28, 28), 12)
	filled := int((float64(completedSteps) + currentStepProgress) / float64(len(orgLoadingSteps)) * float64(progressWidth))
	filled = max(min(filled, progressWidth), 0)
	progressBar := strings.Repeat("█", filled) + strings.Repeat("░", progressWidth-filled)

	var b strings.Builder
	b.WriteString(accentStyle.Render(fmt.Sprintf(" %s Building org activity snapshot", spinner)))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(" Pulling members, PRs, commits, reviews, and diff stats from GitHub"))
	b.WriteString("\n\n")
	b.WriteString(accentStyle.Render(fmt.Sprintf(" Progress [%s] %d/%d steps", progressBar, completedSteps, len(orgLoadingSteps))))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf(" Elapsed %s", formatLoadDuration(elapsed))))
	b.WriteString("\n\n")

	labelWidth := 12
	timeWidth := 8
	detailWidth := max(maxWidth-labelWidth-timeWidth-19, 10)

	for _, step := range orgLoadingSteps {
		progress, ok := m.orgLoadProgress[step]

		status := "○"
		detail := "Queued"
		timing := ""

		switch {
		case ok && progress.Done:
			status = "✓"
			detail = progress.Detail
			timing = formatLoadDuration(progress.UpdatedAt.Sub(progress.StartedAt))
		case ok:
			status = spinner
			detail = progress.Detail
			timing = formatLoadDuration(time.Since(progress.StartedAt))
		}

		detail = truncateOrgLoadingText(detail, detailWidth)
		line := fmt.Sprintf(" %s %-*s %-*s %*s", status, labelWidth, step.String(), detailWidth, detail, timeWidth, timing)
		if ok && !progress.Done {
			b.WriteString(accentStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func truncateOrgLoadingText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func formatLoadDuration(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Round(time.Second)/time.Second))
	}
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm %02ds", minutes, seconds)
}

// renderOrgInput renders the org name text input overlay.
func (m *Model) renderOrgInput() string {
	maxWidth := max(min(56, m.width-4), 30)

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Title).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(m.theme.Subtle)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Enter GitHub Organization"))
	b.WriteString("\n\n")
	b.WriteString(m.orgInput.View())
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("enter: confirm  esc: cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder).
		Padding(1, 2).
		Width(maxWidth).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
