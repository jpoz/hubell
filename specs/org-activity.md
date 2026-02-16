# Org Activity Dashboard - Feature Spec

## Overview

A new TUI overlay that provides organization-wide activity visibility. The user specifies a GitHub org (e.g. `angellist`) and sees a ranked table of active engineers with key metrics. Selecting an engineer opens a detailed drill-down view showing their full week of activity.

## Motivation

The existing dashboard (`d` key) shows the authenticated user's personal stats. This feature expands Hubell to be an engineering team visibility tool - answering "what's happening across the org?" and "what has this person been working on?"

## UX Flow

```
Main TUI (two-pane layout)
  │
  ├─ Press `o` ──▶ Org Activity Overlay
  │                  ┌────────────────────────────────────────────────┐
  │                  │  angellist - Org Activity (last 7 days)       │
  │                  │                                                │
  │                  │  Engineer        Merged  Open  Reviews  +/-    │
  │                  │  ─────────────────────────────────────────────  │
  │                  │▸ @alice            12      3       8   +1.2k   │
  │                  │  @bob               9      5       4   +800    │
  │                  │  @carol             7      2      11   +2.1k   │
  │                  │  @dave              6      1       6   +450    │
  │                  │  @eve               4      4       3   +1.5k   │
  │                  │  ...                                           │
  │                  │                                                │
  │                  │  Total: 24 engineers active  │  82 PRs merged  │
  │                  │  esc: back  enter: drill down  s: sort col     │
  │                  └────────────────────────────────────────────────┘
  │                    │
  │                    ├─ Press `enter` ──▶ Engineer Drill-Down
  │                    │                     ┌──────────────────────────────────────────┐
  │                    │                     │  @alice - Last 7 Days                    │
  │                    │                     │                                          │
  │                    │                     │  PRs Merged (12)          Reviewed (8)   │
  │                    │                     │  ────────────────────     ──────────────  │
  │                    │                     │  repo-a#401  +120 -30    repo-b#220      │
  │                    │                     │  repo-a#398  +45 -12     repo-c#115      │
  │                    │                     │  repo-b#217  +200 -80    repo-a#399      │
  │                    │                     │  ...                     ...              │
  │                    │                     │                                          │
  │                    │                     │  ┌─ Daily Activity ─────────────────┐    │
  │                    │                     │  │  Mon ████████  5                 │    │
  │                    │                     │  │  Tue ███████████  7              │    │
  │                    │                     │  │  Wed ████  2                     │    │
  │                    │                     │  │  Thu ██████████████  9            │    │
  │                    │                     │  │  Fri ██████  3                   │    │
  │                    │                     │  └──────────────────────────────────┘    │
  │                    │                     │                                          │
  │                    │                     │  Stats                                   │
  │                    │                     │  ──────────────────────────────────────   │
  │                    │                     │  Avg PR Size:  +85 / -28                 │
  │                    │                     │  Avg Time to Merge:  4h 12m              │
  │                    │                     │  Repos Touched:  repo-a, repo-b, repo-c  │
  │                    │                     │  Longest PR:  repo-a#401 (2d 3h)         │
  │                    │                     │  Comments Given:  23                     │
  │                    │                     │  Comments Received:  18                  │
  │                    │                     │                                          │
  │                    │                     │  esc: back  enter: open PR in browser    │
  │                    │                     └──────────────────────────────────────────┘
  │                    │
  │                    ├─ Press `esc` ──▶ Back to main TUI
```

## Configuration

The org name is configured via:

1. **CLI flag:** `hubell --org angellist`
2. **Config file:** `~/.config/hubell/config.json` with `{"org": "angellist"}`
3. **Environment variable:** `HUBELL_ORG=angellist`

Priority: CLI flag > env var > config file.

When no org is configured and user presses `o`, show a one-line text input prompt: "Enter GitHub org name:" that saves to the config file for future sessions.

## Data Model

### New Types (`internal/github/types.go`)

