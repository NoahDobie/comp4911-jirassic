package worklog

import "time"

// Day of the week counting Monday as 0
func DayIndex(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

// Returns midnight on the Monday of the given week
func WeekStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day()-DayIndex(t), 0, 0, 0, 0, t.Location())
}

// Returns the start, end range of the week offset weeks from the given week ex. offset -1 is last week, 0 is this week
func WeekRange(t time.Time, offset int) (start, end time.Time) {
	start = WeekStart(t).AddDate(0, 0, 7*offset)
	return start, start.AddDate(0, 0, 7)
}

// Returns midnight days ago through the end of today
func RecentRange(now time.Time, days int) (start, end time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.AddDate(0, 0, -days), today.AddDate(0, 0, 1)
}
