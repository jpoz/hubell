package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptForToken prompts the user to create and enter a GitHub Personal Access Token
func PromptForToken() (string, error) {
	fmt.Println("\n=== GitHub Personal Access Token Required ===")
	fmt.Println("\nTo create a token:")
	fmt.Println("  1. Visit: https://github.com/settings/tokens/new")
	fmt.Println("  2. Add a note (e.g., 'hubell')")
	fmt.Println("  3. Select scope: 'notifications'")
	fmt.Println("  4. Click 'Generate token'")
	fmt.Println("  5. Copy the token and paste it below")
	fmt.Println("\nAlternatively, set the GITHUB_TOKEN environment variable.")
	fmt.Print("\nEnter your GitHub token: ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	return token, nil
}
