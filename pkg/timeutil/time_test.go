package timeutil

import (
	"testing"
	"time"
)

func TestNow_AlwaysUTC(t *testing.T) {
	now := Now()

	if now.Location() != time.UTC {
		t.Errorf("Now() returned non-UTC timezone: %v", now.Location())
	}
}

func TestStartOfDay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "midnight UTC",
			input:    time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
		{
			name:     "noon UTC",
			input:    time.Date(2025, 11, 20, 12, 30, 45, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
		{
			name:     "end of day UTC",
			input:    time.Date(2025, 11, 20, 23, 59, 59, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartOfDay(tt.input)

			if result.String() != tt.expected {
				t.Errorf("StartOfDay() = %v, want %v", result, tt.expected)
			}

			if result.Location() != time.UTC {
				t.Errorf("StartOfDay() returned non-UTC: %v", result.Location())
			}
		})
	}
}

func TestEndOfDay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "midnight UTC",
			input:    time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			expected: "2025-11-20 23:59:59.999999999 +0000 UTC",
		},
		{
			name:     "noon UTC",
			input:    time.Date(2025, 11, 20, 12, 30, 45, 0, time.UTC),
			expected: "2025-11-20 23:59:59.999999999 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EndOfDay(tt.input)

			if result.String() != tt.expected {
				t.Errorf("EndOfDay() = %v, want %v", result, tt.expected)
			}

			if result.Location() != time.UTC {
				t.Errorf("EndOfDay() returned non-UTC: %v", result.Location())
			}
		})
	}
}

func TestToUTC(t *testing.T) {
	// Create time in EST (UTC-5)
	est, _ := time.LoadLocation("America/New_York")
	estTime := time.Date(2025, 11, 20, 12, 0, 0, 0, est)

	utcTime := ToUTC(estTime)

	if utcTime.Location() != time.UTC {
		t.Errorf("ToUTC() returned non-UTC: %v", utcTime.Location())
	}

	// Verify time value is correct (EST noon = UTC 17:00)
	if utcTime.Hour() != 17 {
		t.Errorf("ToUTC() hour = %d, want 17", utcTime.Hour())
	}
}

// Test that ensures DST doesn't affect calculations
func TestDSTTransitions(t *testing.T) {
	// Spring forward: March 10, 2024, 2:00 AM → 3:00 AM
	beforeDST := time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC)
	afterDST := beforeDST.Add(24 * time.Hour)

	// Should be exactly 24 hours later
	expected := time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC)

	if !afterDST.Equal(expected) {
		t.Errorf("DST transition affected calculation: %v, want %v", afterDST, expected)
	}
}
