package tui

import (
	"github.com/jpoz/hubell/internal/github"
)

// NotificationsMsg is sent when new notifications are received
type NotificationsMsg struct {
	Notifications []*github.Notification
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

// MarkAsReadMsg is sent when a notification is marked as read
type MarkAsReadMsg struct {
	ThreadID string
}

// MarkAsReadSuccessMsg is sent when marking as read succeeds
type MarkAsReadSuccessMsg struct {
	ThreadID string
}

// MarkAsReadErrorMsg is sent when marking as read fails
type MarkAsReadErrorMsg struct {
	Err error
}
