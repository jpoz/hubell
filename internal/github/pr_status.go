package github

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// pollAllPRs fetches all open PRs and their CI statuses concurrently.
// If progressCh is non-nil, per-PR progress updates are sent on it.
func pollAllPRs(ctx context.Context, client *Client, username string, progressCh chan<- LoadingProgress) (map[string]PRStatus, map[string]PRInfo, error) {
	searchResult, err := client.SearchUserOpenPRs(ctx, username)
	if err != nil {
		return nil, nil, fmt.Errorf("searching open PRs: %w", err)
	}

	total := len(searchResult.Items)
	statuses := make(map[string]PRStatus)
	infos := make(map[string]PRInfo)

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		completed int32
		sem       = make(chan struct{}, 5) // limit concurrent API calls
	)

	for _, item := range searchResult.Items {
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" || repo == "" {
			continue
		}

		wg.Add(1)
		go func(item SearchItem, owner, repo string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			key := PRKey(owner, repo, item.Number)
			info := PRInfo{
				Owner:     owner,
				Repo:      repo,
				Number:    item.Number,
				Title:     item.Title,
				URL:       item.HTMLURL,
				CreatedAt: item.CreatedAt,
			}
			status := PRStatusNone

			pr, err := client.GetPullRequest(ctx, owner, repo, item.Number)
			if err == nil {
				info.Branch = pr.Head.Ref
				info.Additions = pr.Additions
				info.Deletions = pr.Deletions

				// Fetch check runs, commit status, and reviews concurrently
				var (
					checkRuns    *CheckRunsResponse
					commitStatus *CombinedStatus
					reviews      []Review
					crErr        error
					innerWg      sync.WaitGroup
				)

				innerWg.Add(3)
				go func() {
					defer innerWg.Done()
					checkRuns, crErr = client.GetCheckRuns(ctx, owner, repo, pr.Head.SHA)
				}()
				go func() {
					defer innerWg.Done()
					commitStatus, _ = client.GetCommitStatus(ctx, owner, repo, pr.Head.SHA)
				}()
				go func() {
					defer innerWg.Done()
					reviews, _ = client.GetPullRequestReviews(ctx, owner, repo, item.Number)
				}()
				innerWg.Wait()

				if crErr == nil {
					if commitStatus != nil {
						for _, s := range commitStatus.Statuses {
							checkRuns.CheckRuns = append(checkRuns.CheckRuns, statusToCheckRun(s))
							checkRuns.TotalCount++
						}
					}
					status = computeAggregateStatus(checkRuns)
					info.CheckRuns = checkRuns.CheckRuns
				}

				if reviews != nil {
					info.ReviewState = computeReviewState(reviews)
					info.Reviews = reviews
				}
			}

			done := atomic.AddInt32(&completed, 1)
			if progressCh != nil {
				progressCh <- LoadingProgress{Step: StepPullRequests, Current: int(done), Total: total}
			}

			mu.Lock()
			statuses[key] = status
			infos[key] = info
			mu.Unlock()
		}(item, owner, repo)
	}

	wg.Wait()
	return statuses, infos, nil
}

// PRKey builds the map key for a PR: "owner/repo#number"
func PRKey(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

// parseRepoURL extracts owner and repo from a GitHub API repository URL
// e.g., "https://api.github.com/repos/owner/repo" -> "owner", "repo"
func parseRepoURL(repoURL string) (string, string) {
	const prefix = "https://api.github.com/repos/"
	if !strings.HasPrefix(repoURL, prefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(repoURL, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// computeReviewState computes the aggregate review state from PR reviews.
// It takes the latest review per user (by position in the list) and returns
// the most significant state: changes_requested > approved > reviewed > none.
func computeReviewState(reviews []Review) PRReviewState {
	if len(reviews) == 0 {
		return PRReviewNone
	}

	// GitHub returns reviews in chronological order, so later entries
	// for the same user override earlier ones.
	latestByUser := make(map[string]string)
	for _, r := range reviews {
		if r.User.Login == "" {
			continue
		}
		// Only track meaningful review states
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "COMMENTED":
			latestByUser[r.User.Login] = r.State
		case "DISMISSED":
			delete(latestByUser, r.User.Login)
		}
	}

	if len(latestByUser) == 0 {
		return PRReviewNone
	}

	hasApproval := false
	for _, state := range latestByUser {
		switch state {
		case "CHANGES_REQUESTED":
			return PRReviewChangesRequested
		case "APPROVED":
			hasApproval = true
		}
	}

	if hasApproval {
		return PRReviewApproved
	}

	return PRReviewReviewed
}

// computeAggregateStatus computes the overall CI status from check runs
func computeAggregateStatus(checkRuns *CheckRunsResponse) PRStatus {
	if checkRuns.TotalCount == 0 {
		return PRStatusNone
	}

	for _, cr := range checkRuns.CheckRuns {
		if cr.Status == "queued" || cr.Status == "in_progress" {
			return PRStatusPending
		}
	}

	for _, cr := range checkRuns.CheckRuns {
		switch cr.Conclusion {
		case "failure", "cancelled", "timed_out":
			return PRStatusFailure
		}
	}

	return PRStatusSuccess
}

// statusToCheckRun converts a legacy CommitStatus into a CheckRun so the
// display layer can handle both uniformly.
func statusToCheckRun(s CommitStatus) CheckRun {
	cr := CheckRun{
		ID:   s.ID,
		Name: s.Context,
	}

	switch s.State {
	case "success":
		cr.Status = "completed"
		cr.Conclusion = "success"
	case "failure", "error":
		cr.Status = "completed"
		cr.Conclusion = "failure"
	case "pending":
		cr.Status = "in_progress"
		cr.Conclusion = ""
	default:
		cr.Status = "completed"
		cr.Conclusion = s.State
	}

	return cr
}
