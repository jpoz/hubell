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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/config"
	"github.com/jpoz/hubell/internal/github"
	"github.com/jpoz/hubell/internal/notify"
)

//go:embed banner.txt
var bannerText string

// NotificationItem implements list.Item for the bubbles list
type NotificationItem struct {
	notification  *github.Notification
	ciStatus      github.PRStatus
	commentDetail *github.CommentDetail
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
	timeStr := formatDuration(time.Since(i.notification.UpdatedAt))

	d := i.commentDetail
	if d == nil {
		return fmt.Sprintf("%s · %s", formatReason(i.notification.Reason), timeStr)
	}

	switch d.Type {
	case "review":
		switch d.ReviewState {
		case "APPROVED":
			return fmt.Sprintf("@%s approved · %s", d.Author, timeStr)
		case "CHANGES_REQUESTED":
			return fmt.Sprintf("@%s requested changes · %s", d.Author, timeStr)
		case "COMMENTED":
			return fmt.Sprintf("@%s reviewed · %s", d.Author, timeStr)
		default:
			if d.Author != "" {
				return fmt.Sprintf("@%s reviewed · %s", d.Author, timeStr)
			}
		}
	case "comment", "review_comment":
		if d.Author != "" && d.Body != "" {
			return fmt.Sprintf("@%s: \"%s\" · %s", d.Author, d.Body, timeStr)
		}
		if d.Author != "" {
			return fmt.Sprintf("@%s commented · %s", d.Author, timeStr)
		}
	}

	return fmt.Sprintf("%s · %s", formatReason(i.notification.Reason), timeStr)
}

