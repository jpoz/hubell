package github

import (
	"context"
	"time"
)

// PollResult contains the result of a polling operation
type PollResult struct {
	Notifications []*Notification
	Error         error
}

// Poller polls GitHub notifications at a regular interval
type Poller struct {
	client   *Client
	interval time.Duration
}

// NewPoller creates a new notification poller
func NewPoller(client *Client, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		interval: interval,
	}
}

// Start begins polling and sends results on the returned channel
// The polling will continue until the context is cancelled
func (p *Poller) Start(ctx context.Context) <-chan PollResult {
	resultCh := make(chan PollResult, 1)

	go func() {
		defer close(resultCh)

		// Poll immediately on startup
		notifications, err := p.client.ListNotifications(ctx)
		resultCh <- PollResult{
			Notifications: notifications,
			Error:         err,
		}

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				notifications, err := p.client.ListNotifications(ctx)
				// Only send if we have new notifications or an error
				if notifications != nil || err != nil {
					resultCh <- PollResult{
						Notifications: notifications,
						Error:         err,
					}
				}
			}
		}
	}()

	return resultCh
}
