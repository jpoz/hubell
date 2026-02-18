package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jpoz/hubell/internal/config"
	"github.com/jpoz/hubell/internal/github"
)

// DashboardStats accumulates session-scoped metrics for the activity dashboard.
type DashboardStats struct {
	MergedPRs              []github.MergedPRInfo
	WeeklyMergedCounts     map[string]int // keyed by ISO week (e.g. "2026-W07")
	ReviewLatencies        map[string]time.Duration // keyed by PR key
	ChecksTotal            int
	ChecksSuccess          int
	ChecksFailure          int
	NotificationTimestamps []time.Time
}

func newDashboardStats() DashboardStats {
	return DashboardStats{
		WeeklyMergedCounts: make(map[string]int),
		ReviewLatencies:    make(map[string]time.Duration),
	}
}

// updateFromPollResult refreshes dashboard data from the latest poll cycle.
func (d *DashboardStats) updateFromPollResult(mergedPRs []github.MergedPRInfo, weeklyMergedCounts map[string]int, prInfos map[string]github.PRInfo) {
	// Merge backfill counts (first poll only)
	if weeklyMergedCounts != nil {
		for k, v := range weeklyMergedCounts {
			d.WeeklyMergedCounts[k] = v
		}
	}

	if mergedPRs != nil {
		d.MergedPRs = mergedPRs

		// Update current week count and persist
		weekKey := config.WeekKey(time.Now())
		d.WeeklyMergedCounts[weekKey] = len(mergedPRs)
	}

	// Persist updated counts
	if weeklyMergedCounts != nil || mergedPRs != nil {
		stats := config.WeeklyStats{Weeks: d.WeeklyMergedCounts}
		_ = config.SaveWeeklyStats(stats)
	}

	// Recompute CI tallies from open PR check runs
	d.ChecksTotal = 0
	d.ChecksSuccess = 0
	d.ChecksFailure = 0
	for _, info := range prInfos {
		for _, cr := range info.CheckRuns {
			if cr.Status != "completed" {
				continue
			}
			d.ChecksTotal++
			switch cr.Conclusion {
			case "success":
				d.ChecksSuccess++
			case "failure", "cancelled", "timed_out":
				d.ChecksFailure++
			}
		}
	}

	// Compute review latencies: earliest non-author review per PR
	d.ReviewLatencies = make(map[string]time.Duration)
	for key, info := range prInfos {
		var earliest time.Time
		for _, r := range info.Reviews {
			if r.SubmittedAt.IsZero() {
				continue
			}
			if earliest.IsZero() || r.SubmittedAt.Before(earliest) {
				earliest = r.SubmittedAt
			}
		}
		if !earliest.IsZero() {
			d.ReviewLatencies[key] = earliest.Sub(info.CreatedAt)
		}
	}
}

// recordNotifications appends current timestamps for notification volume tracking.
func (d *DashboardStats) recordNotifications(count int) {
	now := time.Now()
	for range count {
		d.NotificationTimestamps = append(d.NotificationTimestamps, now)
	}
}

