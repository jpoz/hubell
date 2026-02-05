package github

import (
	"strings"
)

// ConvertAPIURLToWeb converts a GitHub API URL to a web URL
// Example: https://api.github.com/repos/owner/repo/issues/123
//       -> https://github.com/owner/repo/issues/123
func ConvertAPIURLToWeb(apiURL string) string {
	// Replace api.github.com/repos/ with github.com/
	webURL := strings.Replace(apiURL, "https://api.github.com/repos/", "https://github.com/", 1)

	// Handle pulls -> pull (GitHub uses 'pull' in web URLs, 'pulls' in API)
	webURL = strings.Replace(webURL, "/pulls/", "/pull/", 1)

	return webURL
}