```go
// OrgMember represents a member of a GitHub organization
type OrgMember struct {
    Login     string `json:"login"`
    AvatarURL string `json:"avatar_url"`
}

// OrgMemberActivity holds aggregated activity stats for one org member
type OrgMemberActivity struct {
    Login           string
    MergedPRs       []MergedPRInfo    // PRs merged in the time window
    OpenPRs         []SearchItem      // Currently open PRs
    ReviewedPRs     []ReviewedPRInfo  // PRs reviewed (not authored) in the time window
    Comments        int               // Total PR/issue comments in the time window
    Additions       int               // Total lines added across merged PRs
    Deletions       int               // Total lines removed across merged PRs
}

// ReviewedPRInfo contains metadata about a PR that was reviewed by this user
type ReviewedPRInfo struct {
    Owner    string
    Repo     string
    Number   int
    Title    string
    URL      string
    Author   string
}

// EngineerDetail holds the full drill-down data for a single engineer
type EngineerDetail struct {
    Login              string
    MergedPRs          []DetailedMergedPR
    OpenPRs            []DetailedOpenPR
    ReviewedPRs        []ReviewedPRInfo
    DailyMergedCounts  map[time.Weekday]int  // Mon-Fri merged PR counts
    DailyEventCounts   map[time.Weekday]int  // Mon-Fri total events (merged + reviewed + commented)
    AvgAdditions       int
    AvgDeletions       int
    AvgTimeToMerge     time.Duration
    LongestPR          *DetailedMergedPR     // PR with longest open-to-merge duration
    ReposContributed   []string              // Unique repo names
    CommentsGiven      int                   // PR review comments authored
    CommentsReceived   int                   // PR review comments received on own PRs
}

// DetailedMergedPR extends MergedPRInfo with diff stats and timing
type DetailedMergedPR struct {
    Owner         string
    Repo          string
    Number        int
    Title         string
    URL           string
    MergedAt      time.Time
    CreatedAt     time.Time
    Additions     int
    Deletions     int
    TimeToMerge   time.Duration  // MergedAt - CreatedAt
}

// DetailedOpenPR extends open PR info with diff stats
type DetailedOpenPR struct {
    Owner       string
    Repo        string
    Number      int
    Title       string
    URL         string
    CreatedAt   time.Time
    Additions   int
    Deletions   int
    Age         time.Duration
}
```

### New TUI State (`internal/tui/model.go`)

```go
// Add to Model struct:
showOrgDashboard    bool
orgName             string
orgMembers          []github.OrgMemberActivity
orgSelectedIndex    int
orgSortColumn       OrgSortColumn
orgLoading          bool
orgError            error

showEngineerDetail  bool
engineerDetail      *github.EngineerDetail
engineerLoading     bool
engineerSelectedPR  int  // cursor index in the merged PR list
```

```go
type OrgSortColumn int

const (
    SortByMerged OrgSortColumn = iota
    SortByOpen
    SortByReviews
    SortByLinesChanged
)
```

## GitHub API Endpoints

### Org Member List

```
GET /orgs/{org}/members?per_page=100
```

Returns all public members. Requires the token to have `read:org` scope for private member visibility. Paginate if the org has >100 members.

### Merged PRs Per Member (last 7 days)

```
GET /search/issues?q=org:{org}+type:pr+is:merged+author:{user}+merged:>={date}&per_page=100
```

One request per member. To minimize API calls, batch by fetching all org merged PRs first, then bucketing by author:

```
GET /search/issues?q=org:{org}+type:pr+is:merged+merged:>={date}&sort=updated&order=desc&per_page=100
```

Paginate if >100 results per page (up to 1000 total from Search API).

### Open PRs Per Member

```
GET /search/issues?q=org:{org}+type:pr+state:open+author:{user}&per_page=100
```

Or batch: fetch all open PRs in org and bucket by author:

```
GET /search/issues?q=org:{org}+type:pr+state:open&per_page=100
```

### Reviews Given (last 7 days)

```
GET /search/issues?q=org:{org}+type:pr+is:merged+reviewed-by:{user}+merged:>={date}&per_page=100
```

The `reviewed-by` qualifier filters PRs that were reviewed by the user. Subtract self-authored PRs to get reviews of others' work.

### PR Detail (for drill-down diff stats and timing)

```
GET /repos/{owner}/{repo}/pulls/{number}
```

Returns `additions`, `deletions`, `created_at`, `merged_at`. Needed for per-PR detail in the engineer drill-down.

### PR Review Comments (for drill-down comment counts)

```
GET /repos/{owner}/{repo}/pulls/{number}/comments?per_page=100
```

Count comments authored by the engineer (comments given) vs comments on the engineer's own PRs (comments received).

Alternative (more efficient): use the search-based approach:

```
GET /search/issues?q=org:{org}+type:pr+commenter:{user}+updated:>={date}&per_page=100
```

This gives an approximate comment count (number of PRs commented on, not individual comments).

## API Call Strategy

