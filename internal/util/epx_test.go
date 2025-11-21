package util

import (
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUUIDToEPXTranNbr_Deterministic verifies that the same UUID always produces the same TRAN_NBR
// Business Rule: Idempotency is critical for EPX integration - same transaction must have same identifier
func TestUUIDToEPXTranNbr_Deterministic(t *testing.T) {
	testCases := []struct {
		name string
		uuid string
	}{
		{
			name: "standard UUID",
			uuid: "12345678-1234-1234-1234-123456789abc",
		},
		{
			name: "different UUID",
			uuid: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		},
		{
			name: "UUID with all lowercase",
			uuid: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		{
			name: "UUID with all uppercase",
			uuid: "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.MustParse(tc.uuid)

			// Call function multiple times with same UUID
			result1 := UUIDToEPXTranNbr(id)
			result2 := UUIDToEPXTranNbr(id)
			result3 := UUIDToEPXTranNbr(id)

			// All results must be identical (deterministic)
			assert.Equal(t, result1, result2, "Second call should produce same result")
			assert.Equal(t, result2, result3, "Third call should produce same result")
			assert.NotEmpty(t, result1, "Result should not be empty")
		})
	}
}

// TestUUIDToEPXTranNbr_Format verifies that output is a numeric string (digits only)
// EPX Requirement: TRAN_NBR must be numeric (no letters, special characters)
func TestUUIDToEPXTranNbr_Format(t *testing.T) {
	testCases := []struct {
		name string
		uuid string
	}{
		{
			name: "standard UUID",
			uuid: "12345678-1234-1234-1234-123456789abc",
		},
		{
			name: "UUID with letters",
			uuid: "abcdefab-cdef-abcd-efab-cdefabcdefab",
		},
		{
			name: "zero UUID",
			uuid: "00000000-0000-0000-0000-000000000000",
		},
		{
			name: "max UUID",
			uuid: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.MustParse(tc.uuid)
			result := UUIDToEPXTranNbr(id)

			// Verify it's a valid numeric string
			_, err := strconv.ParseUint(result, 10, 64)
			require.NoError(t, err, "Result must be a valid numeric string")

			// Verify all characters are digits
			for _, ch := range result {
				assert.True(t, ch >= '0' && ch <= '9', "Result must contain only digits, found: %c", ch)
			}
		})
	}
}

// TestUUIDToEPXTranNbr_Length verifies that output is maximum 10 digits
// EPX Requirement: TRAN_NBR must be max 10 digits (fits uint32 range: 4,294,967,295)
func TestUUIDToEPXTranNbr_Length(t *testing.T) {
	// Test with many random UUIDs to ensure length constraint holds
	testCases := []struct {
		name string
		uuid string
	}{
		{
			name: "standard UUID",
			uuid: "12345678-1234-1234-1234-123456789abc",
		},
		{
			name: "zero UUID",
			uuid: "00000000-0000-0000-0000-000000000000",
		},
		{
			name: "max UUID",
			uuid: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
		{
			name: "random UUID 1",
			uuid: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "random UUID 2",
			uuid: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.MustParse(tc.uuid)
			result := UUIDToEPXTranNbr(id)

			// Verify length is at most 10 digits
			assert.LessOrEqual(t, len(result), 10, "TRAN_NBR must be at most 10 digits")
			assert.Greater(t, len(result), 0, "TRAN_NBR must not be empty")

			// Verify it fits in uint32 (max 4,294,967,295)
			val, err := strconv.ParseUint(result, 10, 32)
			require.NoError(t, err, "Result must fit in uint32")
			assert.LessOrEqual(t, val, uint64(4294967295), "Result must not exceed uint32 max")
		})
	}
}

// TestUUIDToEPXTranNbr_UniqueUUIDs verifies that different UUIDs produce different TRAN_NBRs
// Business Rule: Different transactions should have different identifiers (no collisions)
func TestUUIDToEPXTranNbr_UniqueUUIDs(t *testing.T) {
	uuids := []string{
		"12345678-1234-1234-1234-123456789abc",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"ffffffff-ffff-ffff-ffff-fffffffffffe",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
	}

	results := make(map[string]string)

	for _, uuidStr := range uuids {
		id := uuid.MustParse(uuidStr)
		result := UUIDToEPXTranNbr(id)

		// Check for collisions
		if prevUUID, exists := results[result]; exists {
			t.Errorf("Collision detected! UUID %s and %s both produced TRAN_NBR: %s",
				prevUUID, uuidStr, result)
		}

		results[result] = uuidStr
	}

	// Verify we have unique results for all input UUIDs
	assert.Equal(t, len(uuids), len(results), "All UUIDs should produce unique TRAN_NBRs")
}

