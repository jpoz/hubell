package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jpoz/hubell/internal/github"
)

// OrgSortColumn identifies which column the org table is sorted by.
type OrgSortColumn int

const (
	SortByMerged OrgSortColumn = iota
	SortByOpen
	SortByName
)

func (s OrgSortColumn) String() string {
	switch s {
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
		case SortByMerged:
			return len(m.orgMembers[i].MergedPRs) > len(m.orgMembers[j].MergedPRs)
		case SortByOpen:
			return len(m.orgMembers[i].OpenPRs) > len(m.orgMembers[j].OpenPRs)
		case SortByName:
			return m.orgMembers[i].Login < m.orgMembers[j].Login
		default:
			return len(m.orgMembers[i].MergedPRs) > len(m.orgMembers[j].MergedPRs)
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

// renderOrgDashboard renders the org activity overlay.
func (m *Model) renderOrgDashboard() string {
	if m.orgInputActive {
		return m.renderOrgInput()
	}

	maxWidth := max(min(76, m.width-4), 40)
	maxHeight := max(m.height-6, 10)

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
		spinner := spinnerFrames[m.bannerFrame%len(spinnerFrames)]
		b.WriteString(accentStyle.Render(fmt.Sprintf(" %s Loading org activity...", spinner)))
		b.WriteString("\n")
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
		nameWidth := max(innerWidth-20, 16)

		headerMerged := "Merged"
		headerOpen := "Open"
		if m.orgSortColumn == SortByMerged {
			headerMerged = "Merged ▼"
		} else if m.orgSortColumn == SortByOpen {
			headerOpen = "Open ▼"
		}

		nameHeader := "Engineer"
		if m.orgSortColumn == SortByName {
			nameHeader = "Engineer ▼"
		}

		header := fmt.Sprintf("  %-*s %8s %8s", nameWidth, nameHeader, headerMerged, headerOpen)
		b.WriteString(accentStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("  " + strings.Repeat("─", innerWidth)))
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

			merged := len(member.MergedPRs)
			open := len(member.OpenPRs)

			line := fmt.Sprintf("%-*s %8d %8d", nameWidth, name, merged, open)

			if i == m.orgSelectedIndex {
				b.WriteString(selectedStyle.Render("▸ " + line))
			} else {
				b.WriteString(normalStyle.Render("  " + line))
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
		summary := fmt.Sprintf("%d engineers active  ·  %d PRs merged this week",
			len(m.orgMembers), totalMergedPRs(m.orgMembers))
		b.WriteString(accentStyle.Render(summary))
		b.WriteString("\n\n")

		// Help
		b.WriteString(subtleStyle.Render("↑↓: navigate  enter: details  s: sort  r: refresh  esc: close"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder).
		Padding(1, 2).
		Width(maxWidth).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
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
