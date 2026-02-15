package github

import (
	"context"
	"maps"
	"time"
)

// PollResult contains the result of a polling operation
type PollResult struct {
	Notifications []*Notification
	PRStatuses    map[string]PRStatus
	PRInfos       map[string]PRInfo
	PRChanges     []PRStatusChange
	MergedPRs     []MergedPRInfo
	Error         error
}

// Poller polls GitHub notifications and open PR statuses at a regular interval
type Poller struct {
	client     *Client
	interval   time.Duration
	username   string
	prStatuses map[string]PRStatus
	prInfos    map[string]PRInfo
}

// NewPoller creates a new poller
func NewPoller(client *Client, interval time.Duration, username string) *Poller {
	return &Poller{
		client:     client,
		interval:   interval,
		username:   username,
		prStatuses: make(map[string]PRStatus),
		prInfos:    make(map[string]PRInfo),
	}
}

// Start begins polling and sends results on the returned channel
func (p *Poller) Start(ctx context.Context) <-chan PollResult {
	resultCh := make(chan PollResult, 1)

	go func() {
		defer close(resultCh)

		// Poll immediately on startup (first poll: no PR change notifications)
		result := p.poll(ctx, true)
		resultCh <- result

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result := p.poll(ctx, false)
				resultCh <- result
			}
		}
	}()

	return resultCh
}

// poll performs a single poll cycle for both notifications and PR statuses
func (p *Poller) poll(ctx context.Context, firstPoll bool) PollResult {
	notifications, notifErr := p.client.ListNotifications(ctx)

	prStatuses, prInfos, prErr := pollAllPRs(ctx, p.client, p.username)

	// Fetch merged PRs for dashboard (errors swallowed — informational only)
	var mergedPRs []MergedPRInfo
	if merged, err := p.client.SearchMergedPRsThisWeek(ctx, p.username); err == nil {
		mergedPRs = merged
	}

	// If both failed, return the notification error
	if notifErr != nil && prErr != nil {
		return PollResult{Error: notifErr}
	}

	var result PollResult
	result.Notifications = notifications
	result.MergedPRs = mergedPRs

	if prStatuses != nil {
		// Detect CI status changes (skip on first poll to establish baseline)
		if !firstPoll {
			for key, newStatus := range prStatuses {
				oldStatus, exists := p.prStatuses[key]
				if !exists {
					oldStatus = PRStatusNone
				}
				if oldStatus != newStatus {
					if info, ok := prInfos[key]; ok {
						result.PRChanges = append(result.PRChanges, PRStatusChange{
							Owner:     info.Owner,
							Repo:      info.Repo,
							Number:    info.Number,
							Title:     info.Title,
							URL:       info.URL,
							OldStatus: oldStatus,
							NewStatus: newStatus,
						})
					}
				}
			}
		}

		p.prStatuses = prStatuses
		p.prInfos = prInfos

		result.PRStatuses = make(map[string]PRStatus, len(prStatuses))
		maps.Copy(result.PRStatuses, prStatuses)

		result.PRInfos = make(map[string]PRInfo, len(prInfos))
		maps.Copy(result.PRInfos, prInfos)
	}

	return result
}
