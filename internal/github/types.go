package github

import "time"

// Notification represents a GitHub notification
type Notification struct {
	ID         string     `json:"id"`
	Unread     bool       `json:"unread"`
	Reason     string     `json:"reason"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Subject    Subject    `json:"subject"`
	Repository Repository `json:"repository"`
}

// Subject represents the notification subject
type Subject struct {
	Title string `json:"title"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// Repository represents the repository info
type Repository struct {
	FullName string `json:"full_name"`
	Owner    Owner  `json:"owner"`
}

// Owner represents the repository owner
type Owner struct {
	Login string `json:"login"`
}
