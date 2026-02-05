package auth

import (
	"os"
	"path/filepath"
	"strings"
)

// TokenStore handles persistent storage of GitHub OAuth tokens
type TokenStore struct {
	path string
}

// NewTokenStore creates a new token store in ~/.config/hubell/token
func NewTokenStore() *TokenStore {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		configDir = filepath.Join(home, ".config")
	}

	tokenPath := filepath.Join(configDir, "hubell", "token")
	return &TokenStore{path: tokenPath}
}

// Load reads the token from disk, returns empty string if not found
func (ts *TokenStore) Load() string {
	data, err := os.ReadFile(ts.path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Save writes the token to disk with 0600 permissions
func (ts *TokenStore) Save(token string) error {
	// Ensure directory exists
	dir := filepath.Dir(ts.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write token with restricted permissions
	return os.WriteFile(ts.path, []byte(token+"\n"), 0600)
}

// Delete removes the token file
func (ts *TokenStore) Delete() error {
	return os.Remove(ts.path)
}
