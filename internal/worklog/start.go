package worklog

import (
	"fmt"
	"strings"
	"time"
)

// Accepted clock formats, checked after spaces are removed and input is lowercased
var clockLayouts = []string{"15:04", "3:04pm", "3pm"}

// Accepted date formats: month/day (this year), month/day/year, and ISO
var dateLayouts = []string{"1/2", "1/2/2006", "2006-01-02"}

// Accepted weekday names, mapped to time.Weekday values
var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// Turns input like "9:30am", "yesterday", "mon 2pm", or "10/09 9:30" into a start time
// The day defaults to today and the time defaults to the current time of day
func ParseStartTime(s string, now time.Time) (time.Time, error) {
	words := strings.Fields(strings.ToLower(s))
	invalid := fmt.Errorf("invalid start %q (try 9:30am, yesterday, mon 2pm, or 10/09 9:30)", strings.TrimSpace(s))

	day := now
	if len(words) > 0 {
		d, isDay, err := parseDay(words[0], now) // Check if the first word is a day
		if err != nil {
			return time.Time{}, invalid
		}
		if isDay {
			day, words = d, words[1:] // Remove the day from the words
		}
	}

	hour, minute := now.Hour(), now.Minute()
	if len(words) > 0 {
		clock, err := parseClock(strings.Join(words, "")) // Join the remaining words and parse as a clock
		if err != nil {
			return time.Time{}, invalid
		}
		hour, minute = clock.Hour(), clock.Minute()
	}

	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location()), nil
}

// Reports whether word is a day (today, yesterday, a weekday, or a date) and returns it
// A weekday means the most recent one, counting today
func parseDay(word string, now time.Time) (time.Time, bool, error) {
	switch word {
	case "today":
		return now, true, nil
	case "yesterday":
		return now.AddDate(0, 0, -1), true, nil
	}

	if wd, ok := weekdays[word]; ok {
		daysAgo := (int(now.Weekday()) - int(wd) + 7) % 7
		return now.AddDate(0, 0, -daysAgo), true, nil
	}

	// Anything else with a date separator must be a valid date
	if !strings.ContainsAny(word, "/-") {
		return time.Time{}, false, nil
	}
	for _, layout := range dateLayouts {
		d, err := time.Parse(layout, word)
		if err != nil {
			continue
		}
		year := d.Year()
		if layout == "1/2" {
			year = now.Year()
		}
		return time.Date(year, d.Month(), d.Day(), 0, 0, 0, 0, now.Location()), true, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid date %q", word)
}

// Parses a clock string like "9:30am" or "17:05" into a time.Time with the current date
// The date is ignored, only the hour and minute are used
func parseClock(s string) (time.Time, error) {
	for _, layout := range clockLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", s)
}
