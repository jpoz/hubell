package config

import (
	"os"
	"path/filepath"
	"strings"
)

func themePath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "hubell", "theme")
}

// LoadTheme reads the saved theme name from disk. Returns empty string if not found.
func LoadTheme() string {
	p := themePath()
	if p == "" {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveTheme writes the theme name to disk.
func SaveTheme(name string) error {
	p := themePath()
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(name+"\n"), 0600)
}
