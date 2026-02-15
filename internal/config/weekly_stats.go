package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WeekKey returns an ISO week key like "2026-W07" for the given time.
func WeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// WeeklyStats holds per-week merged PR counts.
type WeeklyStats struct {
	Weeks map[string]int `json:"weeks"`
}

func weeklyStatsPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "hubell", "weekly_stats.json")
}

// LoadWeeklyStats reads cached weekly stats from disk. Returns empty stats on error.
func LoadWeeklyStats() WeeklyStats {
	p := weeklyStatsPath()
	if p == "" {
		return WeeklyStats{Weeks: make(map[string]int)}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return WeeklyStats{Weeks: make(map[string]int)}
	}
	var stats WeeklyStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return WeeklyStats{Weeks: make(map[string]int)}
	}
	if stats.Weeks == nil {
		stats.Weeks = make(map[string]int)
	}
	return stats
}

// SaveWeeklyStats writes weekly stats to disk, pruning entries older than 26 weeks.
func SaveWeeklyStats(stats WeeklyStats) error {
	p := weeklyStatsPath()
	if p == "" {
		return nil
	}

	// Prune entries older than 26 weeks
	cutoff := time.Now().AddDate(0, 0, -26*7)
	cutoffKey := WeekKey(cutoff)
	for k := range stats.Weeks {
		if k < cutoffKey {
			delete(stats.Weeks, k)
		}
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
