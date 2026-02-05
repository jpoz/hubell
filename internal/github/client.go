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

// ListNotifications fetches all notifications for the authenticated user
// Uses Last-Modified header for efficient polling (returns nil if 304 Not Modified)
func (c *Client) ListNotifications(ctx context.Context) ([]*Notification, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/notifications", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", apiVersion)
	req.Header.Set("X-GitHub-Api-Version", apiVersionHdr)

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

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", apiVersion)
	req.Header.Set("X-GitHub-Api-Version", apiVersionHdr)

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
