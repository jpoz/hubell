package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/browser"
	"github.com/jpoz/hubell/internal/github"
)

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-5) // Leave room for help text
		return m, nil

	case NotificationsMsg:
		m.err = nil
		m.updateNotifications(msg.Notifications)
		return m, waitForPollResult(m.pollCh)

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
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit

		case "enter":
			// Open notification in browser
			if selectedItem, ok := m.list.SelectedItem().(NotificationItem); ok {
				webURL := github.ConvertAPIURLToWeb(selectedItem.notification.Subject.URL)
				if err := browser.Open(webURL); err != nil {
					m.err = err
				}
			}
			return m, nil

		case "r", "m":
			// Mark selected notification as read
			if selectedItem, ok := m.list.SelectedItem().(NotificationItem); ok {
				return m, markAsRead(m.ctx, m.githubClient, selectedItem.notification.ID)
			}
			return m, nil

		case "f":
			// Cycle through filter modes
			m.filterMode = (m.filterMode + 1) % 2
			m.updateNotifications(nil)
			return m, nil
		}
	}

	// Pass to list for navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
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
