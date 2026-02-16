package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

// FetchOrgActivity fetches org-wide activity stats for the overview table.
func (c *Client) FetchOrgActivity(ctx context.Context, org string) ([]OrgMemberActivity, error) {
	since := time.Now().AddDate(0, 0, -7)

	members, err := c.ListOrgMembers(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		memberSet[strings.ToLower(m.Login)] = true
	}

	mergedItems, err := c.SearchOrgMergedPRs(ctx, org, since)
	if err != nil {
		return nil, fmt.Errorf("search merged PRs: %w", err)
	}

	openItems, err := c.SearchOrgOpenPRs(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("search open PRs: %w", err)
	}

	activity := make(map[string]*OrgMemberActivity)

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
			Owner:    owner,
			Repo:     repo,
			Number:   item.Number,
			Title:    item.Title,
			URL:      item.HTMLURL,
			MergedAt: mergedAt,
		})
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
			Owner:  owner,
			Repo:   repo,
			Number: item.Number,
			Title:  item.Title,
			URL:    item.HTMLURL,
		})
	}

	var result []OrgMemberActivity
	for _, a := range activity {
		if len(a.MergedPRs) > 0 || len(a.OpenPRs) > 0 {
			result = append(result, *a)
		}
	}

	// Default sort: most merged PRs first
	sort.Slice(result, func(i, j int) bool {
		return len(result[i].MergedPRs) > len(result[j].MergedPRs)
	})

	return result, nil
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
			detail.DailyActivity[int(mergedAt.Weekday())]++
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
	}

	// Comments given (PRs commented on, not authored)
	q = fmt.Sprintf("org:%s+type:pr+commenter:%s+-author:%s+updated:>=%s", org, login, login, sinceStr)
	commentItems, _ := c.searchAllPages(ctx, q)
	detail.CommentsGiven = len(commentItems)

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
