package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/browser"
	"github.com/jpoz/hubell/internal/config"
	"github.com/jpoz/hubell/internal/github"
	"github.com/jpoz/hubell/internal/notify"
)

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case PollResultMsg:
		m.loading = false
		m.err = nil
		if msg.PRStatuses != nil {
			m.prStatuses = msg.PRStatuses
		}
		if msg.PRInfos != nil {
			m.prInfos = msg.PRInfos
		}
		for _, change := range msg.PRChanges {
			notify.SendDesktopNotification(
				fmt.Sprintf("CI %s: %s/%s", change.NewStatus, change.Owner, change.Repo),
				fmt.Sprintf("PR #%d: %s (%s → %s)", change.Number, change.Title, change.OldStatus, change.NewStatus),
			)
		}
		m.dashboardStats.updateFromPollResult(msg.MergedPRs, msg.WeeklyMergedCounts, msg.PRInfos)
		m.updateNotifications(msg.Notifications)
		m.updatePRList()
		return m, waitForPollResult(m.pollCh)

	case LoadingProgressMsg:
		if msg.Done {
			m.loadingSteps[msg.Step] = true
		}
		m.prProgress = msg.LoadingProgress
		return m, waitForLoadingStep(m.progressCh)

	case BannerTickMsg:
		if m.loading || m.orgLoading || m.engineerLoading {
			m.bannerFrame++
			return m, bannerTick()
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, waitForPollResult(m.pollCh)

	case MarkAsReadSuccessMsg:
		delete(m.allNotifications, msg.ThreadID)
		m.updateNotifications(nil)
		return m, nil

	case MarkAsReadErrorMsg:
		m.err = msg.Err
		return m, nil

	case OrgDataMsg:
		m.orgLoading = false
		m.orgError = nil
		m.orgMembers = msg.Members
		m.orgSelectedIndex = 0
		m.sortOrgMembers()
		return m, nil

	case EngineerDetailMsg:
		m.engineerLoading = false
		m.engineerDetail = msg.Detail
		m.engineerSelectedPR = 0
		m.engineerScroll = 0
		return m, nil

	case OrgErrorMsg:
		m.orgLoading = false
		m.engineerLoading = false
		m.orgError = msg.Err
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	// Pass to the focused list for navigation
	var cmd tea.Cmd
	if m.focusedPane == LeftPane {
		m.list, cmd = m.list.Update(msg)
	} else {
		m.prList, cmd = m.prList.Update(msg)
	}
	return m, cmd
}

// handleKeyMsg routes keyboard events to the appropriate handler.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Engineer detail overlay (innermost)
	if m.showEngineerDetail {
		return m.handleEngineerDetailKey(msg)
	}

	// Org dashboard overlay
	if m.showOrgDashboard {
		return m.handleOrgDashboardKey(msg)
	}

	// Activity dashboard overlay
	if m.showDashboard {
		switch msg.String() {
		case "esc", "q", "d":
			m.showDashboard = false
			return m, nil
		}
		return m, nil
	}

	// Theme selector overlay
	if m.showThemeSelector {
		switch msg.String() {
		case "esc", "q":
			m.showThemeSelector = false
			return m, nil
		case "enter":
			if item, ok := m.themeList.SelectedItem().(ThemeItem); ok {
				m.applyTheme(item.key)
			}
			m.showThemeSelector = false
			return m, nil
		default:
			var cmd tea.Cmd
			m.themeList, cmd = m.themeList.Update(msg)
			return m, cmd
		}
	}

	// Main TUI keys
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancel()
		return m, tea.Quit

	case "d":
		m.showDashboard = true
		return m, nil

	case "t":
		m.showThemeSelector = true
		return m, nil

	case "o":
		m.showOrgDashboard = true
		m.orgError = nil
		if m.orgName == "" {
			m.orgInputActive = true
			m.orgInput.Focus()
			return m, m.orgInput.Cursor.BlinkCmd()
		}
		if len(m.orgMembers) == 0 && !m.orgLoading {
			m.orgLoading = true
			return m, tea.Batch(bannerTick(), fetchOrgData(m.ctx, m.githubClient, m.orgName))
		}
		return m, nil

	case "tab":
		if m.focusedPane == LeftPane {
			m.focusedPane = RightPane
		} else {
			m.focusedPane = LeftPane
		}
		return m, nil

	case "enter":
		if m.focusedPane == LeftPane {
			if selectedItem, ok := m.list.SelectedItem().(NotificationItem); ok {
				webURL := github.ConvertAPIURLToWeb(selectedItem.notification.Subject.URL)
				if err := browser.Open(webURL); err != nil {
					m.err = err
				}
			}
		} else {
			if selectedItem, ok := m.prList.SelectedItem().(PRItem); ok {
				if err := browser.Open(selectedItem.info.URL); err != nil {
					m.err = err
				}
			}
		}
		return m, nil

	case "r", "m":
		if m.focusedPane == LeftPane {
			if selectedItem, ok := m.list.SelectedItem().(NotificationItem); ok {
				return m, markAsRead(m.ctx, m.githubClient, selectedItem.notification.ID)
			}
		}
		return m, nil

	case "f":
		if m.focusedPane == LeftPane {
			m.filterMode = (m.filterMode + 1) % 2
			m.updateNotifications(nil)
		}
		return m, nil
	}

	return m, nil
}

