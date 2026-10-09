package worklog

import (
	"testing"
	"time"
)

func TestParseStartTime(t *testing.T) {
	// Friday, October 9 2026 at 4:45pm
	now := time.Date(2026, 10, 9, 16, 45, 30, 0, time.Local)
	at := func(month time.Month, day, h, m int) time.Time {
		return time.Date(2026, month, day, h, m, 0, 0, time.Local)
	}

	tests := []struct {
		in   string
		want time.Time
	}{
		// Time only, today
		{"", at(10, 9, 16, 45)},
		{"  ", at(10, 9, 16, 45)},
		{"9:30", at(10, 9, 9, 30)},
		{"09:30", at(10, 9, 9, 30)},
		{"13:05", at(10, 9, 13, 5)},
		{"9:30am", at(10, 9, 9, 30)},
		{"9:30 AM", at(10, 9, 9, 30)},
		{"2:15pm", at(10, 9, 14, 15)},
		{"2pm", at(10, 9, 14, 0)},
		{"12am", at(10, 9, 0, 0)},
		{"12pm", at(10, 9, 12, 0)},

		// Day only, keeps the current time of day
		{"today", at(10, 9, 16, 45)},
		{"yesterday", at(10, 8, 16, 45)},
		{"Yesterday", at(10, 8, 16, 45)},
		{"friday", at(10, 9, 16, 45)},
		{"mon", at(10, 5, 16, 45)},
		{"monday", at(10, 5, 16, 45)},
		{"sat", at(10, 3, 16, 45)},
		{"10/09", at(10, 9, 16, 45)},
		{"9/30", at(9, 30, 16, 45)},
		{"9/30/2026", at(9, 30, 16, 45)},
		{"2026-09-30", at(9, 30, 16, 45)},

		// Day and time
		{"yesterday 9am", at(10, 8, 9, 0)},
		{"mon 2:30 pm", at(10, 5, 14, 30)},
		{"10/07 13:00", at(10, 7, 13, 0)},
	}
	for _, tt := range tests {
		got, err := ParseStartTime(tt.in, now)
		if err != nil {
			t.Errorf("ParseStartTime(%q) error: %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseStartTime(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseStartTimeInvalid(t *testing.T) {
	now := time.Now()
	for _, in := range []string{
		"soon", "25:00", "9:60", "13pm", "9.30", "9",
		"13/01", "2/30", "10/09/26x", "funday", "yesterday later", "mon tue",
	} {
		if _, err := ParseStartTime(in, now); err == nil {
			t.Errorf("ParseStartTime(%q) expected error", in)
		}
	}
}
