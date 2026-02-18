package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jpoz/hubell/internal/github"
)

const maxCheckDots = 10

// PRDelegate is a custom list.ItemDelegate that renders PR items with
// individually colored CI badges, review badges, check dots, and diff stats.
type PRDelegate struct {
	theme Theme
}

func newPRDelegate(t Theme) PRDelegate {
	return PRDelegate{theme: t}
}

func (d PRDelegate) Height() int                             { return 2 }
func (d PRDelegate) Spacing() int                            { return 1 }
func (d PRDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d PRDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	prItem, ok := item.(PRItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := m.Width()

	// Build title line from individually-styled segments
	var segments []string

	// Repo identifier
	repoID := fmt.Sprintf("%s/%s#%d", prItem.info.Owner, prItem.info.Repo, prItem.info.Number)
	repoColor := d.theme.NormalForeground
	if selected {
		repoColor = d.theme.SelectedForeground
	}
	segments = append(segments, lipgloss.NewStyle().Foreground(repoColor).Render(repoID))

	// CI badge
	switch prItem.status {
	case github.PRStatusSuccess:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusSuccess).Bold(true).Render("  ✓"))
	case github.PRStatusFailure:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusFailure).Bold(true).Render("  ✗"))
	case github.PRStatusPending:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusPending).Bold(true).Render("  ⋯"))
	}

	// Review badge
	switch prItem.info.ReviewState {
	case github.PRReviewApproved:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusSuccess).Render("  Approved"))
	case github.PRReviewChangesRequested:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusFailure).Render("  Changes Requested"))
	case github.PRReviewReviewed:
		segments = append(segments, lipgloss.NewStyle().Foreground(d.theme.StatusPending).Render("  Reviewed"))
	}

	// Check dots (one per check run, colored by result)
	// Sort: pending first, then failed, then successful so the most
	// important statuses are visible when truncated.
	if len(prItem.info.CheckRuns) > 0 {
		sorted := make([]github.CheckRun, len(prItem.info.CheckRuns))
		copy(sorted, prItem.info.CheckRuns)
		sort.Slice(sorted, func(i, j int) bool {
			return checkRunSortKey(sorted[i]) < checkRunSortKey(sorted[j])
		})

		var dots strings.Builder
		dots.WriteString("  ")
		shown := len(sorted)
		overflow := 0
		if shown > maxCheckDots {
			overflow = shown - maxCheckDots
			shown = maxCheckDots
		}
		for i := 0; i < shown; i++ {
			cr := sorted[i]
			var color lipgloss.Color
			var dot string
			switch {
			case cr.Status == "queued" || cr.Status == "in_progress":
				color = d.theme.StatusPending
				dot = "○"
			case cr.Conclusion == "success":
				color = d.theme.StatusSuccess
				dot = "●"
			case cr.Conclusion == "failure" || cr.Conclusion == "cancelled" || cr.Conclusion == "timed_out":
				color = d.theme.StatusFailure
				dot = "●"
			default:
				color = d.theme.Subtle
				dot = "●"
			}
			dots.WriteString(lipgloss.NewStyle().Foreground(color).Render(dot))
		}
		if overflow > 0 {
			dots.WriteString(lipgloss.NewStyle().Foreground(d.theme.Subtle).Render(fmt.Sprintf("+%d", overflow)))
		}
		segments = append(segments, dots.String())
	}

	// Diff stats
	if prItem.info.Additions > 0 || prItem.info.Deletions > 0 {
		var stats strings.Builder
		stats.WriteString("  ")
		if prItem.info.Additions > 0 {
			stats.WriteString(lipgloss.NewStyle().Foreground(d.theme.StatusSuccess).Render(fmt.Sprintf("+%d", prItem.info.Additions)))
		}
		if prItem.info.Deletions > 0 {
			if prItem.info.Additions > 0 {
				stats.WriteString(" ")
			}
			stats.WriteString(lipgloss.NewStyle().Foreground(d.theme.StatusFailure).Render(fmt.Sprintf("-%d", prItem.info.Deletions)))
		}
		segments = append(segments, stats.String())
	}

	titleLine := strings.Join(segments, "")

	// Description line (branch name + PR title)
	descColor := d.theme.NormalDesc
	if selected {
		descColor = d.theme.SelectedDesc
	}
	var descParts []string
	if prItem.info.Branch != "" {
		descParts = append(descParts, lipgloss.NewStyle().Foreground(d.theme.Subtle).Render(prItem.info.Branch))
	}
	descParts = append(descParts, lipgloss.NewStyle().Foreground(descColor).Render(prItem.info.Title))
	descLine := strings.Join(descParts, " ")

	// Truncate lines to fit available width (account for padding/border)
	contentWidth := width - 4
	if contentWidth < 0 {
		contentWidth = 0
	}
	titleLine = ansi.Truncate(titleLine, contentWidth, "…")
	descLine = ansi.Truncate(descLine, contentWidth, "…")

	// Apply wrapper styling (padding/border only, no foreground override)
	var rendered string
	if selected {
		wrapper := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(d.theme.Accent).
			PaddingLeft(1)
		rendered = wrapper.Render(titleLine + "\n" + descLine)
	} else {
		wrapper := lipgloss.NewStyle().PaddingLeft(2)
		rendered = wrapper.Render(titleLine + "\n" + descLine)
	}

	fmt.Fprint(w, rendered)
}

// checkRunSortKey returns a sort priority for a check run:
// 0 = pending/in-progress, 1 = failed/cancelled/timed_out, 2 = success, 3 = other.
func checkRunSortKey(cr github.CheckRun) int {
	switch {
	case cr.Status == "queued" || cr.Status == "in_progress":
		return 0
	case cr.Conclusion == "failure" || cr.Conclusion == "cancelled" || cr.Conclusion == "timed_out":
		return 1
	case cr.Conclusion == "success":
		return 2
	default:
		return 3
	}
}
