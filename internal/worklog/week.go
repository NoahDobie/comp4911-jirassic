package worklog

import "time"

// Returns midnight on the Monday of the given week
func WeekStart(t time.Time) time.Time {
	daysSinceMonday := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-daysSinceMonday, 0, 0, 0, 0, t.Location())
}

// Returns the start, end range of the week offset weeks from the given week ex. offset -1 is last week, 0 is this week
func WeekRange(t time.Time, offset int) (start, end time.Time) {
	start = WeekStart(t).AddDate(0, 0, 7*offset)
	return start, start.AddDate(0, 0, 7)
}
