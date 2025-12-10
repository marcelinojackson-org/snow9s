package models

import (
	"fmt"
	"strings"
	"time"
)

// Service represents an SPCS service record surfaced in the UI.
type Service struct {
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	ComputePool string    `json:"computePool"`
	CreatedAt   time.Time `json:"createdAt"`
	Age         string    `json:"age"`
}

const (
	StatusRunning   = "running"
	StatusStarting  = "starting"
	StatusStopped   = "stopped"
	StatusSuspended = "suspended"
)

// FormatAge renders durations in the terse style used by k9s (s, m, h, d, w).
func FormatAge(d time.Duration) string {
	if d < time.Minute {
		seconds := int(d.Seconds())
		if seconds < 0 {
			seconds = 0
		}
		return formatUnit(seconds, "s")
	}
	if d < time.Hour {
		return formatUnit(int(d.Minutes()), "m")
	}
	if d < 24*time.Hour {
		return formatUnit(int(d.Hours()), "h")
	}
	if d < 7*24*time.Hour {
		return formatUnit(int(d.Hours()/24), "d")
	}
	return formatUnit(int(d.Hours()/(24*7)), "w")
}

func formatUnit(value int, suffix string) string {
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%d%s", value, suffix)
}

// HumanizeAge converts a creation timestamp into the k9s-style age.
func HumanizeAge(created time.Time) string {
	if created.IsZero() {
		return ""
	}
	return FormatAge(time.Since(created))
}

// MatchesFilter reports whether any field contains the filter value (case-insensitive).
func (s Service) MatchesFilter(filter string) bool {
	if filter == "" {
		return true
	}
	needle := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(s.Name), needle) ||
		strings.Contains(strings.ToLower(s.Namespace), needle) ||
		strings.Contains(strings.ToLower(s.Status), needle) ||
		strings.Contains(strings.ToLower(s.ComputePool), needle)
}
