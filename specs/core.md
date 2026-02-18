# Hubell - Core Specification

## Overview

Hubell is a terminal user interface (TUI) application for managing GitHub notifications and monitoring pull requests. It provides an interactive dashboard for viewing, filtering, and acting on GitHub notifications and open PRs directly from the terminal.

## Architecture

**Language:** Go (1.24)
**TUI Framework:** Bubble Tea
**Styling:** Lipgloss for layout and theming, Bubbles for pre-built list components

The application follows the MVU pattern:

- **Model** (`internal/tui/model.go`) - Application state: notification lists, PR lists, filter mode, theme, dashboard state, loading progress
- **Update** (`internal/tui/update.go`) - Processes keyboard input and async messages (poll results, errors, loading progress)
- **View** (`internal/tui/view.go`) - Renders the two-pane layout, overlays (theme selector, dashboard), loading banner

## Packages

### `internal/github`

GitHub REST API v3 client and polling system.

- **`client.go`** - HTTP client wrapping the GitHub API. Handles authentication (Bearer token), notification fetching with `If-Modified-Since` caching, PR search (open and merged), check runs, commit statuses, and reviews.
- **`poller.go`** - Periodic polling orchestrator (30s default interval). Runs in a goroutine, sends results to a channel consumed by the TUI. First poll backfills 12 weeks of merge history. Emits progress updates for loading UI.
- **`pr_status.go`** - Aggregates check runs and legacy commit statuses into a unified PR status (none/pending/success/failure). Computes review state per PR (approved/changes_requested/reviewed/none) by tracking the latest state per reviewer.
- **`types.go`** - Data models: `Notification`, `PullRequest`, `PRInfo`, `CheckRun`, `Review`, `MergedPRInfo`, status enums.
- **`url.go`** - Converts GitHub API URLs to web URLs for browser opening.

### `internal/tui`

Terminal UI components.

- **`model.go`** - Main Bubble Tea model. Dual-pane layout with notification list (left) and open PR list (right). Manages filter mode, theme, dashboard state, and loading progress.
- **`update.go`** - Keyboard handling (`tab`, `enter`, `r`/`m`, `f`, `d`, `t`, `q`) and poll result integration.
- **`view.go`** - Renders two-pane layout, loading banner with pulsing animation, progress checklist with spinner, and help footer.
- **`pr_delegate.go`** - Custom list item renderer for PRs. Displays CI badge, review state badge, individual check run dots (up to 10), and diff stats.
- **`dashboard.go`** - Activity dashboard overlay. Shows 12-week merged PR bar chart, review latency, CI pass rate, notification volume by age bucket.
- **`barchart.go`** - ASCII block-style bar chart with dynamic scaling and current-week highlighting.
- **`theme.go`** - 8 built-in themes: default, nord, dracula, catppuccin, solarized, gruvbox, tokyonight, rosepine. Persistent theme preference.
- **`messages.go`** - Bubble Tea message types: `PollResultMsg`, `ErrorMsg`, `MarkAsReadMsg`, `BannerTickMsg`, `LoadingProgressMsg`.

### `internal/auth`

- **`token.go`** - Reads/writes GitHub token to `~/.config/hubell/token` (0600 permissions). Respects `XDG_CONFIG_HOME`.
- **`auth.go`** - Interactive token prompt. Links user to GitHub token creation page.

### `internal/config`

- **`config.go`** - Theme preference persistence (`~/.config/hubell/theme`).
- **`weekly_stats.go`** - JSON-based weekly merged PR count cache (`~/.config/hubell/weekly_stats.json`). ISO week keys (`2026-W07`), auto-prunes entries older than 26 weeks.

### `internal/browser`

- **`browser.go`** - Cross-platform browser opening (macOS: `open`, Linux: `xdg-open`, Windows: `cmd /c start`).

### `internal/notify`

- **`osc.go`** - Desktop notifications via OSC 777 escape sequences. Tmux-aware escaping. Falls back to stdout if `/dev/tty` unavailable.

## GitHub API Usage

**Base URL:** `https://api.github.com`
**Auth:** Bearer token, `application/vnd.github+json`, API version `2022-11-28`

| Endpoint | Purpose |
|---|---|
| `GET /notifications` | Fetch notification threads (uses `If-Modified-Since` for 304 caching) |
| `GET /user` | Get authenticated user |
| `GET /user/issues` | Open PRs in private repos |
| `GET /search/issues` | Open PRs in external/fork repos |
| `GET /repos/{o}/{r}/pulls/{n}` | PR detail (additions, deletions) |
| `GET /repos/{o}/{r}/pulls/{n}/reviews` | PR reviews |
| `GET /repos/{o}/{r}/commits/{sha}/check-runs` | Modern CI check runs |
| `GET /repos/{o}/{r}/commits/{sha}/status` | Legacy CI statuses (converted to CheckRun format) |
| `PATCH /notifications/threads/{id}` | Mark notification as read |

Open PR fetching deduplicates results from `/user/issues` and `/search/issues` by HTML URL.

## UI Layout

```
+----------------------------------+----------------------------------+
|  Notifications                   |  Open Pull Requests              |
|                                  |                                  |
|  [filterable list]               |  [list with CI badges,           |
|  - My PRs / All toggle (f)       |   review state, check dots,      |
|                                  |   diff stats]                    |
|                                  |                                  |
+----------------------------------+----------------------------------+
  tab: switch pane | enter: open | r: mark read | f: filter | d: dashboard | t: theme | q: quit
```

**Overlays:** Theme selector, Activity dashboard

**Loading state:** Animated banner with progress checklist and per-PR progress bars during initial poll.

## Storage

All config lives in `~/.config/hubell/` (or `$XDG_CONFIG_HOME/hubell/`):

| File | Format | Purpose |
|---|---|---|
| `token` | Plaintext (0600) | GitHub personal access token |
| `theme` | Plaintext | Selected theme name |
| `weekly_stats.json` | JSON | Cached weekly merged PR counts |

## Key Design Decisions

- **Polling over webhooks** - Simpler deployment (no server needed), uses `If-Modified-Since` to minimize API calls.
- **Dual API sources for PRs** - `/user/issues` covers private repos, `/search/issues` covers external contributions. Results are deduplicated.
- **Legacy CI compat** - Converts old `CommitStatus` API responses to modern `CheckRun` format so the UI has a single rendering path.
- **Review state per reviewer** - Takes the latest review per user, supports dismiss overrides. Priority: changes_requested > approved > commented > none.
- **OSC 777 notifications** - Works in kitty, iTerm2, and other modern terminals. Tmux-aware escaping for nested sessions.
- **Persistent weekly stats** - Avoids re-fetching historical merge data on every startup. Backfills 12 weeks on first poll, prunes data older than 26 weeks.