// handleOrgDashboardKey handles keyboard events in the org dashboard overlay.
func (m *Model) handleOrgDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Text input mode for org name
	if m.orgInputActive {
		switch msg.String() {
		case "esc":
			m.orgInputActive = false
			m.showOrgDashboard = false
			return m, nil
		case "enter":
			val := m.orgInput.Value()
			if val != "" {
				m.orgName = val
				m.orgInputActive = false
				_ = config.SaveOrg(m.orgName)
				m.orgLoading = true
				return m, tea.Batch(bannerTick(), fetchOrgData(m.ctx, m.githubClient, m.orgName))
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.orgInput, cmd = m.orgInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc", "q":
		m.showOrgDashboard = false
		return m, nil

	case "up", "k":
		if m.orgSelectedIndex > 0 {
			m.orgSelectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.orgSelectedIndex < len(m.orgMembers)-1 {
			m.orgSelectedIndex++
		}
		return m, nil

	case "s":
		m.orgSortColumn = (m.orgSortColumn + 1) % 3
		m.sortOrgMembers()
		m.orgSelectedIndex = 0
		return m, nil

	case "enter":
		if !m.orgLoading && m.orgSelectedIndex < len(m.orgMembers) {
			member := m.orgMembers[m.orgSelectedIndex]
			m.showEngineerDetail = true
			m.engineerLoading = true
			m.engineerSelectedPR = 0
			m.engineerScroll = 0
			return m, tea.Batch(bannerTick(), fetchEngineerDetail(m.ctx, m.githubClient, m.orgName, member.Login))
		}
		return m, nil

	case "r":
		if !m.orgLoading {
			m.orgLoading = true
			m.orgError = nil
			return m, tea.Batch(bannerTick(), fetchOrgData(m.ctx, m.githubClient, m.orgName))
		}
		return m, nil
	}

	return m, nil
}

// handleEngineerDetailKey handles keyboard events in the engineer detail overlay.
func (m *Model) handleEngineerDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showEngineerDetail = false
		m.engineerDetail = nil
		return m, nil

	case "up", "k":
		if m.engineerSelectedPR > 0 {
			m.engineerSelectedPR--
		} else if m.engineerScroll > 0 {
			m.engineerScroll--
		}
		return m, nil

	case "down", "j":
		if m.engineerDetail != nil && m.engineerSelectedPR < len(m.engineerDetail.MergedPRs)-1 {
			m.engineerSelectedPR++
		} else {
			m.engineerScroll++
		}
		return m, nil

	case "enter":
		if m.engineerDetail != nil && len(m.engineerDetail.MergedPRs) > 0 && m.engineerSelectedPR < len(m.engineerDetail.MergedPRs) {
			pr := m.engineerDetail.MergedPRs[m.engineerSelectedPR]
			if err := browser.Open(pr.URL); err != nil {
				m.err = err
			}
		}
		return m, nil
	}

	return m, nil
}

// fetchOrgData creates a command that fetches org activity data.
func fetchOrgData(ctx context.Context, client *github.Client, org string) tea.Cmd {
	return func() tea.Msg {
		members, err := client.FetchOrgActivity(ctx, org)
		if err != nil {
			return OrgErrorMsg{Err: err}
		}
		return OrgDataMsg{Members: members}
	}
}

// fetchEngineerDetail creates a command that fetches detailed engineer data.
func fetchEngineerDetail(ctx context.Context, client *github.Client, org, login string) tea.Cmd {
	return func() tea.Msg {
		detail, err := client.FetchEngineerDetail(ctx, org, login)
		if err != nil {
			return OrgErrorMsg{Err: err}
		}
		return EngineerDetailMsg{Detail: detail}
	}
}

// markAsRead creates a command to mark a notification as read
func markAsRead(ctx context.Context, client *github.Client, threadID string) tea.Cmd {
	return func() tea.Msg {
		err := client.MarkAsRead(ctx, threadID)
		if err != nil {
			return MarkAsReadErrorMsg{Err: err}
		}
		return MarkAsReadSuccessMsg{ThreadID: threadID}
	}
}
