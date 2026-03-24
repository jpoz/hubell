package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ListOrgMembers fetches all members of a GitHub organization.
func (c *Client) ListOrgMembers(ctx context.Context, org string) ([]OrgMember, error) {
	var all []OrgMember
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/orgs/%s/members?per_page=100&page=%d", baseURL, org, page)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, fmt.Errorf("token needs read:org scope to list members. Update at https://github.com/settings/tokens")
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, fmt.Errorf("org %q not found or not accessible", org)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("list org members: status %d", resp.StatusCode)
		}

		var members []OrgMember
		if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode org members: %w", err)
		}
		resp.Body.Close()

		all = append(all, members...)
		if len(members) < 100 {
			break
		}
	}
	return all, nil
}

// SearchOrgMergedPRs fetches all merged PRs in an org since the given date.
func (c *Client) SearchOrgMergedPRs(ctx context.Context, org string, since time.Time) ([]SearchItem, error) {
	sinceStr := since.Format("2006-01-02")
	q := fmt.Sprintf("org:%s+type:pr+is:merged+merged:>=%s", org, sinceStr)
	return c.searchAllPages(ctx, q)
}

// SearchOrgOpenPRs fetches all open PRs in an org.
func (c *Client) SearchOrgOpenPRs(ctx context.Context, org string) ([]SearchItem, error) {
	q := fmt.Sprintf("org:%s+type:pr+state:open", org)
	return c.searchAllPages(ctx, q)
}

func reportOrgLoading(progressCh chan<- OrgLoadingProgress, step OrgLoadingStep, startedAt time.Time, detail string, current, total int, done bool) {
	if progressCh == nil {
		return
	}
	progressCh <- OrgLoadingProgress{
		Step:      step,
		Detail:    detail,
		Current:   current,
		Total:     total,
		StartedAt: startedAt,
		UpdatedAt: time.Now(),
		Done:      done,
	}
}

func shouldReportOrgProgress(current, total int) bool {
	if total <= 0 {
		return current <= 1
	}
	if current == 0 || current == 1 || current == total {
		return true
	}
	if total <= 25 {
		return true
	}
	return current%5 == 0
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

// searchAllPages performs a paginated search, up to 1000 results (GitHub limit).
func (c *Client) searchAllPages(ctx context.Context, query string) ([]SearchItem, error) {
	var all []SearchItem
	for page := 1; page <= 10; page++ {
		u := fmt.Sprintf("%s/search/issues?q=%s&sort=updated&order=desc&per_page=100&page=%d", baseURL, query, page)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnprocessableEntity {
			resp.Body.Close()
			return nil, fmt.Errorf("search validation failed")
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("search: status %d", resp.StatusCode)
		}

		var result SearchResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode search: %w", err)
		}
		resp.Body.Close()

		all = append(all, result.Items...)
		if len(all) >= result.TotalCount || len(result.Items) < 100 {
			break
		}
	}
	return all, nil
}

