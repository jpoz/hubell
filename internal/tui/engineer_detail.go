package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderEngineerDetail renders the engineer drill-down overlay.
func (m *Model) renderEngineerDetail() string {
	maxWidth := max(min(80, m.width-4), 40)
	maxHeight := max(m.height-4, 10)

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Title).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(m.theme.Subtle)
	selectedStyle := lipgloss.NewStyle().Foreground(m.theme.SelectedForeground).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.NormalForeground)
	successStyle := lipgloss.NewStyle().Foreground(m.theme.StatusSuccess)
	failureStyle := lipgloss.NewStyle().Foreground(m.theme.StatusFailure)

	var lines []string

	if m.engineerLoading {
		spinner := spinnerFrames[m.bannerFrame%len(spinnerFrames)]
		lines = append(lines, accentStyle.Render(fmt.Sprintf(" %s Loading engineer details...", spinner)))

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.theme.FocusedBorder).
			Padding(1, 2).
			Width(maxWidth).
			Render(strings.Join(lines, "\n"))

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	d := m.engineerDetail
	if d == nil {
		return ""
	}

	innerWidth := maxWidth - 6
	sep := subtleStyle.Render(strings.Repeat("─", innerWidth))

	// Title
	lines = append(lines, titleStyle.Render(fmt.Sprintf("@%s - Last 7 Days", d.Login)))
	lines = append(lines, "")

	// Merged PRs section
	lines = append(lines, accentStyle.Render(fmt.Sprintf("PRs Merged (%d)", len(d.MergedPRs))))
	lines = append(lines, sep)

	if len(d.MergedPRs) == 0 {
		lines = append(lines, subtleStyle.Render("  No merged PRs"))
	} else {
		for i, pr := range d.MergedPRs {
			repoID := fmt.Sprintf("%s/%s#%d", pr.Owner, pr.Repo, pr.Number)
			diffStr := ""
			if pr.Additions > 0 || pr.Deletions > 0 {
				diffStr = fmt.Sprintf("  %s %s",
					successStyle.Render(fmt.Sprintf("+%d", pr.Additions)),
					failureStyle.Render(fmt.Sprintf("-%d", pr.Deletions)))
			}

			line := repoID + diffStr
			if i == m.engineerSelectedPR {
				lines = append(lines, selectedStyle.Render("▸ "+line))
			} else {
				lines = append(lines, normalStyle.Render("  "+line))
			}

			// PR title on next line (indented)
			title := pr.Title
			maxTitleWidth := innerWidth - 6
			if len(title) > maxTitleWidth {
				title = title[:maxTitleWidth-1] + "…"
			}
			lines = append(lines, subtleStyle.Render("    "+title))
		}
	}
	lines = append(lines, "")

	// Reviews section
	lines = append(lines, accentStyle.Render(fmt.Sprintf("Reviews Given (%d)", len(d.ReviewedPRs))))
	lines = append(lines, sep)

	if len(d.ReviewedPRs) == 0 {
		lines = append(lines, subtleStyle.Render("  No reviews"))
	} else {
		maxReviews := min(len(d.ReviewedPRs), 10)
		for i := 0; i < maxReviews; i++ {
			pr := d.ReviewedPRs[i]
			line := fmt.Sprintf("  %s/%s#%d", pr.Owner, pr.Repo, pr.Number)
			if pr.Author != "" {
				line += subtleStyle.Render(fmt.Sprintf(" by @%s", pr.Author))
			}
			lines = append(lines, normalStyle.Render(line))
		}
		if len(d.ReviewedPRs) > maxReviews {
			lines = append(lines, subtleStyle.Render(fmt.Sprintf("  ... and %d more", len(d.ReviewedPRs)-maxReviews)))
		}
	}
	lines = append(lines, "")

	// Daily Activity chart
	lines = append(lines, accentStyle.Render("Daily Activity"))
	lines = append(lines, sep)

	// Find max for scaling (Mon-Fri only for bar chart, show all 7 days)
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	dayIndices := []int{1, 2, 3, 4, 5, 6, 0} // time.Weekday: 0=Sun, 1=Mon, ...

	maxActivity := 0
	for _, idx := range dayIndices {
		if d.DailyActivity[idx] > maxActivity {
			maxActivity = d.DailyActivity[idx]
		}
	}

	barMaxWidth := max(innerWidth-16, 10) // space for "  Mon ████  N"
	for i, dayIdx := range dayIndices {
		count := d.DailyActivity[dayIdx]
		barLen := 0
		if maxActivity > 0 {
			barLen = count * barMaxWidth / maxActivity
			if count > 0 && barLen == 0 {
				barLen = 1
			}
		}

		bar := strings.Repeat("█", barLen)
		dayStyle := normalStyle
		if i < 5 { // weekdays
			dayStyle = accentStyle
		}

		line := fmt.Sprintf("  %s %s  %d", dayNames[i], dayStyle.Render(bar), count)
		lines = append(lines, line)
	}
	lines = append(lines, "")

	// Stats section
	lines = append(lines, accentStyle.Render("Stats"))
	lines = append(lines, sep)

	// Avg PR Size
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Avg PR Size:        %s / %s",
		successStyle.Render(fmt.Sprintf("+%d", d.AvgAdditions)),
		failureStyle.Render(fmt.Sprintf("-%d", d.AvgDeletions)))))

	// Avg Time to Merge
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Avg Time to Merge:  %s",
		accentStyle.Render(formatMergeDuration(d.AvgTimeToMerge)))))

	// Repos Touched
	reposStr := strings.Join(d.ReposContributed, ", ")
	maxReposLen := innerWidth - 22
	if len(reposStr) > maxReposLen && maxReposLen > 3 {
		reposStr = reposStr[:maxReposLen-3] + "..."
	}
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Repos Touched:      %s",
		accentStyle.Render(reposStr))))

	// Longest PR
	if d.LongestPR != nil {
		lines = append(lines, normalStyle.Render(fmt.Sprintf("  Longest PR:         %s (%s)",
			accentStyle.Render(fmt.Sprintf("%s/%s#%d", d.LongestPR.Owner, d.LongestPR.Repo, d.LongestPR.Number)),
			formatMergeDuration(d.LongestPR.TimeToMerge))))
	}

	// Comments
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Comments Given:     %s",
		accentStyle.Render(fmt.Sprintf("%d", d.CommentsGiven)))))
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Comments Received:  %s",
		accentStyle.Render(fmt.Sprintf("%d", d.CommentsReceived)))))

	lines = append(lines, "")

	// Open PRs section
	if len(d.OpenPRs) > 0 {
		lines = append(lines, accentStyle.Render(fmt.Sprintf("Open PRs (%d)", len(d.OpenPRs))))
		lines = append(lines, sep)

		for _, pr := range d.OpenPRs {
			diffStr := ""
			if pr.Additions > 0 || pr.Deletions > 0 {
				diffStr = fmt.Sprintf("  %s %s",
					successStyle.Render(fmt.Sprintf("+%d", pr.Additions)),
					failureStyle.Render(fmt.Sprintf("-%d", pr.Deletions)))
			}
			ageStr := subtleStyle.Render(fmt.Sprintf("(%s old)", formatMergeDuration(pr.Age)))

			line := fmt.Sprintf("  %s/%s#%d%s %s", pr.Owner, pr.Repo, pr.Number, diffStr, ageStr)
			lines = append(lines, normalStyle.Render(line))
		}
		lines = append(lines, "")
	}

	// Help
	lines = append(lines, subtleStyle.Render("↑↓: select PR  enter: open in browser  esc: back"))

	// Apply scroll viewport
	contentHeight := max(maxHeight-4, 5) // account for box border + padding
	totalLines := len(lines)

	if m.engineerScroll > totalLines-contentHeight {
		m.engineerScroll = max(totalLines-contentHeight, 0)
	}
	if m.engineerScroll < 0 {
		m.engineerScroll = 0
	}

	startLine := m.engineerScroll
	endLine := min(startLine+contentHeight, totalLines)

	visibleContent := strings.Join(lines[startLine:endLine], "\n")

	// Add scroll indicator if content overflows
	if totalLines > contentHeight {
		scrollPct := 0
		if totalLines-contentHeight > 0 {
			scrollPct = m.engineerScroll * 100 / (totalLines - contentHeight)
		}
		scrollIndicator := subtleStyle.Render(fmt.Sprintf(" [%d%%]", scrollPct))
		visibleContent += "\n" + scrollIndicator
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder).
		Padding(1, 2).
		Width(maxWidth).
		Render(visibleContent)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// formatMergeDuration formats a duration in a human-readable way for merge times.
func formatMergeDuration(d time.Duration) string {
	if d <= 0 {
		return "N/A"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	remainHours := hours % 24
	return fmt.Sprintf("%dd %dh", days, remainHours)
}