// formatReason maps raw notification reason strings to human-readable labels.
func formatReason(reason string) string {
	switch reason {
	case "assign":
		return "You were assigned"
	case "author":
		return "Activity on your PR"
	case "comment":
		return "New comment"
	case "ci_activity":
		return "CI activity"
	case "invitation":
		return "Repo invitation"
	case "manual":
		return "Subscribed"
	case "mention":
		return "You were mentioned"
	case "review_requested":
		return "Review requested"
	case "security_alert":
		return "Security alert"
	case "state_change":
		return "State changed"
	case "subscribed":
		return "Watching"
	case "team_mention":
		return "Team mentioned"
	default:
		return reason
	}
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

// Title implements list.DefaultItem (used as FilterValue fallback)
func (i PRItem) Title() string {
	return fmt.Sprintf("%s/%s#%d %s", i.info.Owner, i.info.Repo, i.info.Number, i.info.Title)
}

// Description implements list.DefaultItem
func (i PRItem) Description() string {
	return i.info.Title
}

// TimelineEventType identifies the kind of timeline event.
type TimelineEventType int

const (
	TimelineEventCreated TimelineEventType = iota
	TimelineEventApproved
	TimelineEventMerged
)

// TimelineEvent represents a single chronological event for the timeline pane.
type TimelineEvent struct {
	EventType TimelineEventType
	Timestamp time.Time
	Owner     string
	Repo      string
	Number    int
	Title     string
	URL       string
	Actor     string
}

// FilterValue implements list.Item.
func (e TimelineEvent) FilterValue() string {
	return e.Title
}

// Pane identifies which pane has keyboard focus
type Pane int

const (
	TimelinePane Pane = iota
	LeftPane
	RightPane
	paneCount // used for modular tab cycling
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
	timelineList     list.Model
	githubClient     *github.Client
	pollCh           <-chan github.PollResult
	ctx              context.Context
	cancel           context.CancelFunc
	notifications    []*github.Notification
	allNotifications map[string]*github.Notification
	notificationMap  map[string]*github.Notification
	prStatuses       map[string]github.PRStatus
	prInfos          map[string]github.PRInfo
	commentDetails   map[string]*github.CommentDetail
	lastNotifyCount  int
	filterMode       FilterMode
	focusedPane      Pane
	loading      bool
	loadingSteps map[github.LoadingStep]bool
	prProgress   github.LoadingProgress
	progressCh   <-chan github.LoadingProgress
	bannerFrame  int
	err          error
	width        int
	height       int

	theme             Theme
	showThemeSelector bool
	themeList         list.Model

	showDashboard  bool
	dashboardStats DashboardStats

	// Org activity overlay
	showOrgDashboard   bool
	orgName            string
	orgMembers         []github.OrgMemberActivity
	orgSelectedIndex   int
	orgSortColumn      OrgSortColumn
	orgLoading         bool
	orgError           error
	orgInput           textinput.Model
	orgInputActive     bool
	showEngineerDetail bool
	engineerDetail     *github.EngineerDetail
	engineerLoading    bool
	engineerSelectedPR int
	engineerScroll     int
}

// New creates a new TUI model
func New(ctx context.Context, client *github.Client, pollCh <-chan github.PollResult, progressCh <-chan github.LoadingProgress, orgName string) *Model {
	ctx, cancel := context.WithCancel(ctx)

	theme := GetTheme(config.LoadTheme())

	// Initialize notification list with themed delegate
	delegate := newThemedDelegate(theme)
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Notifications"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	applyListTheme(&l, theme)

	// Initialize PR list with custom delegate for colored rendering
	prDelegate := newPRDelegate(theme)
	pl := list.New([]list.Item{}, prDelegate, 0, 0)
	pl.Title = "Open PRs"
	pl.SetShowStatusBar(false)
	pl.SetFilteringEnabled(true)
	applyListTheme(&pl, theme)

	// Initialize timeline list with custom delegate
	tlDelegate := newTimelineDelegate(theme)
	tl := list.New([]list.Item{}, tlDelegate, 0, 0)
	tl.Title = "Timeline"
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(true)
	applyListTheme(&tl, theme)

	dashStats := newDashboardStats()
	cached := config.LoadWeeklyStats()
	for k, v := range cached.Weeks {
		dashStats.WeeklyMergedCounts[k] = v
	}

	ti := textinput.New()
	ti.Placeholder = "organization name (e.g. angellist)"
	ti.CharLimit = 100
	ti.Width = 40

	return &Model{
		list:             l,
		prList:           pl,
		timelineList:     tl,
		githubClient:     client,
		pollCh:           pollCh,
		progressCh:       progressCh,
		ctx:              ctx,
		cancel:           cancel,
		allNotifications: make(map[string]*github.Notification),
		notificationMap:  make(map[string]*github.Notification),
		prStatuses:       make(map[string]github.PRStatus),
		prInfos:          make(map[string]github.PRInfo),
		commentDetails:   make(map[string]*github.CommentDetail),
		filterMode:       FilterMyPRs,
		focusedPane:      TimelinePane,
		loading:          true,
		loadingSteps:     make(map[github.LoadingStep]bool),
		theme:            theme,
		themeList:        buildThemeList(),
		dashboardStats:   dashStats,
		orgName:          orgName,
		orgInput:         ti,
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		waitForPollResult(m.pollCh),
		waitForLoadingStep(m.progressCh),
		tea.EnterAltScreen,
		bannerTick(),
	}
	// Auto-fetch org data for the timeline when an org is configured
	if m.orgName != "" {
		m.orgLoading = true
		cmds = append(cmds, fetchOrgData(m.ctx, m.githubClient, m.orgName))
	}
	return tea.Batch(cmds...)
}

// waitForLoadingStep reads the next loading progress update from the progress channel
func waitForLoadingStep(ch <-chan github.LoadingProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return LoadingProgressMsg{p}
	}
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
			Notifications:      result.Notifications,
			PRStatuses:         result.PRStatuses,
			PRInfos:            result.PRInfos,
			PRChanges:          result.PRChanges,
			MergedPRs:          result.MergedPRs,
			WeeklyMergedCounts: result.WeeklyMergedCounts,
			CommentDetails:     result.CommentDetails,
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

	// Convert to list items with CI status and comment detail
	items := make([]list.Item, len(m.notifications))
	for i, n := range m.notifications {
		items[i] = NotificationItem{
			notification:  n,
			ciStatus:      m.prStatusForNotification(n),
			commentDetail: m.commentDetails[n.ID],
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
		m.dashboardStats.recordNotifications(newCount)
		notify.SendDesktopNotification(
			"GitHub Notifications",
			fmt.Sprintf("You have %d new notification(s)", newCount),
		)
	}
	m.lastNotifyCount = unreadCount
}

// updatePRList rebuilds the right-pane PR list from current prInfos and prStatuses
func (m *Model) updatePRList() {
	// Collect PRItems and sort by CreatedAt descending (newest first)
	items := make([]list.Item, 0, len(m.prInfos))
	for key := range m.prInfos {
		items = append(items, PRItem{
			info:   m.prInfos[key],
			status: m.prStatuses[key],
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].(PRItem).info.CreatedAt.After(items[j].(PRItem).info.CreatedAt)
	})
	m.prList.SetItems(items)
}

// buildTimelineEvents derives timeline events from org-wide data when
// available, falling back to the authenticated user's data otherwise.
func (m *Model) buildTimelineEvents() []TimelineEvent {
	var events []TimelineEvent

	if len(m.orgMembers) > 0 {
		// Org-wide timeline: created + merged from all members
		for _, member := range m.orgMembers {
			for _, pr := range member.OpenPRs {
				events = append(events, TimelineEvent{
					EventType: TimelineEventCreated,
					Timestamp: pr.CreatedAt,
					Owner:     pr.Owner,
					Repo:      pr.Repo,
					Number:    pr.Number,
					Title:     pr.Title,
					URL:       pr.URL,
					Actor:     member.Login,
				})
			}
			for _, pr := range member.MergedPRs {
				events = append(events, TimelineEvent{
					EventType: TimelineEventMerged,
					Timestamp: pr.MergedAt,
					Owner:     pr.Owner,
					Repo:      pr.Repo,
					Number:    pr.Number,
					Title:     pr.Title,
					URL:       pr.URL,
					Actor:     member.Login,
				})
			}
		}
	} else {
		// Fallback: user-scoped data
		for _, info := range m.prInfos {
			events = append(events, TimelineEvent{
				EventType: TimelineEventCreated,
				Timestamp: info.CreatedAt,
				Owner:     info.Owner,
				Repo:      info.Repo,
				Number:    info.Number,
				Title:     info.Title,
				URL:       info.URL,
			})
		}
		for _, pr := range m.dashboardStats.MergedPRs {
			events = append(events, TimelineEvent{
				EventType: TimelineEventMerged,
				Timestamp: pr.MergedAt,
				Owner:     pr.Owner,
				Repo:      pr.Repo,
				Number:    pr.Number,
				Title:     pr.Title,
				URL:       pr.URL,
			})
		}
	}

	// Approved events from user's PRs reviews (always available)
	for _, info := range m.prInfos {
		for _, r := range info.Reviews {
			if r.State == "APPROVED" {
				events = append(events, TimelineEvent{
					EventType: TimelineEventApproved,
					Timestamp: r.SubmittedAt,
					Owner:     info.Owner,
					Repo:      info.Repo,
					Number:    info.Number,
					Title:     info.Title,
					URL:       info.URL,
					Actor:     r.User.Login,
				})
			}
		}
	}

	// Sort most-recent-first
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events
}

// updateTimelineList rebuilds the timeline pane from current data.
func (m *Model) updateTimelineList() {
	events := m.buildTimelineEvents()
	items := make([]list.Item, len(events))
	for i, e := range events {
		items[i] = e
	}
	m.timelineList.SetItems(items)
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
