package worklog

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Matches 1h30m, 1h 30m, 1.5h, 2h, 90m
var durationPattern = regexp.MustCompile(`^(?:(\d+(?:\.\d+)?)h)?\s*(?:(\d+)m)?$`)

// Turns user input like "1h 30m" into a time.Duration
func ParseDuration(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	m := durationPattern.FindStringSubmatch(s)
	if s == "" || m == nil {
		return 0, fmt.Errorf("invalid duration %q (try 1h 30m, 1.5h, or 45m)", s)
	}

	var d time.Duration
	if m[1] != "" {
		hours, _ := strconv.ParseFloat(m[1], 64)
		d += time.Duration(hours * float64(time.Hour))
	}
	if m[2] != "" {
		mins, _ := strconv.Atoi(m[2])
		d += time.Duration(mins) * time.Minute
	}

	d = d.Round(time.Minute)
	if d <= 0 {
		return 0, errors.New("duration must be at least 1m")
	}
	return d, nil
}

// Formats a duration the way Jira expects timeSpent value, ex 1h 30m
func FormatDuration(d time.Duration) string {
	mins := int(math.Round(d.Minutes()))
	h, m := mins/60, mins%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
