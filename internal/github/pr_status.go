package github

import (
	"context"
	"fmt"
	"strings"
)

// pollAllPRs fetches all open PRs and their CI statuses.
// If progressCh is non-nil, per-PR progress updates are sent on it.
func pollAllPRs(ctx context.Context, client *Client, username string, progressCh chan<- LoadingProgress) (map[string]PRStatus, map[string]PRInfo, error) {
	searchResult, err := client.SearchUserOpenPRs(ctx, username)
	if err != nil {
		return nil, nil, fmt.Errorf("searching open PRs: %w", err)
	}

	total := len(searchResult.Items)
	statuses := make(map[string]PRStatus)
	infos := make(map[string]PRInfo)

	for i, item := range searchResult.Items {
		if progressCh != nil {
			progressCh <- LoadingProgress{Step: StepPullRequests, Current: i, Total: total}
		}
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" || repo == "" {
			continue
		}

		key := PRKey(owner, repo, item.Number)
		infos[key] = PRInfo{
			Owner:     owner,
			Repo:      repo,
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.HTMLURL,
			CreatedAt: item.CreatedAt,
		}

		pr, err := client.GetPullRequest(ctx, owner, repo, item.Number)
		if err != nil {
			statuses[key] = PRStatusNone
			continue
		}

		checkRuns, err := client.GetCheckRuns(ctx, owner, repo, pr.Head.SHA)
		if err != nil {
			statuses[key] = PRStatusNone
			continue
		}

		// Fetch legacy commit statuses and merge them in as synthetic CheckRuns
		// so the display layer handles them uniformly.
		commitStatus, err := client.GetCommitStatus(ctx, owner, repo, pr.Head.SHA)
		if err == nil {
			for _, s := range commitStatus.Statuses {
				checkRuns.CheckRuns = append(checkRuns.CheckRuns, statusToCheckRun(s))
				checkRuns.TotalCount++
			}
		}

		statuses[key] = computeAggregateStatus(checkRuns)

		// Populate diff stats and check runs from already-fetched data
		info := infos[key]
		info.Additions = pr.Additions
		info.Deletions = pr.Deletions
		info.CheckRuns = checkRuns.CheckRuns

		reviews, err := client.GetPullRequestReviews(ctx, owner, repo, item.Number)
		if err == nil {
			info.ReviewState = computeReviewState(reviews)
			info.Reviews = reviews
		}
		infos[key] = info
	}

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