// SearchOrgCommits returns a map of login -> commit count for the org since the given date.
func (c *Client) SearchOrgCommits(ctx context.Context, org string, since time.Time) (map[string]int, error) {
	sinceStr := since.Format("2006-01-02")
	q := fmt.Sprintf("org:%s+author-date:>=%s", org, sinceStr)

	counts := make(map[string]int)
	for page := 1; page <= 10; page++ {
		u := fmt.Sprintf("%s/search/commits?q=%s&sort=author-date&order=desc&per_page=100&page=%d", baseURL, q, page)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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
			return nil, fmt.Errorf("search commits: status %d", resp.StatusCode)
		}

		var result struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				Author *struct {
					Login string `json:"login"`
				} `json:"author"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode commit search: %w", err)
		}
		resp.Body.Close()

		for _, item := range result.Items {
			if item.Author != nil && item.Author.Login != "" {
				counts[strings.ToLower(item.Author.Login)]++
			}
		}

		if len(result.Items) < 100 {
			break
		}
	}
	return counts, nil
}

// SearchOrgReviewCounts returns a map of login -> review count for the org since the given date.
// It searches for PRs reviewed by each member concurrently.
func (c *Client) SearchOrgReviewCounts(ctx context.Context, org string, members []OrgMember, since time.Time, progress func(current, total int)) map[string]int {
	sinceStr := since.Format("2006-01-02")
	type result struct {
		login string
		count int
	}
	ch := make(chan result, len(members))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var doneCount int32

	for _, m := range members {
		wg.Add(1)
		go func(login string) {
			defer wg.Done()
			defer func() {
				if progress == nil {
					return
				}
				current := int(atomic.AddInt32(&doneCount, 1))
				if shouldReportOrgProgress(current, len(members)) {
					progress(current, len(members))
				}
			}()
			sem <- struct{}{}
			defer func() { <-sem }()

			q := fmt.Sprintf("org:%s+type:pr+reviewed-by:%s+-author:%s+updated:>=%s", org, login, login, sinceStr)
			u := fmt.Sprintf("%s/search/issues?q=%s&per_page=1", baseURL, q)
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				return
			}
			c.setHeaders(req)
			resp, err := c.httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			var sr struct {
				TotalCount int `json:"total_count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
				return
			}
			if sr.TotalCount > 0 {
				ch <- result{login: strings.ToLower(login), count: sr.TotalCount}
			}
		}(m.Login)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	counts := make(map[string]int)
	for r := range ch {
		counts[r.login] = r.count
	}
	return counts
}

// FetchOrgActivity fetches org-wide activity stats for the overview table.
func (c *Client) FetchOrgActivity(ctx context.Context, org string) ([]OrgMemberActivity, error) {
	members, _, err := c.FetchOrgActivityWithProgress(ctx, org, nil)
	return members, err
}

