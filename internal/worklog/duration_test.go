package worklog

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"1h30m", 90 * time.Minute},
		{"1h 30m", 90 * time.Minute},
		{"1.5h", 90 * time.Minute},
		{"0.25h", 15 * time.Minute},
		{"45m", 45 * time.Minute},
		{"90m", 90 * time.Minute},
		{" 2H ", 2 * time.Hour},
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if err != nil {
			t.Errorf("ParseDuration(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationInvalid(t *testing.T) {
	for _, in := range []string{"", "abc", "1x", "h", "1m30h", "-1h", "0m", "1.5m"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) expected error", in)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h 30m"},
		{8 * time.Hour, "8h"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.in); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
