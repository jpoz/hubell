package tui

import (
	"github.com/jpoz/hubell/internal/github"
)

// PollResultMsg is sent when new poll results are received
type PollResultMsg struct {
	Notifications []*github.Notification
	PRStatuses    map[string]github.PRStatus
	PRInfos       map[string]github.PRInfo
	PRChanges     []github.PRStatusChange
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

// BannerTickMsg is sent on each animation frame for the loading banner pulse
type BannerTickMsg struct{}
