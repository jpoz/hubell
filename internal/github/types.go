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

// User represents the authenticated GitHub user
type User struct {
	Login string `json:"login"`
}

// SearchResult represents a GitHub search API response
type SearchResult struct {
	Items []SearchItem `json:"items"`
}

// SearchItem represents an item from the search API
type SearchItem struct {
	Number         int            `json:"number"`
	Title          string         `json:"title"`
	HTMLURL        string         `json:"html_url"`
	CreatedAt      time.Time      `json:"created_at"`
	ClosedAt       *time.Time     `json:"closed_at"`
	PullRequestRef PullRequestRef `json:"pull_request"`
	RepositoryURL  string         `json:"repository_url"`
}

// PullRequestRef contains pull request metadata from a search result
type PullRequestRef struct {
	URL string `json:"url"`
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Head      PRHead `json:"head"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// PRHead represents the head ref of a pull request
type PRHead struct {
	SHA string `json:"sha"`
}

// CheckRunsResponse represents the response from the check-runs API
type CheckRunsResponse struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []CheckRun `json:"check_runs"`
}

// CheckRun represents a single check run
type CheckRun struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// PRInfo contains metadata about an open pull request
type PRInfo struct {
	Owner       string
	Repo        string
	Number      int
	Title       string
	URL         string
	CreatedAt   time.Time
	ReviewState PRReviewState
	Reviews     []Review
	Additions   int
	Deletions   int
	CheckRuns   []CheckRun
}

// MergedPRInfo contains metadata about a merged pull request
type MergedPRInfo struct {
	Owner    string
	Repo     string
	Number   int
	Title    string
	URL      string
	MergedAt time.Time
}

// PRReviewState represents the aggregate review approval state of a PR
type PRReviewState string

const (
	PRReviewNone             PRReviewState = ""
	PRReviewApproved         PRReviewState = "approved"
	PRReviewChangesRequested PRReviewState = "changes_requested"
	PRReviewReviewed         PRReviewState = "reviewed"
)

// Review represents a single pull request review
type Review struct {
	ID          int       `json:"id"`
	User        User      `json:"user"`
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// PRStatus represents the aggregate CI status of a pull request
type PRStatus string

const (
	PRStatusNone    PRStatus = "none"
	PRStatusPending PRStatus = "pending"
	PRStatusSuccess PRStatus = "success"
	PRStatusFailure PRStatus = "failure"
)

// PRStatusChange represents a CI status transition on a pull request
type PRStatusChange struct {
	Owner     string
	Repo      string
	Number    int
	Title     string
	URL       string
	OldStatus PRStatus
	NewStatus PRStatus
}
