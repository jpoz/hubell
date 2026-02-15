package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/browser"
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
		if m.loading {
			m.bannerFrame++
			return m, bannerTick()
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, waitForPollResult(m.pollCh)

	case MarkAsReadSuccessMsg:
		// Remove the notification from the list
		delete(m.allNotifications, msg.ThreadID)
		m.updateNotifications(nil)
		return m, nil

	case MarkAsReadErrorMsg:
		m.err = msg.Err
		return m, nil

	case tea.KeyMsg:
		// Route keys to dashboard when it's open
		if m.showDashboard {
			switch msg.String() {
			case "esc", "q", "d":
				m.showDashboard = false
				return m, nil
			}
			return m, nil
		}

		// Route keys to theme selector when it's open
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
