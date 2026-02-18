package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TimelineDelegate is a custom list.ItemDelegate that renders timeline events
// with colored icons per event type.
type TimelineDelegate struct {
	theme Theme
}

func newTimelineDelegate(t Theme) TimelineDelegate {
	return TimelineDelegate{theme: t}
}

func (d TimelineDelegate) Height() int                             { return 2 }
func (d TimelineDelegate) Spacing() int                            { return 1 }
func (d TimelineDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d TimelineDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	evt, ok := item.(TimelineEvent)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := m.Width()

	// Pick icon and color based on event type
	var icon, label string
	var iconColor lipgloss.Color
	switch evt.EventType {
	case TimelineEventCreated:
		icon = "+"
		label = "created"
		iconColor = d.theme.TimelineCreated
	case TimelineEventApproved:
		icon = "✓"
		label = "approved"
		iconColor = d.theme.TimelineApproved
	case TimelineEventMerged:
		icon = "⊕"
		label = "merged"
		iconColor = d.theme.TimelineMerged
	}

	// Line 1: icon + "merged 2h ago owner/repo#number"
	timeStr := formatDuration(time.Since(evt.Timestamp))
	repoRef := fmt.Sprintf("%s/%s#%d", evt.Owner, evt.Repo, evt.Number)

	iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)
	labelTimeStr := lipgloss.NewStyle().Foreground(iconColor).Render(fmt.Sprintf("%s %s", label, timeStr))

	repoColor := d.theme.NormalForeground
	if selected {
		repoColor = d.theme.SelectedForeground
	}
	repoStr := lipgloss.NewStyle().Foreground(repoColor).Render(repoRef)

	titleLine := fmt.Sprintf("%s %s %s", iconStr, labelTimeStr, repoStr)

	// Line 2: actor + PR title
	descColor := d.theme.NormalDesc
	if selected {
		descColor = d.theme.SelectedDesc
	}

	descText := evt.Title
	if evt.Actor != "" {
		descText = fmt.Sprintf("@%s · %s", evt.Actor, evt.Title)
	}
	descLine := lipgloss.NewStyle().Foreground(descColor).Render(descText)

	// Truncate to fit
	contentWidth := width - 4
	if contentWidth < 0 {
		contentWidth = 0
	}
	titleLine = ansi.Truncate(titleLine, contentWidth, "…")
	descLine = ansi.Truncate(descLine, contentWidth, "…")

	// Wrapper styling
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