// averageReviewLatency returns the mean review latency across all tracked PRs.
func (d *DashboardStats) averageReviewLatency() time.Duration {
	if len(d.ReviewLatencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, lat := range d.ReviewLatencies {
		total += lat
	}
	return total / time.Duration(len(d.ReviewLatencies))
}

// ciPassRate returns the CI pass rate as a fraction (0.0–1.0).
func (d *DashboardStats) ciPassRate() float64 {
	if d.ChecksTotal == 0 {
		return 0
	}
	return float64(d.ChecksSuccess) / float64(d.ChecksTotal)
}

// notificationBuckets returns notification counts bucketed by age.
func (d *DashboardStats) notificationBuckets() (lastHour, oneToThree, threeToSix, sixPlus int) {
	now := time.Now()
	for _, ts := range d.NotificationTimestamps {
		age := now.Sub(ts)
		switch {
		case age < time.Hour:
			lastHour++
		case age < 3*time.Hour:
			oneToThree++
		case age < 6*time.Hour:
			threeToSix++
		default:
			sixPlus++
		}
	}
	return
}

// buildWeeklyChartData returns bar chart data for the last numWeeks weeks.
func (d *DashboardStats) buildWeeklyChartData(numWeeks int) []BarChartData {
	now := time.Now()
	data := make([]BarChartData, numWeeks)
	for i := range numWeeks {
		// Walk backwards: index 0 = oldest, last = current week
		t := now.AddDate(0, 0, -(numWeeks-1-i)*7)
		key := config.WeekKey(t)
		_, week := t.ISOWeek()
		data[i] = BarChartData{
			Label: fmt.Sprintf("W%d", week),
			Value: d.WeeklyMergedCounts[key],
		}
	}
	return data
}

// renderDashboard draws the activity dashboard overlay.
func (m *Model) renderDashboard() string {
	d := &m.dashboardStats

	maxWidth := max(min(72, m.width-4), 30)

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Title).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(m.theme.Subtle)
	successStyle := lipgloss.NewStyle().Foreground(m.theme.StatusSuccess)
	failureStyle := lipgloss.NewStyle().Foreground(m.theme.StatusFailure)

	sep := subtleStyle.Render(strings.Repeat("─", maxWidth-4))

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Activity Dashboard"))
	b.WriteString("\n\n")

	// Merged PRs bar chart (last 12 weeks)
	b.WriteString(accentStyle.Render("PRs Merged Per Week"))
	b.WriteString("\n")
	b.WriteString(sep)
	b.WriteString("\n")

	chartData := d.buildWeeklyChartData(12)
	chart := renderBarChart(chartData, maxWidth-4, 10, m.theme.Accent, m.theme.Subtle, m.theme.StatusSuccess)
	b.WriteString(chart)
	b.WriteString("\n\n")

	// Review latency + CI pass rate
	avgReview := d.averageReviewLatency()
	var reviewStr string
	if avgReview == 0 {
		reviewStr = "N/A"
	} else {
		reviewStr = formatReviewDuration(avgReview)
	}

	rate := d.ciPassRate()
	var ciStr string
	if d.ChecksTotal == 0 {
		ciStr = "N/A"
	} else {
		pct := int(rate * 100)
		ciStr = fmt.Sprintf("%d%% (%d/%d)", pct, d.ChecksSuccess, d.ChecksTotal)
	}

	b.WriteString(fmt.Sprintf("Avg Time to Review: %s",
		accentStyle.Render(reviewStr)))
	padding := max(maxWidth-24-len(reviewStr)-16-len(ciStr), 4)
	b.WriteString(strings.Repeat(" ", padding))

	ciLabel := "CI Pass Rate: "
	if d.ChecksTotal > 0 && rate >= 0.8 {
		b.WriteString(ciLabel + successStyle.Render(ciStr))
	} else if d.ChecksTotal > 0 {
		b.WriteString(ciLabel + failureStyle.Render(ciStr))
	} else {
		b.WriteString(ciLabel + subtleStyle.Render(ciStr))
	}
	b.WriteString("\n\n")

	// Notification volume
	total := len(d.NotificationTimestamps)
	b.WriteString(accentStyle.Render(fmt.Sprintf("Notifications This Session: %d", total)))
	b.WriteString("\n")
	b.WriteString(sep)
	b.WriteString("\n")

	lastHour, oneToThree, threeToSix, sixPlus := d.notificationBuckets()
	b.WriteString(fmt.Sprintf("  Last hour: %s  |  1-3h: %s  |  3-6h: %s  |  6h+: %s",
		accentStyle.Render(fmt.Sprintf("%d", lastHour)),
		accentStyle.Render(fmt.Sprintf("%d", oneToThree)),
		accentStyle.Render(fmt.Sprintf("%d", threeToSix)),
		accentStyle.Render(fmt.Sprintf("%d", sixPlus)),
	))
	b.WriteString("\n\n")

	b.WriteString(subtleStyle.Render("esc to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.FocusedBorder).
		Padding(1, 2).
		Width(maxWidth).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// formatReviewDuration formats a review latency duration in a human-readable way.
func formatReviewDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
