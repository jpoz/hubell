package tui

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/github"
	"github.com/jpoz/hubell/internal/notify"
)

//go:embed banner.txt
var bannerText string

// NotificationItem implements list.Item for the bubbles list
type NotificationItem struct {
	notification *github.Notification
	ciStatus     github.PRStatus
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

	ciIndicator := ""
	switch i.ciStatus {
	case github.PRStatusSuccess:
		ciIndicator = " [OK]"
	case github.PRStatusFailure:
		ciIndicator = " [FAIL]"
	case github.PRStatusPending:
		ciIndicator = " [...]"
	}

	return fmt.Sprintf("%s [%s] %s%s",
		unreadIndicator,
		i.notification.Repository.FullName,
		i.notification.Subject.Title,
		ciIndicator)
}

// Description implements list.DefaultItem
func (i NotificationItem) Description() string {
	elapsed := time.Since(i.notification.UpdatedAt)
	timeStr := formatDuration(elapsed)
	return fmt.Sprintf("%s | Updated: %s", i.notification.Reason, timeStr)
}

// PRItem implements list.Item for the PR list pane
type PRItem struct {
	info   github.PRInfo
	status github.PRStatus
}

// FilterValue implements list.Item
func (i PRItem) FilterValue() string {
	return i.info.Title
}

// Title implements list.DefaultItem
func (i PRItem) Title() string {
	statusIndicator := ""
	switch i.status {
	case github.PRStatusSuccess:
		statusIndicator = " [OK]"
	case github.PRStatusFailure:
		statusIndicator = " [FAIL]"
	case github.PRStatusPending:
		statusIndicator = " [...]"
	}

	reviewIndicator := ""
	switch i.info.ReviewState {
	case github.PRReviewApproved:
		reviewIndicator = " [Approved]"
	case github.PRReviewChangesRequested:
		reviewIndicator = " [Changes Requested]"
	case github.PRReviewReviewed:
		reviewIndicator = " [Reviewed]"
	}

	return fmt.Sprintf("%s/%s#%d%s%s", i.info.Owner, i.info.Repo, i.info.Number, statusIndicator, reviewIndicator)
}

// Description implements list.DefaultItem
func (i PRItem) Description() string {
	return i.info.Title
}

// Pane identifies which pane has keyboard focus
type Pane int

const (
	LeftPane Pane = iota
	RightPane
)

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
	list             list.Model
	prList           list.Model
	githubClient     *github.Client
	pollCh           <-chan github.PollResult
	ctx              context.Context
	cancel           context.CancelFunc
	notifications    []*github.Notification
	allNotifications map[string]*github.Notification
	notificationMap  map[string]*github.Notification
	prStatuses       map[string]github.PRStatus
	prInfos          map[string]github.PRInfo
	lastNotifyCount  int
	filterMode       FilterMode
	focusedPane      Pane
	loading          bool
	bannerFrame      int
	err              error
	width            int
	height           int
}

// New creates a new TUI model
func New(ctx context.Context, client *github.Client, pollCh <-chan github.PollResult) *Model {
	ctx, cancel := context.WithCancel(ctx)

	// Initialize notification list with default delegate
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Notifications"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Initialize PR list
	prDelegate := list.NewDefaultDelegate()
	pl := list.New([]list.Item{}, prDelegate, 0, 0)
	pl.Title = "Open PRs"
	pl.SetShowStatusBar(false)
	pl.SetFilteringEnabled(true)

	return &Model{
		list:             l,
		prList:           pl,
		githubClient:     client,
		pollCh:           pollCh,
		ctx:              ctx,
		cancel:           cancel,
		allNotifications: make(map[string]*github.Notification),
		notificationMap:  make(map[string]*github.Notification),
		prStatuses:       make(map[string]github.PRStatus),
		prInfos:          make(map[string]github.PRInfo),
		filterMode:       FilterMyPRs,
		focusedPane:      LeftPane,
		loading:          true,
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		waitForPollResult(m.pollCh),
		tea.EnterAltScreen,
		bannerTick(),
	)
}

// bannerTick returns a command that sends a BannerTickMsg after a short delay
func bannerTick() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return BannerTickMsg{}
	})
}

// waitForPollResult waits for the next poll result
func waitForPollResult(pollCh <-chan github.PollResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-pollCh
		if !ok {
			return nil
		}
		if result.Error != nil {
			return ErrorMsg{Err: result.Error}
		}
		if result.Notifications == nil && result.PRStatuses == nil {
			return waitForPollResult(pollCh)()
		}
		return PollResultMsg{
			Notifications: result.Notifications,
			PRStatuses:    result.PRStatuses,
			PRInfos:       result.PRInfos,
			PRChanges:     result.PRChanges,
		}
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

	// Convert to list items with CI status
	items := make([]list.Item, len(m.notifications))
	for i, n := range m.notifications {
		items[i] = NotificationItem{
			notification: n,
			ciStatus:     m.prStatusForNotification(n),
		}
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

// updatePRList rebuilds the right-pane PR list from current prInfos and prStatuses
func (m *Model) updatePRList() {
	// Collect and sort keys for stable ordering
	keys := make([]string, 0, len(m.prInfos))
	for k := range m.prInfos {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]list.Item, 0, len(keys))
	for _, key := range keys {
		items = append(items, PRItem{
			info:   m.prInfos[key],
			status: m.prStatuses[key],
		})
	}
	m.prList.SetItems(items)
}

// prAPIURLPattern matches GitHub API PR URLs like
// https://api.github.com/repos/{owner}/{repo}/pulls/{number}
var prAPIURLPattern = regexp.MustCompile(`/repos/([^/]+)/([^/]+)/pulls/(\d+)$`)

// prStatusForNotification looks up the CI status for a notification's PR
func (m *Model) prStatusForNotification(n *github.Notification) github.PRStatus {
	if n.Subject.Type != "PullRequest" || n.Subject.URL == "" {
		return ""
	}

	matches := prAPIURLPattern.FindStringSubmatch(n.Subject.URL)
	if matches == nil {
		return ""
	}

	owner := matches[1]
	repo := matches[2]
	number, err := strconv.Atoi(matches[3])
	if err != nil {
		return ""
	}

	key := github.PRKey(owner, repo, number)
	return m.prStatuses[key]
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