### Rate Limit Awareness

GitHub Search API is limited to 30 requests/minute. The strategy optimizes for minimal API calls:

**Phase 1: Org Overview (triggered on `o` press)**

| Step | Endpoint | Calls | Purpose |
|------|----------|-------|---------|
| 1 | `GET /orgs/{org}/members` | 1-2 | Get member list (paginated) |
| 2 | `GET /search/issues?q=org:{org}+type:pr+is:merged+merged:>={7d}` | 1-3 | All merged PRs, bucket by author |
| 3 | `GET /search/issues?q=org:{org}+type:pr+state:open` | 1-3 | All open PRs, bucket by author |
| 4 | `GET /search/issues?q=org:{org}+type:pr+is:merged+merged:>={7d}` (with review info) | 0 | Reviews extracted from step 2 results |

Total: ~4-8 API calls for the overview. Results cached for the session (refresh with `r`).

For lines changed in the overview (+/- column), sum `additions`/`deletions` from the search result items. The Search API does not return these fields directly, so we fetch PR details in the background:

| Step | Endpoint | Calls | Purpose |
|------|----------|-------|---------|
| 5 | `GET /repos/{o}/{r}/pulls/{n}` | N (async) | Diff stats per merged PR |

Display "+/-" as "..." until the background fetches complete, then update in place.

**Phase 2: Engineer Drill-Down (triggered on `enter`)**

| Step | Endpoint | Calls | Purpose |
|------|----------|-------|---------|
| 1 | `GET /repos/{o}/{r}/pulls/{n}` | N | Detail for each of the engineer's merged PRs (if not already cached from phase 1) |
| 2 | `GET /search/issues?q=org:{org}+type:pr+reviewed-by:{user}+-author:{user}+merged:>={7d}` | 1 | PRs reviewed by this engineer |
| 3 | `GET /search/issues?q=org:{org}+type:pr+commenter:{user}+updated:>={7d}` | 1 | Approximate comment activity |

Total: ~2 + N API calls (where N = PRs not yet cached).

### Caching

