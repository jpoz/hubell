package config

import (
	"os"
	"path/filepath"
	"strings"
)

func orgPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "hubell", "org")
}

// LoadOrg reads the saved org name from disk. Returns empty string if not found.
func LoadOrg() string {
	p := orgPath()
	if p == "" {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveOrg writes the org name to disk.
func SaveOrg(name string) error {
	p := orgPath()
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(name+"\n"), 0600)
}
