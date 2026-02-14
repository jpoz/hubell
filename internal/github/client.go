package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	baseURL       = "https://api.github.com"
	apiVersion    = "application/vnd.github+json"
	apiVersionHdr = "2022-11-28"
)

// Client is a GitHub API client
type Client struct {
	token        string
	httpClient   *http.Client
	lastModified string
}

// NewClient creates a new GitHub API client
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// setHeaders sets the common GitHub API headers on a request
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", apiVersion)
	req.Header.Set("X-GitHub-Api-Version", apiVersionHdr)
}

// ListNotifications fetches all notifications for the authenticated user
// Uses Last-Modified header for efficient polling (returns nil if 304 Not Modified)
func (c *Client) ListNotifications(ctx context.Context) ([]*Notification, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/notifications", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	// Add If-Modified-Since header for efficient polling
	if c.lastModified != "" {
		req.Header.Set("If-Modified-Since", c.lastModified)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified - no new notifications
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}

	// Handle other error status codes
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: token may be invalid or expired")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited or forbidden (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Store Last-Modified header for next request
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		c.lastModified = lm
	}

	// Decode notifications
	var notifications []*Notification
	if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return notifications, nil
}

// MarkAsRead marks a notification thread as read
func (c *Client) MarkAsRead(ctx context.Context, threadID string) error {
	url := fmt.Sprintf("%s/notifications/threads/%s", baseURL, threadID)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Expect 205 Reset Content
	if resp.StatusCode != http.StatusResetContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// GetAuthenticatedUser returns the currently authenticated user
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/user", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// SearchUserOpenPRs fetches all open pull requests created by the authenticated user.
// It merges results from /user/issues (which includes private repos when the token has
// repo scope) and the search API (which includes PRs on repos where the user is not a
// member, e.g. open source contributions via forks).
func (c *Client) SearchUserOpenPRs(ctx context.Context, username string) (*SearchResult, error) {
	seen := make(map[string]struct{})
	var allItems []SearchItem

	// /user/issues covers private repos where the user is a collaborator/member
	if result, err := c.listUserOpenPRs(ctx); err == nil {
		for _, item := range result.Items {
			key := item.HTMLURL
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				allItems = append(allItems, item)
			}
		}
	}

	// Search API covers external repos (forks, open source contributions)
	if result, err := c.searchUserOpenPRs(ctx, username); err == nil {
		for _, item := range result.Items {
			key := item.HTMLURL
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				allItems = append(allItems, item)
			}
		}
	}

	if len(allItems) == 0 {
		return nil, fmt.Errorf("failed to fetch open PRs from any source")
	}

	return &SearchResult{Items: allItems}, nil
}

// listUserOpenPRs uses GET /user/issues to list PRs including private repos.
// Requires repo scope on the token.
func (c *Client) listUserOpenPRs(ctx context.Context) (*SearchResult, error) {
	var allItems []SearchItem

	for page := 1; ; page++ {
		pageURL := fmt.Sprintf("%s/user/issues?filter=created&state=open&per_page=100&page=%d", baseURL, page)

		req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
		if err != nil {
			return nil, err
		}

		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("/user/issues: status %d", resp.StatusCode)
		}

		var items []SearchItem
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode user issues: %w", err)
		}
		resp.Body.Close()

		for _, item := range items {
			// Only include pull requests (items with a pull_request ref)
			if item.PullRequestRef.URL != "" {
				allItems = append(allItems, item)
			}
		}

		if len(items) < 100 {
			break
		}
	}

	return &SearchResult{Items: allItems}, nil
}

// searchUserOpenPRs uses the search API as a fallback. Works for public repos
// without repo scope but does not reliably include private repos.
func (c *Client) searchUserOpenPRs(ctx context.Context, username string) (*SearchResult, error) {
	q := fmt.Sprintf("author:%s+type:pr+state:open", username)
	u := fmt.Sprintf("%s/search/issues?q=%s&per_page=100", baseURL, q)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search: status %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return &result, nil
}

// GetPullRequest fetches a specific pull request
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", baseURL, owner, repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var pr PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to decode pull request: %w", err)
	}

	return &pr, nil
}

// GetPullRequestReviews fetches reviews for a pull request
func (c *Client) GetPullRequestReviews(ctx context.Context, owner, repo string, number int) ([]Review, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", baseURL, owner, repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var reviews []Review
	if err := json.NewDecoder(resp.Body).Decode(&reviews); err != nil {
		return nil, fmt.Errorf("failed to decode reviews: %w", err)
	}

	return reviews, nil
}

// GetCheckRuns fetches check runs for a given commit SHA
func (c *Client) GetCheckRuns(ctx context.Context, owner, repo, sha string) (*CheckRunsResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs", baseURL, owner, repo, sha)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CheckRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode check runs: %w", err)
	}

	return &result, nil
}
