package epxutil

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
