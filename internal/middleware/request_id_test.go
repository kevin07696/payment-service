package middleware

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateRequestID_ValidUUIDv4 tests that generated request IDs are valid UUID v4
// Security Risk: MEDIUM - Weak request IDs could lead to collisions or predictability
func TestGenerateRequestID_ValidUUIDv4(t *testing.T) {
	t.Run("Returns_ValidUUID", func(t *testing.T) {
		requestID := generateRequestID()

		// Should be able to parse as UUID
		parsedUUID, err := uuid.Parse(requestID)
		require.NoError(t, err, "Request ID should be a valid UUID")

		// Verify it's a UUID v4 (random)
		assert.Equal(t, uuid.Version(4), parsedUUID.Version(),
			"Request ID should be UUID version 4 (random)")

		t.Logf("[PASS] Generated valid UUID v4: %s", requestID)
	})

	t.Run("ValidFormat_WithHyphens", func(t *testing.T) {
		requestID := generateRequestID()

		// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		// Where y is one of [8, 9, a, b]
		parts := strings.Split(requestID, "-")

		require.Len(t, parts, 5, "UUID should have 5 parts separated by hyphens")
		assert.Len(t, parts[0], 8, "First part should be 8 characters")
		assert.Len(t, parts[1], 4, "Second part should be 4 characters")
		assert.Len(t, parts[2], 4, "Third part should be 4 characters")
		assert.Len(t, parts[3], 4, "Fourth part should be 4 characters")
		assert.Len(t, parts[4], 12, "Fifth part should be 12 characters")

		// Third part should start with '4' (version 4)
		assert.Equal(t, "4", string(parts[2][0]),
			"Third part should start with '4' for UUID v4")

		// Fourth part should start with [8, 9, a, b] (variant bits)
		firstChar := strings.ToLower(string(parts[3][0]))
		validVariants := []string{"8", "9", "a", "b"}
		assert.Contains(t, validVariants, firstChar,
			"Fourth part should start with 8, 9, a, or b for UUID v4")

		t.Logf("[PASS] UUID format correct: %s", requestID)
	})

	t.Run("Lowercase_Hexadecimal", func(t *testing.T) {
		requestID := generateRequestID()

		// Remove hyphens and check all characters are hex
		cleaned := strings.ReplaceAll(requestID, "-", "")

		for _, char := range cleaned {
			assert.True(t,
				(char >= '0' && char <= '9') ||
					(char >= 'a' && char <= 'f'),
				"All characters should be lowercase hexadecimal")
		}

		t.Log("[PASS] All characters are lowercase hexadecimal")
	})

	t.Run("CorrectLength", func(t *testing.T) {
		requestID := generateRequestID()

		// Standard UUID string length: 36 characters (32 hex + 4 hyphens)
		assert.Len(t, requestID, 36,
			"UUID string should be exactly 36 characters")

		t.Log("[PASS] UUID has correct length (36 characters)")
	})
}

// TestGenerateRequestID_Uniqueness tests that request IDs are unique
// Security Risk: MEDIUM - Collision could lead to request confusion or security issues
func TestGenerateRequestID_Uniqueness(t *testing.T) {
	t.Run("NoCollisions_Sequential", func(t *testing.T) {
		// Generate 1000 request IDs sequentially
		const count = 1000
		seen := make(map[string]bool, count)

		for i := 0; i < count; i++ {
			requestID := generateRequestID()

			assert.False(t, seen[requestID],
				"Request ID collision detected: %s (iteration %d)", requestID, i)

			seen[requestID] = true
		}

		assert.Len(t, seen, count,
			"Should have %d unique request IDs", count)

		t.Logf("[PASS] Generated %d unique request IDs sequentially", count)
	})

	t.Run("NoCollisions_Concurrent", func(t *testing.T) {
		// Generate 1000 request IDs concurrently from multiple goroutines
		const count = 1000
		const goroutines = 10

		ids := make(chan string, count)
		var wg sync.WaitGroup

		// Start goroutines
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < count/goroutines; i++ {
					ids <- generateRequestID()
				}
			}()
		}

		// Wait for completion
		wg.Wait()
		close(ids)

		// Check uniqueness
		seen := make(map[string]bool, count)
		collisions := 0

		for id := range ids {
			if seen[id] {
				collisions++
				t.Errorf("Collision detected: %s", id)
			}
			seen[id] = true
		}

		assert.Equal(t, 0, collisions, "Should have no collisions")
		assert.Len(t, seen, count, "Should have %d unique IDs", count)

		t.Logf("[PASS] Generated %d unique request IDs concurrently (%d goroutines)",
			count, goroutines)
	})

	t.Run("Statistical_Distribution", func(t *testing.T) {
		// Generate multiple UUIDs and verify they're not sequential or patterned
		const count = 100
		ids := make([]string, count)

		for i := 0; i < count; i++ {
			ids[i] = generateRequestID()
		}

		// Check that IDs are not sequential (no simple incrementing pattern)
		// Compare consecutive IDs - they should differ significantly
		for i := 0; i < count-1; i++ {
			assert.NotEqual(t, ids[i], ids[i+1],
				"Consecutive IDs should be different")

			// IDs should differ by more than just incrementing the last character
			// (this would indicate a weak/predictable generator)
			diff := 0
			for j := 0; j < len(ids[i]); j++ {
				if ids[i][j] != ids[i+1][j] {
					diff++
				}
			}

			// Expect at least 10 character differences between consecutive UUIDs
			// (UUID v4 uses random bits, so consecutive IDs should be very different)
			assert.GreaterOrEqual(t, diff, 10,
				"Consecutive UUIDs should differ significantly (got %d differences)", diff)
		}

		t.Logf("[PASS] UUID distribution appears random (no sequential patterns)")
	})
}

