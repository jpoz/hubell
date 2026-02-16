package github

import (
	"context"
	"fmt"
	"maps"
	"sync"
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
	CommentDetails     map[string]*CommentDetail // keyed by notification ID
	Error              error
}

// Poller polls GitHub notifications and open PR statuses at a regular interval
type Poller struct {
	client         *Client
	interval       time.Duration
	username       string
	prStatuses     map[string]PRStatus
	prInfos        map[string]PRInfo
	progressCh     chan<- LoadingProgress
	commentDetails map[string]*CommentDetail // cache keyed by LatestCommentURL
}

// NewPoller creates a new poller
func NewPoller(client *Client, interval time.Duration, username string, progressCh chan<- LoadingProgress) *Poller {
	return &Poller{
		client:         client,
		interval:       interval,
		username:       username,
		prStatuses:     make(map[string]PRStatus),
		prInfos:        make(map[string]PRInfo),
		progressCh:     progressCh,
		commentDetails: make(map[string]*CommentDetail),
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

// poll performs a single poll cycle for both notifications and PR statuses.
// All independent API calls run concurrently to minimize startup latency.
func (p *Poller) poll(ctx context.Context, firstPoll bool) PollResult {
	var (
		notifications      []*Notification
		notifErr           error
		prStatuses         map[string]PRStatus
		prInfos            map[string]PRInfo
		prErr              error
		mergedPRs          []MergedPRInfo
		weeklyMergedCounts map[string]int
	)

	var wg sync.WaitGroup

	// 1. Notifications
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifications, notifErr = p.client.ListNotifications(ctx)
		if firstPoll && p.progressCh != nil {
			p.progressCh <- LoadingProgress{Step: StepNotifications, Done: true}
		}
	}()

	// 2. Open PR statuses
	wg.Add(1)
	go func() {
		defer wg.Done()
		var prProgressCh chan<- LoadingProgress
		if firstPoll {
			prProgressCh = p.progressCh
		}
		prStatuses, prInfos, prErr = pollAllPRs(ctx, p.client, p.username, prProgressCh)
		if firstPoll && p.progressCh != nil {
			p.progressCh <- LoadingProgress{Step: StepPullRequests, Done: true}
		}
	}()

	// 3. Merged PRs this week
	wg.Add(1)
	go func() {
		defer wg.Done()
		if merged, err := p.client.SearchMergedPRsThisWeek(ctx, p.username); err == nil {
			mergedPRs = merged
		}
		if firstPoll && p.progressCh != nil {
			p.progressCh <- LoadingProgress{Step: StepMergedPRs, Done: true}
		}
	}()

	// 4. Weekly stats backfill (first poll only)
	if firstPoll {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	wg.Wait()

	// If both failed, return the notification error
	if notifErr != nil && prErr != nil {
		return PollResult{Error: notifErr}
	}

	// Enrich notifications with comment details
	commentDetails := p.enrichNotifications(ctx, notifications)

	var result PollResult
	result.Notifications = notifications
	result.MergedPRs = mergedPRs
	result.WeeklyMergedCounts = weeklyMergedCounts
	result.CommentDetails = commentDetails

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

// enrichNotifications concurrently fetches comment details for notifications
// that have a LatestCommentURL. Results are cached by URL to avoid redundant
// requests. Returns a map keyed by notification ID.
func (p *Poller) enrichNotifications(ctx context.Context, notifications []*Notification) map[string]*CommentDetail {
	if len(notifications) == 0 {
		return nil
	}

	// Identify notifications that need fetching (not yet cached)
	type fetchItem struct {
		notifID string
		url     string
	}
	var toFetch []fetchItem
	result := make(map[string]*CommentDetail)

	activeURLs := make(map[string]struct{})
	for _, n := range notifications {
		url := n.Subject.LatestCommentURL
		if url == "" {
			continue
		}
		activeURLs[url] = struct{}{}
		if detail, ok := p.commentDetails[url]; ok {
			result[n.ID] = detail
		} else {
			toFetch = append(toFetch, fetchItem{notifID: n.ID, url: url})
		}
	}

	// Evict stale cache entries
	for url := range p.commentDetails {
		if _, ok := activeURLs[url]; !ok {
			delete(p.commentDetails, url)
		}
	}

	if len(toFetch) == 0 {
		return result
	}

	// Fetch concurrently with a semaphore of 5
	type fetchResult struct {
		notifID string
		url     string
		detail  *CommentDetail
	}
	resultCh := make(chan fetchResult, len(toFetch))
	sem := make(chan struct{}, 5)

	var wg sync.WaitGroup
	for _, item := range toFetch {
		wg.Add(1)
		go func(fi fetchItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			detail, err := p.client.FetchCommentDetail(ctx, fi.url)
			if err != nil {
				return
			}
			resultCh <- fetchResult{notifID: fi.notifID, url: fi.url, detail: detail}
		}(item)
	}
	wg.Wait()
	close(resultCh)

	for fr := range resultCh {
		p.commentDetails[fr.url] = fr.detail
		result[fr.notifID] = fr.detail
	}

	return result
}