// FetchOrgActivityWithProgress fetches org-wide activity stats and reports loading progress.
func (c *Client) FetchOrgActivityWithProgress(ctx context.Context, org string, progressCh chan<- OrgLoadingProgress) ([]OrgMemberActivity, OrgActivitySummary, error) {
	overallStart := time.Now()
	since := time.Now().AddDate(0, 0, -7)
	summary := OrgActivitySummary{}

	membersStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepMembers, membersStartedAt, "Listing organization members", 0, 0, false)
	members, err := c.ListOrgMembers(ctx, org)
	if err != nil {
		return nil, summary, fmt.Errorf("list members: %w", err)
	}
	summary.Members = len(members)
	reportOrgLoading(progressCh, OrgStepMembers, membersStartedAt, fmt.Sprintf("%d members found", len(members)), len(members), len(members), true)

	mergedStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepMergedPRs, mergedStartedAt, "Searching merged pull requests from the last 7 days", 0, 0, false)
	mergedItems, err := c.SearchOrgMergedPRs(ctx, org, since)
	if err != nil {
		return nil, summary, fmt.Errorf("search merged PRs: %w", err)
	}
	summary.MergedPRs = len(mergedItems)
	reportOrgLoading(progressCh, OrgStepMergedPRs, mergedStartedAt, fmt.Sprintf("%d merged PRs found", len(mergedItems)), len(mergedItems), len(mergedItems), true)

	openStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepOpenPRs, openStartedAt, "Searching open pull requests", 0, 0, false)
	openItems, err := c.SearchOrgOpenPRs(ctx, org)
	if err != nil {
		return nil, summary, fmt.Errorf("search open PRs: %w", err)
	}
	summary.OpenPRs = len(openItems)
	reportOrgLoading(progressCh, OrgStepOpenPRs, openStartedAt, fmt.Sprintf("%d open PRs found", len(openItems)), len(openItems), len(openItems), true)

	// Fetch commit counts (best-effort, don't fail the whole operation)
	commitsStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepCommits, commitsStartedAt, "Counting authored commits", 0, 0, false)
	commitCounts, commitErr := c.SearchOrgCommits(ctx, org, since)
	if commitCounts == nil {
		commitCounts = make(map[string]int)
	}
	summary.Commits = sumCounts(commitCounts)
	commitDetail := fmt.Sprintf("%d commits attributed", summary.Commits)
	if commitErr != nil {
		commitDetail = "Commit counts unavailable"
	}
	reportOrgLoading(progressCh, OrgStepCommits, commitsStartedAt, commitDetail, summary.Commits, summary.Commits, true)

	// Fetch review counts (best-effort)
	reviewsStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepReviews, reviewsStartedAt, fmt.Sprintf("Checking review activity across %d engineers", len(members)), 0, len(members), false)
	reviewCounts := c.SearchOrgReviewCounts(ctx, org, members, since, func(current, total int) {
		reportOrgLoading(progressCh, OrgStepReviews, reviewsStartedAt, fmt.Sprintf("Checked %d/%d engineers", current, total), current, total, false)
	})
	summary.Reviews = sumCounts(reviewCounts)
	summary.ReviewedEngineers = len(reviewCounts)
	reportOrgLoading(progressCh, OrgStepReviews, reviewsStartedAt, fmt.Sprintf("%d reviews across %d engineers", summary.Reviews, summary.ReviewedEngineers), len(members), len(members), true)

	activity := make(map[string]*OrgMemberActivity)

	// Track merged PRs for LOC fetching
	type prRef struct {
		owner, repo, login string
		number             int
	}
	var mergedPRRefs []prRef

	for _, item := range mergedItems {
		login := item.User.Login
		if login == "" || isBot(login) {
			continue
		}
		a, ok := activity[login]
		if !ok {
			a = &OrgMemberActivity{Login: login}
			activity[login] = a
		}
		owner, repo := parseRepoURL(item.RepositoryURL)
		mergedAt := time.Time{}
		if item.ClosedAt != nil {
			mergedAt = *item.ClosedAt
		}
		a.MergedPRs = append(a.MergedPRs, MergedPRInfo{
			Owner:     owner,
			Repo:      repo,
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.HTMLURL,
			Author:    login,
			CreatedAt: item.CreatedAt,
			MergedAt:  mergedAt,
		})
		mergedPRRefs = append(mergedPRRefs, prRef{owner: owner, repo: repo, login: login, number: item.Number})
	}

	for _, item := range openItems {
		login := item.User.Login
		if login == "" || isBot(login) {
			continue
		}
		a, ok := activity[login]
		if !ok {
			a = &OrgMemberActivity{Login: login}
			activity[login] = a
		}
		owner, repo := parseRepoURL(item.RepositoryURL)
		a.OpenPRs = append(a.OpenPRs, MergedPRInfo{
			Owner:     owner,
			Repo:      repo,
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.HTMLURL,
			Author:    login,
			CreatedAt: item.CreatedAt,
		})
	}

	// Fetch LOC for merged PRs concurrently
	diffStatsStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepDiffStats, diffStatsStartedAt, fmt.Sprintf("Fetching diff stats for %d merged PRs", len(mergedPRRefs)), 0, len(mergedPRRefs), false)
	type locResult struct {
		login     string
		additions int
		deletions int
	}
	locCh := make(chan locResult, len(mergedPRRefs))
	sem := make(chan struct{}, 10) // limit concurrency
	var wg sync.WaitGroup
	var locDoneCount int32
	for _, ref := range mergedPRRefs {
		wg.Add(1)
		go func(r prRef) {
			defer wg.Done()
			defer func() {
				current := int(atomic.AddInt32(&locDoneCount, 1))
				if shouldReportOrgProgress(current, len(mergedPRRefs)) {
					reportOrgLoading(progressCh, OrgStepDiffStats, diffStatsStartedAt, fmt.Sprintf("Fetched %d/%d PR diffs", current, len(mergedPRRefs)), current, len(mergedPRRefs), false)
				}
			}()
			sem <- struct{}{}
			defer func() { <-sem }()
			pr, err := c.GetPullRequest(ctx, r.owner, r.repo, r.number)
			if err == nil {
				locCh <- locResult{login: r.login, additions: pr.Additions, deletions: pr.Deletions}
			}
		}(ref)
	}
	go func() {
		wg.Wait()
		close(locCh)
	}()
	fetchedDiffStats := 0
	for lr := range locCh {
		if a, ok := activity[lr.login]; ok {
			a.Additions += lr.additions
			a.Deletions += lr.deletions
		}
		fetchedDiffStats++
	}
	summary.DiffStatsFetched = fetchedDiffStats
	diffDetail := fmt.Sprintf("Diff stats fetched for %d/%d merged PRs", fetchedDiffStats, len(mergedPRRefs))
	if len(mergedPRRefs) == 0 {
		diffDetail = "No merged PR diffs to inspect"
	}
	reportOrgLoading(progressCh, OrgStepDiffStats, diffStatsStartedAt, diffDetail, len(mergedPRRefs), len(mergedPRRefs), true)

	// Assign commit and review counts
	aggregateStartedAt := time.Now()
	reportOrgLoading(progressCh, OrgStepAggregate, aggregateStartedAt, "Ranking active engineers", 0, 0, false)
	for login, a := range activity {
		lower := strings.ToLower(login)
		a.Commits = commitCounts[lower]
		a.Reviews = reviewCounts[lower]
	}

	// Ensure members with only reviews also appear
	for login, count := range reviewCounts {
		if _, ok := activity[login]; !ok && count > 0 {
			// Find the original-case login
			for _, m := range members {
				if strings.ToLower(m.Login) == login {
					activity[login] = &OrgMemberActivity{Login: m.Login, Reviews: count}
					break
				}
			}
		}
	}

	var result []OrgMemberActivity
	totalLOC := 0
	for _, a := range activity {
		if len(a.MergedPRs) > 0 || len(a.OpenPRs) > 0 || a.Commits > 0 || a.Reviews > 0 {
			result = append(result, *a)
			totalLOC += a.Additions + a.Deletions
		}
	}

	// Default sort: most commits first
	sort.Slice(result, func(i, j int) bool {
		return result[i].Commits > result[j].Commits
	})

	summary.ActiveEngineers = len(result)
	summary.LOC = totalLOC
	summary.Duration = time.Since(overallStart)
	reportOrgLoading(progressCh, OrgStepAggregate, aggregateStartedAt, fmt.Sprintf("%d active engineers ranked", len(result)), len(result), len(result), true)

	return result, summary, nil
}

