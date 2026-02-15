package github

import (
	"context"
	"fmt"
	"maps"
	"time"
)

// LoadingStep identifies a step in the initial loading sequence.
type LoadingStep int

const (
	StepNotifications LoadingStep = iota
	StepPullRequests
	StepMergedPRs
	StepWeeklyStats
)

// LoadingProgress reports progress for a loading step.
// For steps without granular progress, Current and Total are both 0.
type LoadingProgress struct {
	Step    LoadingStep
	Current int
	Total   int
	Done    bool
}

// PollResult contains the result of a polling operation
type PollResult struct {
	Notifications      []*Notification
	PRStatuses         map[string]PRStatus
	PRInfos            map[string]PRInfo
	PRChanges          []PRStatusChange
	MergedPRs          []MergedPRInfo
	WeeklyMergedCounts map[string]int // backfill: ISO week key → count (first poll only)
	Error              error
}

// Poller polls GitHub notifications and open PR statuses at a regular interval
type Poller struct {
	client     *Client
	interval   time.Duration
	username   string
	prStatuses map[string]PRStatus
	prInfos    map[string]PRInfo
	progressCh chan<- LoadingProgress
}

// NewPoller creates a new poller
func NewPoller(client *Client, interval time.Duration, username string, progressCh chan<- LoadingProgress) *Poller {
	return &Poller{
		client:     client,
		interval:   interval,
		username:   username,
		prStatuses: make(map[string]PRStatus),
		prInfos:    make(map[string]PRInfo),
		progressCh: progressCh,
	}
}

// Start begins polling and sends results on the returned channel
func (p *Poller) Start(ctx context.Context) <-chan PollResult {
	resultCh := make(chan PollResult, 1)

	go func() {
		defer close(resultCh)

		// Poll immediately on startup (first poll: no PR change notifications)
		result := p.poll(ctx, true)
		if p.progressCh != nil {
			close(p.progressCh)
			p.progressCh = nil
		}
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
	if firstPoll && p.progressCh != nil {
		p.progressCh <- LoadingProgress{Step: StepNotifications, Done: true}
	}

	var prProgressCh chan<- LoadingProgress
	if firstPoll {
		prProgressCh = p.progressCh
	}
	prStatuses, prInfos, prErr := pollAllPRs(ctx, p.client, p.username, prProgressCh)
	if firstPoll && p.progressCh != nil {
		p.progressCh <- LoadingProgress{Step: StepPullRequests, Done: true}
	}

	// Fetch merged PRs for dashboard (errors swallowed — informational only)
	var mergedPRs []MergedPRInfo
	if merged, err := p.client.SearchMergedPRsThisWeek(ctx, p.username); err == nil {
		mergedPRs = merged
	}
	if firstPoll && p.progressCh != nil {
		p.progressCh <- LoadingProgress{Step: StepMergedPRs, Done: true}
	}

	// On first poll, backfill weekly merged counts for the last 12 weeks
	var weeklyMergedCounts map[string]int
	if firstPoll {
		since := time.Now().AddDate(0, 0, -12*7)
		if allMerged, err := p.client.SearchMergedPRsSince(ctx, p.username, since); err == nil {
			weeklyMergedCounts = make(map[string]int)
			for _, pr := range allMerged {
				if pr.MergedAt.IsZero() {
					continue
				}
				year, week := pr.MergedAt.ISOWeek()
				key := fmt.Sprintf("%d-W%02d", year, week)
				weeklyMergedCounts[key]++
			}
		}
		if p.progressCh != nil {
			p.progressCh <- LoadingProgress{Step: StepWeeklyStats, Done: true}
		}
	}

	// If both failed, return the notification error
	if notifErr != nil && prErr != nil {
		return PollResult{Error: notifErr}
	}

	var result PollResult
	result.Notifications = notifications
	result.MergedPRs = mergedPRs
	result.WeeklyMergedCounts = weeklyMergedCounts

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
