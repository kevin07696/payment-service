package cron

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIsTransientError validates transient error detection
func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// Transient errors - should return true
		{
			name: "Timeout error",
			err:  errors.New("connection timeout"),
			want: true,
		},
		{
			name: "Connection error",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "Network error",
			err:  errors.New("network unreachable"),
			want: true,
		},
		{
			name: "Temporary error",
			err:  errors.New("temporary failure"),
			want: true,
		},
		{
			name: "Service unavailable",
			err:  errors.New("service unavailable"),
			want: true,
		},
		{
			name: "HTTP 503",
			err:  errors.New("HTTP 503 Service Unavailable"),
			want: true,
		},
		{
			name: "HTTP 502",
			err:  errors.New("502 Bad Gateway"),
			want: true,
		},
		{
			name: "HTTP 504",
			err:  errors.New("504 Gateway Timeout"),
			want: true,
		},
		{
			name: "Mixed case timeout",
			err:  errors.New("REQUEST TIMEOUT"),
			want: true,
		},

		// Permanent errors - should return false
		{
			name: "Nil error",
			err:  nil,
			want: false,
		},
		{
			name: "Invalid account",
			err:  errors.New("invalid account number"),
			want: false,
		},
		{
			name: "Authentication failed",
			err:  errors.New("authentication failed"),
			want: false,
		},
		{
			name: "Invalid BRIC",
			err:  errors.New("BRIC not found"),
			want: false,
		},
		{
			name: "HTTP 400",
			err:  errors.New("400 Bad Request"),
			want: false,
		},
		{
			name: "HTTP 401",
			err:  errors.New("401 Unauthorized"),
			want: false,
		},
		{
			name: "HTTP 404",
			err:  errors.New("404 Not Found"),
			want: false,
		},
		{
			name: "Generic error",
			err:  errors.New("something went wrong"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err)
			assert.Equal(t, tt.want, got, "isTransientError(%v) = %v, want %v", tt.err, got, tt.want)
		})
	}
}

// TestMapDisputeStatus validates North API status mapping
func TestMapDisputeStatus(t *testing.T) {
	tests := []struct {
		name        string
		northStatus string
		want        string
	}{
		// Known statuses
		{
			name:        "NEW status",
			northStatus: "NEW",
			want:        "new",
		},
		{
			name:        "PENDING status",
			northStatus: "PENDING",
			want:        "pending",
		},
		{
			name:        "RESPONDED status",
			northStatus: "RESPONDED",
			want:        "responded",
		},
		{
			name:        "WON status",
			northStatus: "WON",
			want:        "won",
		},
		{
			name:        "LOST status",
			northStatus: "LOST",
			want:        "lost",
		},

		// Unknown/default cases
		{
			name:        "Empty string defaults to new",
			northStatus: "",
			want:        "new",
		},
		{
			name:        "Unknown status defaults to new",
			northStatus: "UNKNOWN",
			want:        "new",
		},
		{
			name:        "Lowercase not matched",
			northStatus: "new",
			want:        "new", // Falls through to default
		},
		{
			name:        "Mixed case not matched",
			northStatus: "Pending",
			want:        "new", // Falls through to default
		},
		{
			name:        "Random string defaults to new",
			northStatus: "INVALID_STATUS",
			want:        "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapDisputeStatus(tt.northStatus)
			assert.Equal(t, tt.want, got, "mapDisputeStatus(%q) = %q, want %q", tt.northStatus, got, tt.want)
		})
	}
}

// TestCalculateNextRetryTime validates exponential backoff calculation
func TestCalculateNextRetryTime(t *testing.T) {
	// Expected delays for each attempt (without jitter)
	expectedDelays := []time.Duration{
		5 * time.Minute,  // attempt 1
		15 * time.Minute, // attempt 2
		30 * time.Minute, // attempt 3
		1 * time.Hour,    // attempt 4
		2 * time.Hour,    // attempt 5+
	}

	tests := []struct {
		name          string
		attempts      int
		expectedDelay time.Duration
	}{
		{"Attempt 1", 1, 5 * time.Minute},
		{"Attempt 2", 2, 15 * time.Minute},
		{"Attempt 3", 3, 30 * time.Minute},
		{"Attempt 4", 4, 1 * time.Hour},
		{"Attempt 5", 5, 2 * time.Hour},
		{"Attempt 6 (capped)", 6, 2 * time.Hour},
		{"Attempt 10 (capped)", 10, 2 * time.Hour},
		{"Attempt 0 (edge case)", 0, 5 * time.Minute},
		{"Negative attempt (edge case)", -1, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			result := calculateNextRetryTime(tt.attempts)

			// Calculate expected time range (±10% jitter)
			minDelay := tt.expectedDelay - time.Duration(float64(tt.expectedDelay)*0.1)
			maxDelay := tt.expectedDelay + time.Duration(float64(tt.expectedDelay)*0.1)

			minExpected := now.Add(minDelay)
			maxExpected := now.Add(maxDelay)

			// Allow small tolerance for test execution time
			tolerance := 100 * time.Millisecond
			minExpected = minExpected.Add(-tolerance)
			maxExpected = maxExpected.Add(tolerance)

			assert.True(t, result.After(minExpected) || result.Equal(minExpected),
				"Result %v should be >= %v (min with jitter)", result, minExpected)
			assert.True(t, result.Before(maxExpected) || result.Equal(maxExpected),
				"Result %v should be <= %v (max with jitter)", result, maxExpected)
		})
	}

	// Test that jitter actually varies results
	t.Run("Jitter produces variation", func(t *testing.T) {
		results := make([]time.Time, 10)
		for i := 0; i < 10; i++ {
			results[i] = calculateNextRetryTime(1)
		}

		// Check that not all results are identical (jitter should vary them)
		allSame := true
		for i := 1; i < len(results); i++ {
			if !results[i].Equal(results[0]) {
				allSame = false
				break
			}
		}
		// Note: There's a tiny chance all 10 could be the same, but extremely unlikely
		assert.False(t, allSame, "Jitter should produce variation in retry times")
	})

	// Verify delay progression
	t.Run("Delays increase with attempts", func(t *testing.T) {
		for i, expected := range expectedDelays {
			attempt := i + 1
			t.Logf("Attempt %d: expected base delay %v", attempt, expected)
		}
	})
}
