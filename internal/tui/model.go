package tui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/github"
	"github.com/jpoz/hubell/internal/notify"
)

// NotificationItem implements list.Item for the bubbles list
type NotificationItem struct {
	notification *github.Notification
}

// FilterValue implements list.Item
func (i NotificationItem) FilterValue() string {
	return i.notification.Subject.Title
}

// Title implements list.DefaultItem
func (i NotificationItem) Title() string {
	unreadIndicator := " "
	if i.notification.Unread {
		unreadIndicator = "•"
	}
	return fmt.Sprintf("%s [%s] %s",
		unreadIndicator,
		i.notification.Repository.FullName,
		i.notification.Subject.Title)
}

// Description implements list.DefaultItem
func (i NotificationItem) Description() string {
	elapsed := time.Since(i.notification.UpdatedAt)
	timeStr := formatDuration(elapsed)
	return fmt.Sprintf("%s | Updated: %s", i.notification.Reason, timeStr)
}

// FilterMode controls which notifications are displayed
type FilterMode int

const (
	// FilterMyPRs shows only PullRequest notifications where the user is author or commenter
	FilterMyPRs FilterMode = iota
	// FilterAll shows all notifications
	FilterAll
)

func (f FilterMode) String() string {
	switch f {
	case FilterMyPRs:
		return "My PRs"
	case FilterAll:
		return "All"
	default:
		return "Unknown"
	}
}

// Model is the main bubbletea model
type Model struct {
	list            list.Model
	githubClient    *github.Client
	pollCh          <-chan github.PollResult
	ctx             context.Context
	cancel          context.CancelFunc
	notifications   []*github.Notification
	allNotifications map[string]*github.Notification
	notificationMap map[string]*github.Notification
	lastNotifyCount int
	filterMode      FilterMode
	err             error
	width           int
	height          int
}

// New creates a new TUI model
func New(ctx context.Context, client *github.Client, pollCh <-chan github.PollResult) *Model {
	ctx, cancel := context.WithCancel(ctx)

	// Initialize list with default delegate
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "GitHub Notifications"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return &Model{
		list:             l,
		githubClient:     client,
		pollCh:           pollCh,
		ctx:              ctx,
		cancel:           cancel,
		allNotifications: make(map[string]*github.Notification),
		notificationMap:  make(map[string]*github.Notification),
		filterMode:       FilterMyPRs,
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		waitForPollResult(m.pollCh),
		tea.EnterAltScreen,
	)
}

// waitForPollResult waits for the next poll result
func waitForPollResult(pollCh <-chan github.PollResult) tea.Cmd {
	return func() tea.Msg {
		result := <-pollCh
		if result.Error != nil {
			return ErrorMsg{Err: result.Error}
		}
		if result.Notifications != nil {
			return NotificationsMsg{Notifications: result.Notifications}
		}
		// 304 Not Modified - wait for next poll
		return waitForPollResult(pollCh)()
	}
}

// mergeNotifications merges incoming notifications into the allNotifications map
func (m *Model) mergeNotifications(incoming []*github.Notification) {
	for _, n := range incoming {
		m.allNotifications[n.ID] = n
	}
}

// applyFilter returns notifications matching the current filter mode
func (m *Model) applyFilter() []*github.Notification {
	var filtered []*github.Notification
	for _, n := range m.allNotifications {
		if m.matchesFilter(n) {
			filtered = append(filtered, n)
		}
	}

	// Sort by UpdatedAt descending (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})

	return filtered
}

// matchesFilter returns true if a notification matches the current filter
func (m *Model) matchesFilter(n *github.Notification) bool {
	switch m.filterMode {
	case FilterMyPRs:
		if n.Subject.Type != "PullRequest" {
			return false
		}
		return n.Reason == "author" || n.Reason == "comment"
	case FilterAll:
		return true
	default:
		return true
	}
}

// updateNotifications merges new notifications and refreshes the display
func (m *Model) updateNotifications(incoming []*github.Notification) {
	if incoming != nil {
		m.mergeNotifications(incoming)
	}

	// Apply filter
	m.notifications = m.applyFilter()

	// Update notification map for quick lookups
	m.notificationMap = make(map[string]*github.Notification)
	for _, n := range m.notifications {
		m.notificationMap[n.ID] = n
	}

	// Convert to list items
	items := make([]list.Item, len(m.notifications))
	for i, n := range m.notifications {
		items[i] = NotificationItem{notification: n}
	}
	m.list.SetItems(items)

	// Send desktop notification if unread count increased
	unreadCount := 0
	for _, n := range m.notifications {
		if n.Unread {
			unreadCount++
		}
	}

	if unreadCount > m.lastNotifyCount {
		newCount := unreadCount - m.lastNotifyCount
		notify.SendDesktopNotification(
			"GitHub Notifications",
			fmt.Sprintf("You have %d new notification(s)", newCount),
		)
	}
	m.lastNotifyCount = unreadCount
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}
