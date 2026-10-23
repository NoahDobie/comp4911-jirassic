package worklog

import (
	"testing"
	"time"
)

func TestDayIndex(t *testing.T) {
	tests := []struct {
		day  int // October 2026, the 12th is a Monday
		want int
	}{
		{12, 0}, {14, 2}, {16, 4}, {18, 6},
	}
	for _, tt := range tests {
		if got := DayIndex(time.Date(2026, 10, tt.day, 9, 0, 0, 0, time.Local)); got != tt.want {
			t.Errorf("DayIndex(Oct %d) = %d, want %d", tt.day, got, tt.want)
		}
	}
}

func TestRecentRange(t *testing.T) {
	now := time.Date(2026, 10, 14, 13, 30, 0, 0, time.Local)
	start, end := RecentRange(now, 14)
	wantStart := time.Date(2026, 9, 30, 0, 0, 0, 0, time.Local)
	wantEnd := time.Date(2026, 10, 15, 0, 0, 0, 0, time.Local)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Errorf("RecentRange = %v - %v, want %v - %v", start, end, wantStart, wantEnd)
	}
}

func TestWeekStart(t *testing.T) {
	monday := time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)
	tests := []time.Time{
		time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local),    // Monday midnight
		time.Date(2026, 10, 14, 13, 30, 0, 0, time.Local),  // Wednesday
		time.Date(2026, 10, 18, 23, 59, 59, 0, time.Local), // Sunday night
	}
	for _, in := range tests {
		if got := WeekStart(in); !got.Equal(monday) {
			t.Errorf("WeekStart(%v) = %v, want %v", in, got, monday)
		}
	}
}

func TestWeekStartAcrossMonth(t *testing.T) {
	// Thursday Oct 1 2026 belongs to the week starting Monday Sep 28
	got := WeekStart(time.Date(2026, 10, 1, 9, 0, 0, 0, time.Local))
	want := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWeekRange(t *testing.T) {
	now := time.Date(2026, 10, 14, 13, 30, 0, 0, time.Local)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.Local) }

	tests := []struct {
		offset     int
		start, end time.Time
	}{
		{0, day(10, 12), day(10, 19)},
		{-1, day(10, 5), day(10, 12)},
		{-2, day(9, 28), day(10, 5)},
		{1, day(10, 19), day(10, 26)},
	}
	for _, tt := range tests {
		start, end := WeekRange(now, tt.offset)
		if !start.Equal(tt.start) || !end.Equal(tt.end) {
			t.Errorf("WeekRange(offset %d) = %v - %v, want %v - %v", tt.offset, start, end, tt.start, tt.end)
		}
	}
}