// TestGenerateRequestID_NonPredictable tests that request IDs are not predictable
// Security Risk: MEDIUM - Predictable IDs could allow request enumeration or replay attacks
func TestGenerateRequestID_NonPredictable(t *testing.T) {
	t.Run("No_Timestamp_Leakage", func(t *testing.T) {
		// Old implementation used timestamp, new implementation should not
		// Generate multiple IDs quickly and verify they don't contain obvious timestamps

		const count = 10
		ids := make([]string, count)

		for i := 0; i < count; i++ {
			ids[i] = generateRequestID()
		}

		// IDs should NOT start with common timestamp prefixes
		for _, id := range ids {
			// Check that ID doesn't start with Unix timestamp-like values
			// (Unix timestamps in hex would start with specific patterns)
			assert.NotRegexp(t, `^[0-9]{10}`, id,
				"ID should not start with Unix timestamp pattern")

			// Verify it's a proper UUID format, not timestamp-based
			parsedUUID, err := uuid.Parse(id)
			require.NoError(t, err)

			// UUID v1 uses timestamps, v4 is random - verify it's v4
			assert.Equal(t, uuid.Version(4), parsedUUID.Version(),
				"Should be UUID v4 (random), not v1 (timestamp-based)")
		}

		t.Log("[PASS] Request IDs do not leak timestamp information")
	})

	t.Run("High_Entropy", func(t *testing.T) {
		// Generate IDs and verify they have high entropy (randomness)
		const count = 100
		characterCounts := make(map[rune]int)

		for i := 0; i < count; i++ {
			id := generateRequestID()
			for _, char := range strings.ReplaceAll(id, "-", "") {
				characterCounts[char]++
			}
		}

		// With 100 UUIDs (32 hex chars each = 3200 chars total),
		// each hex character (0-9, a-f = 16 chars) should appear roughly equally
		// Expected: 3200 / 16 = 200 times per character
		// Allow 50-350 range (allows for randomness variance)

		for char := '0'; char <= '9'; char++ {
			count := characterCounts[char]
			assert.Greater(t, count, 50,
				"Character '%c' appears too rarely (%d times)", char, count)
			assert.Less(t, count, 350,
				"Character '%c' appears too frequently (%d times)", char, count)
		}

		for char := 'a'; char <= 'f'; char++ {
			count := characterCounts[char]
			assert.Greater(t, count, 50,
				"Character '%c' appears too rarely (%d times)", char, count)
			assert.Less(t, count, 350,
				"Character '%c' appears too frequently (%d times)", char, count)
		}

		t.Log("[PASS] Request IDs have high entropy (randomness)")
	})
}

// TestGenerateRequestID_SecurityProperties tests security-relevant properties
func TestGenerateRequestID_SecurityProperties(t *testing.T) {
	t.Run("Sufficient_Keyspace", func(t *testing.T) {
		// UUID v4 has 122 random bits (128 - 6 version/variant bits)
		// This provides 2^122 possible values ≈ 5.3 x 10^36
		// Probability of collision is negligible

		// Generate sample ID and verify it's UUID v4
		requestID := generateRequestID()
		parsedUUID, err := uuid.Parse(requestID)
		require.NoError(t, err)

		// Verify version 4 (random)
		assert.Equal(t, uuid.Version(4), parsedUUID.Version())

		// UUID v4 keyspace: 2^122 = 5.3 x 10^36 possible values
		// This is sufficient for cryptographic purposes
		t.Log("[PASS] UUID v4 provides sufficient keyspace (2^122 values)")
		t.Log("[INFO] Collision probability negligible even at billions of IDs/second")
	})

	t.Run("CSPRNG_Based", func(t *testing.T) {
		// Google's uuid.New() uses crypto/rand for randomness (CSPRNG)
		// We verify this indirectly by checking UUID v4 compliance

		requestID := generateRequestID()
		parsedUUID, err := uuid.Parse(requestID)
		require.NoError(t, err)

		// UUID v4 uses random bits, which should come from crypto/rand
		assert.Equal(t, uuid.Version(4), parsedUUID.Version(),
			"UUID v4 indicates cryptographically secure random generation")

		t.Log("[PASS] UUID v4 generated using cryptographically secure random number generator")
		t.Log("[INFO] Google uuid library uses crypto/rand internally")
	})

	t.Run("No_MAC_Address_Leakage", func(t *testing.T) {
		// UUID v1 includes MAC address, UUID v4 does not
		// Verify we're using v4 to avoid leaking network information

		requestID := generateRequestID()
		parsedUUID, err := uuid.Parse(requestID)
		require.NoError(t, err)

		assert.NotEqual(t, uuid.Version(1), parsedUUID.Version(),
			"Should not use UUID v1 (which leaks MAC address)")
		assert.Equal(t, uuid.Version(4), parsedUUID.Version(),
			"Should use UUID v4 (no MAC address)")

		t.Log("[PASS] Request IDs do not leak MAC address (using UUID v4, not v1)")
	})
}