// TestUUIDToEPXTranNbr_EdgeCases tests edge cases like nil, zero, and max UUIDs
// Business Rule: Function should handle all valid UUID values correctly
func TestUUIDToEPXTranNbr_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		uuid        string
		description string
	}{
		{
			name:        "nil UUID (all zeros)",
			uuid:        "00000000-0000-0000-0000-000000000000",
			description: "Nil/zero UUID should produce valid TRAN_NBR",
		},
		{
			name:        "max UUID (all ones)",
			uuid:        "ffffffff-ffff-ffff-ffff-ffffffffffff",
			description: "Max UUID should produce valid TRAN_NBR",
		},
		{
			name:        "UUID with single bit set",
			uuid:        "00000000-0000-0000-0000-000000000001",
			description: "Minimal non-zero UUID",
		},
		{
			name:        "UUID with single bit unset",
			uuid:        "ffffffff-ffff-ffff-ffff-fffffffffffe",
			description: "Maximal UUID with one bit unset",
		},
		{
			name:        "UUID with alternating bits",
			uuid:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			description: "UUID with pattern 10101010...",
		},
		{
			name:        "UUID with alternating bits inverted",
			uuid:        "55555555-5555-5555-5555-555555555555",
			description: "UUID with pattern 01010101...",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.MustParse(tc.uuid)
			result := UUIDToEPXTranNbr(id)

			// Verify basic requirements
			assert.NotEmpty(t, result, "Result should not be empty for %s", tc.description)
			assert.LessOrEqual(t, len(result), 10, "Result must be at most 10 digits")

			// Verify numeric format
			val, err := strconv.ParseUint(result, 10, 32)
			require.NoError(t, err, "Result must be valid uint32 for %s", tc.description)
			assert.LessOrEqual(t, val, uint64(4294967295), "Result must fit in uint32")

			// Verify determinism
			result2 := UUIDToEPXTranNbr(id)
			assert.Equal(t, result, result2, "Same UUID should always produce same result")
		})
	}
}

// TestUUIDToEPXTranNbr_CollisionResistance tests for collisions with many random UUIDs
// Business Rule: Hash function should minimize collisions in practical use
func TestUUIDToEPXTranNbr_CollisionResistance(t *testing.T) {
	const numTests = 1000
	results := make(map[string]uuid.UUID)
	collisions := 0

	for i := 0; i < numTests; i++ {
		id := uuid.New() // Generate random UUID
		result := UUIDToEPXTranNbr(id)

		// Check for collision
		if existingUUID, exists := results[result]; exists {
			collisions++
			t.Logf("Collision %d: UUID %s and %s both produced TRAN_NBR: %s",
				collisions, existingUUID.String(), id.String(), result)
		} else {
			results[result] = id
		}

		// Verify format constraints
		assert.LessOrEqual(t, len(result), 10, "Result must be at most 10 digits")
		_, err := strconv.ParseUint(result, 10, 32)
		require.NoError(t, err, "Result must be valid uint32")
	}

	// Calculate collision rate
	collisionRate := float64(collisions) / float64(numTests) * 100

	// With FNV-1a 32-bit and 1000 UUIDs, we expect very few collisions
	// uint32 has ~4.3 billion possible values, so collision probability is very low
	// We allow up to 1% collision rate (10 collisions out of 1000) to be conservative
	assert.LessOrEqual(t, collisionRate, 1.0,
		"Collision rate should be less than 1%% (got %.2f%%, %d collisions out of %d)",
		collisionRate, collisions, numTests)

	t.Logf("Collision test: %d unique TRAN_NBRs from %d UUIDs (%.2f%% collision rate)",
		len(results), numTests, collisionRate)
}

// TestUUIDToEPXTranNbr_Distribution verifies that hash function distributes well across UUID space
// Business Rule: Hash should distribute uniformly to avoid clustering
func TestUUIDToEPXTranNbr_Distribution(t *testing.T) {
	const numTests = 1000
	const numBuckets = 10 // Divide uint32 space into 10 buckets

	buckets := make([]int, numBuckets)
	bucketSize := uint64(4294967295) / uint64(numBuckets)

	for i := 0; i < numTests; i++ {
		id := uuid.New()
		result := UUIDToEPXTranNbr(id)

		val, err := strconv.ParseUint(result, 10, 32)
		require.NoError(t, err)

		// Determine which bucket this value falls into
		bucket := int(val / bucketSize)
		if bucket >= numBuckets {
			bucket = numBuckets - 1 // Handle edge case for max value
		}
		buckets[bucket]++
	}

	// Calculate expected count per bucket (uniform distribution)
	expectedPerBucket := float64(numTests) / float64(numBuckets)

	// Verify distribution is reasonably uniform
	// Allow 50% deviation from expected (statistical tolerance)
	minExpected := expectedPerBucket * 0.5
	maxExpected := expectedPerBucket * 1.5

	for i, count := range buckets {
		assert.GreaterOrEqual(t, float64(count), minExpected,
			"Bucket %d has too few values (got %d, expected ~%.0f)", i, count, expectedPerBucket)
		assert.LessOrEqual(t, float64(count), maxExpected,
			"Bucket %d has too many values (got %d, expected ~%.0f)", i, count, expectedPerBucket)
	}

	t.Logf("Distribution test: %d UUIDs distributed across %d buckets", numTests, numBuckets)
	for i, count := range buckets {
		t.Logf("  Bucket %d: %d values (%.1f%%)", i, count, float64(count)/float64(numTests)*100)
	}
}