- Org member list: cached for the session, refreshed on manual `r` press.
- Merged/open PR search results: cached for 5 minutes, refreshed on manual `r`.
- PR detail (additions/deletions): cached for the session (PR details don't change after merge).
- Engineer drill-down data: cached per-engineer for 5 minutes.

Cache stored in-memory only (no disk persistence for org data - it's always fresh on restart).

## UI Components

### Org Overview Table

A scrollable table rendered with Lipgloss. Columns:

| Column | Width | Description | Sort |
|--------|-------|-------------|------|
| Engineer | 20 | `@login` | Alpha |
| Merged | 8 | PRs merged in last 7 days | Desc (default) |
| Open | 6 | Currently open PRs | Desc |
| Reviews | 8 | PRs reviewed (not self-authored) | Desc |
| +/- | 10 | Total lines changed (additions + deletions) | Desc |

The table supports:
- Arrow keys to navigate rows
- `s` to cycle sort column
- `enter` to drill down into selected engineer
- `r` to refresh data
- `esc` to close overlay

Footer shows summary: "Total: N engineers active | M PRs merged"

### Engineer Drill-Down View

A scrollable view with sections. Uses the existing bar chart component for daily activity.

**Sections:**

1. **Header:** `@login - Last 7 Days`

2. **Two-column PR lists:**
   - Left: "PRs Merged (N)" - list of merged PRs with `repo#number +add -del`
   - Right: "Reviewed (N)" - list of PRs reviewed with `repo#number`
   - Arrow keys navigate the merged PR list; `enter` opens in browser

3. **Daily Activity Bar Chart:**
   - Horizontal bar chart (Mon-Fri)
   - Bars show total events per day (merges + reviews + comments)
   - Reuses the existing `barchart.go` rendering (adapted for horizontal layout)

4. **Stats Section:**
   | Stat | Description |
   |------|-------------|
   | Avg PR Size | Mean additions/deletions across merged PRs |
   | Avg Time to Merge | Mean duration from PR creation to merge |
   | Repos Touched | Unique repos with merged or open PRs |
   | Longest PR | PR with longest creation-to-merge duration |
   | Comments Given | PR review comments authored on others' PRs |
   | Comments Received | PR review comments received on own PRs |

**Navigation:**
- `esc` returns to org overview
- `enter` opens the selected PR in the browser
- Arrow keys scroll through the view / PR list

## Loading States

### Org Overview Loading

```
┌────────────────────────────────────────┐
│  Loading angellist org activity...     │
│                                        │
│  ⠋ Fetching members                   │
│  ⠋ Fetching merged PRs (last 7 days)  │
│  ⠋ Fetching open PRs                  │
│    Fetching PR details (12/47)         │
└────────────────────────────────────────┘
```

Each step shows a spinner while in-progress, a checkmark when complete. PR detail fetching shows a progress counter.

### Engineer Drill-Down Loading

```
┌──────────────────────────────────────┐
│  Loading @alice details...           │
│                                      │
│  ⠋ Fetching PR details              │
│  ⠋ Fetching review activity         │
└──────────────────────────────────────┘
```

## Keyboard Bindings

| Key | Context | Action |
|-----|---------|--------|
| `o` | Main TUI | Open org activity overlay |
| `esc` | Org overview | Close overlay, return to main TUI |
| `esc` | Engineer drill-down | Return to org overview |
| `enter` | Org overview | Drill down into selected engineer |
| `enter` | Engineer drill-down | Open selected PR in browser |
| `↑/↓` | Org overview | Navigate engineer list |
| `↑/↓` | Engineer drill-down | Scroll view / navigate PR list |
| `s` | Org overview | Cycle sort column |
| `r` | Org overview | Refresh data |

## New Files

| File | Purpose |
|------|---------|
| `internal/github/org.go` | Org-specific API methods (list members, fetch org PRs, fetch engineer detail) |
| `internal/tui/org_dashboard.go` | Org overview overlay rendering and state |
| `internal/tui/engineer_detail.go` | Engineer drill-down overlay rendering |
| `internal/config/org.go` | Org name persistence in config file |

## Modified Files

| File | Changes |
|------|---------|
| `internal/github/types.go` | Add `OrgMember`, `OrgMemberActivity`, `ReviewedPRInfo`, `EngineerDetail`, `DetailedMergedPR`, `DetailedOpenPR` types |
| `internal/tui/model.go` | Add org dashboard state fields to `Model` struct |
| `internal/tui/update.go` | Handle `o` key, org navigation keys, async org data messages |
| `internal/tui/view.go` | Render org overlay when `showOrgDashboard` or `showEngineerDetail` is true |
| `internal/tui/messages.go` | Add `OrgDataMsg`, `EngineerDetailMsg`, `OrgErrorMsg` message types |
| `main.go` | Accept `--org` flag, pass org name to TUI model |

## New Bubble Tea Messages

```go
// OrgDataMsg delivers org overview data to the TUI
type OrgDataMsg struct {
    Members []github.OrgMemberActivity
}

// OrgPRDetailMsg delivers background-fetched PR diff stats
type OrgPRDetailMsg struct {
    PRKey     string  // "owner/repo#number"
    Additions int
    Deletions int
}

// EngineerDetailMsg delivers drill-down data for a single engineer
type EngineerDetailMsg struct {
    Detail *github.EngineerDetail
}

// OrgErrorMsg reports an error from org data fetching
type OrgErrorMsg struct {
    Err error
}
```

## Token Scope Requirements

The feature requires the token to have:

- `repo` - For accessing private repo PRs (already required by existing features)
- `read:org` - For listing org members (new requirement)

If the token lacks `read:org`, the member list API returns 403. In that case, display a helpful error: "Token needs `read:org` scope. Update at https://github.com/settings/tokens"

## Edge Cases

- **Large orgs (100+ members):** Paginate member list. Only show members with activity in the last 7 days (filter out inactive members from the table).
- **Search API 1000-result limit:** GitHub Search API returns max 1000 results. For very active orgs, results may be truncated. Show a note: "Showing top 1000 results" if the search `total_count` exceeds 1000.
- **Rate limiting:** If rate-limited (403/429), show the rate limit reset time and retry after the window. Queue API calls and process sequentially with small delays between batches.
- **Bot accounts:** Filter out known bot patterns from the engineer list (logins ending in `[bot]`, `dependabot`, `renovate`, etc.).
- **No org configured:** Pressing `o` with no org set shows a text input prompt. Saving persists to config file.
- **Private org:** If the authenticated user is not a member, the members API returns 404. Show: "Not a member of this org or org not found."

## Future Enhancements (out of scope)

- Multi-org support (switch between orgs)
- Custom time windows (last 14 days, last 30 days)
- Export to CSV/JSON
- Compare engineers side-by-side
- Trend arrows (up/down vs previous period)
- Slack/email summary generation