// FetchEngineerDetail fetches detailed activity for a single engineer.
func (c *Client) FetchEngineerDetail(ctx context.Context, org, login string) (*EngineerDetail, error) {
	since := time.Now().AddDate(0, 0, -7)
	sinceStr := since.Format("2006-01-02")

	detail := &EngineerDetail{
		Login: login,
	}

	// Fetch merged PRs by this user in the org
	q := fmt.Sprintf("org:%s+type:pr+is:merged+author:%s+merged:>=%s", org, login, sinceStr)
	mergedItems, err := c.searchAllPages(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search merged PRs: %w", err)
	}

	repoSet := make(map[string]bool)
	var totalAdditions, totalDeletions int
	var totalMergeDuration time.Duration
	var longestDuration time.Duration

	for _, item := range mergedItems {
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" {
			continue
		}

		mergedAt := time.Time{}
		if item.ClosedAt != nil {
			mergedAt = *item.ClosedAt
		}

		d := DetailedMergedPR{
			Owner:     owner,
			Repo:      repo,
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.HTMLURL,
			MergedAt:  mergedAt,
			CreatedAt: item.CreatedAt,
		}

		// Fetch PR detail for diff stats
		pr, err := c.GetPullRequest(ctx, owner, repo, item.Number)
		if err == nil {
			d.Additions = pr.Additions
			d.Deletions = pr.Deletions
			totalAdditions += pr.Additions
			totalDeletions += pr.Deletions
		}

		if !mergedAt.IsZero() && !item.CreatedAt.IsZero() {
			d.TimeToMerge = mergedAt.Sub(item.CreatedAt)
			totalMergeDuration += d.TimeToMerge
			if d.TimeToMerge > longestDuration {
				longestDuration = d.TimeToMerge
				longest := d
				detail.LongestPR = &longest
			}
		}

		repoSet[owner+"/"+repo] = true

		if !mergedAt.IsZero() {
			detail.DailyMerges[int(mergedAt.Weekday())]++
		}

		detail.MergedPRs = append(detail.MergedPRs, d)
	}

	if len(detail.MergedPRs) > 0 {
		detail.AvgAdditions = totalAdditions / len(detail.MergedPRs)
		detail.AvgDeletions = totalDeletions / len(detail.MergedPRs)
		detail.AvgTimeToMerge = totalMergeDuration / time.Duration(len(detail.MergedPRs))
	}

	// Fetch open PRs
	q = fmt.Sprintf("org:%s+type:pr+state:open+author:%s", org, login)
	openItems, _ := c.searchAllPages(ctx, q)
	for _, item := range openItems {
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" {
			continue
		}
		d := DetailedOpenPR{
			Owner:     owner,
			Repo:      repo,
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.HTMLURL,
			CreatedAt: item.CreatedAt,
			Age:       time.Since(item.CreatedAt),
		}
		pr, err := c.GetPullRequest(ctx, owner, repo, item.Number)
		if err == nil {
			d.Additions = pr.Additions
			d.Deletions = pr.Deletions
		}
		detail.OpenPRs = append(detail.OpenPRs, d)
		repoSet[owner+"/"+repo] = true
	}

	// Fetch reviews given (PRs reviewed but not authored)
	q = fmt.Sprintf("org:%s+type:pr+reviewed-by:%s+-author:%s+updated:>=%s", org, login, login, sinceStr)
	reviewedItems, _ := c.searchAllPages(ctx, q)
	for _, item := range reviewedItems {
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" {
			continue
		}
		detail.ReviewedPRs = append(detail.ReviewedPRs, ReviewedPRInfo{
			Owner:  owner,
			Repo:   repo,
			Number: item.Number,
			Title:  item.Title,
			URL:    item.HTMLURL,
			Author: item.User.Login,
		})

		// Fetch individual reviews for daily activity tracking
		reviews, err := c.GetPullRequestReviews(ctx, owner, repo, item.Number)
		if err == nil {
			for _, r := range reviews {
				if strings.EqualFold(r.User.Login, login) && !r.SubmittedAt.Before(since) {
					detail.DailyReviews[int(r.SubmittedAt.Weekday())]++
				}
			}
		}
	}

	// Comments given (PRs commented on, not authored)
	q = fmt.Sprintf("org:%s+type:pr+commenter:%s+-author:%s+updated:>=%s", org, login, login, sinceStr)
	commentItems, _ := c.searchAllPages(ctx, q)
	for _, item := range commentItems {
		owner, repo := parseRepoURL(item.RepositoryURL)
		if owner == "" {
			continue
		}
		comments, err := c.GetIssueComments(ctx, owner, repo, item.Number, since)
		if err == nil {
			for _, comment := range comments {
				if strings.EqualFold(comment.User.Login, login) {
					detail.CommentsGiven++
					detail.DailyComments[int(comment.CreatedAt.Weekday())]++
				}
			}
		}
	}

	// Comments received (other people commenting on user's PRs)
	q = fmt.Sprintf("org:%s+type:pr+author:%s+comments:>0+updated:>=%s", org, login, sinceStr)
	receivedItems, _ := c.searchAllPages(ctx, q)
	detail.CommentsReceived = len(receivedItems)

	for repo := range repoSet {
		detail.ReposContributed = append(detail.ReposContributed, repo)
	}
	sort.Strings(detail.ReposContributed)

	return detail, nil
}

// isBot returns true if the login appears to be a bot account.
func isBot(login string) bool {
	lower := strings.ToLower(login)
	if strings.HasSuffix(lower, "[bot]") || strings.HasSuffix(lower, "-bot") {
		return true
	}
	bots := []string{"dependabot", "renovate", "greenkeeper", "snyk-bot", "codecov", "coveralls"}
	for _, b := range bots {
		if lower == b {
			return true
		}
	}
	return false
}