// TestUUIDToEPXTranNbr_EPXCompliance verifies all EPX requirements are met
// EPX Requirements:
// - TRAN_NBR must be numeric (digits only)
// - TRAN_NBR must be max 10 digits
// - Same transaction UUID must produce same TRAN_NBR (idempotency)
func TestUUIDToEPXTranNbr_EPXCompliance(t *testing.T) {
	testCases := []struct {
		name string
		uuid string
	}{
		{
			name: "typical transaction UUID",
			uuid: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "another typical UUID",
			uuid: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
		{
			name: "UUID from payment",
			uuid: "12345678-1234-1234-1234-123456789abc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.MustParse(tc.uuid)
			result := UUIDToEPXTranNbr(id)

			// EPX Requirement 1: Must be numeric
			_, err := strconv.ParseUint(result, 10, 64)
			require.NoError(t, err, "EPX requires TRAN_NBR to be numeric")

			for _, ch := range result {
				assert.True(t, ch >= '0' && ch <= '9',
					"EPX requires TRAN_NBR to contain only digits, found: %c", ch)
			}

			// EPX Requirement 2: Must be max 10 digits
			assert.LessOrEqual(t, len(result), 10,
				"EPX requires TRAN_NBR to be max 10 digits (got %d digits)", len(result))

			// Verify it fits in uint32 (4,294,967,295 = 10 digits max)
			val, err := strconv.ParseUint(result, 10, 32)
			require.NoError(t, err, "TRAN_NBR must fit in uint32 for EPX")
			assert.LessOrEqual(t, val, uint64(4294967295),
				"TRAN_NBR must not exceed uint32 max (4,294,967,295)")

			// EPX Requirement 3: Idempotency - same UUID always produces same TRAN_NBR
			result2 := UUIDToEPXTranNbr(id)
			result3 := UUIDToEPXTranNbr(id)
			assert.Equal(t, result, result2,
				"EPX requires idempotency: same UUID must produce same TRAN_NBR")
			assert.Equal(t, result, result3,
				"EPX requires idempotency: same UUID must produce same TRAN_NBR")

			t.Logf("EPX Compliance verified for UUID %s -> TRAN_NBR %s", tc.uuid, result)
		})
	}
}

// TestUUIDToEPXTranNbr_NoLeadingZeros verifies EPX accepts variable length numeric strings
// EPX allows variable length TRAN_NBR (no need for zero-padding)
func TestUUIDToEPXTranNbr_NoLeadingZeros(t *testing.T) {
	// Test many UUIDs to see range of output lengths
	lengths := make(map[int]int)

	testUUIDs := []string{
		"00000000-0000-0000-0000-000000000000",
		"00000000-0000-0000-0000-000000000001",
		"12345678-1234-1234-1234-123456789abc",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
	}

	// Add some random UUIDs
	for i := 0; i < 100; i++ {
		testUUIDs = append(testUUIDs, uuid.New().String())
	}

	for _, uuidStr := range testUUIDs {
		id := uuid.MustParse(uuidStr)
		result := UUIDToEPXTranNbr(id)

		// Track length distribution
		lengths[len(result)]++

		// Verify no leading zeros (except for "0" itself)
		if len(result) > 1 {
			assert.NotEqual(t, '0', result[0],
				"TRAN_NBR should not have leading zeros (got %s)", result)
		}

		// Verify valid numeric format
		_, err := strconv.ParseUint(result, 10, 32)
		require.NoError(t, err, "Result must be valid numeric string")
	}

	// Log length distribution
	t.Logf("TRAN_NBR length distribution across %d UUIDs:", len(testUUIDs))
	for length := 1; length <= 10; length++ {
		if count := lengths[length]; count > 0 {
			t.Logf("  %d digits: %d occurrences (%.1f%%)",
				length, count, float64(count)/float64(len(testUUIDs))*100)
		}
	}

	// Verify we see variety in lengths (FNV-1a should produce varied lengths)
	assert.Greater(t, len(lengths), 1,
		"Should see variety in TRAN_NBR lengths (not all same length)")
}
