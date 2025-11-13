package util

import (
	"fmt"
	"hash/fnv"

	"github.com/google/uuid"
)

// UUIDToEPXTranNbr converts a UUID to a numeric TRAN_NBR for EPX (max 10 digits)
// Uses FNV-1a 32-bit hash to create a deterministic number from UUID
// This ensures idempotency - the same UUID always produces the same TRAN_NBR
// EPX requirement: TRAN_NBR must be numeric with max 10 digits
// FNV-1a 32-bit produces numbers up to ~4.3 billion (10 digits), which fits EPX's requirement
func UUIDToEPXTranNbr(id uuid.UUID) string {
	// Use FNV-1a 32-bit hash for deterministic hashing
	h := fnv.New32a()
	h.Write(id[:])
	hash := h.Sum32()

	// uint32 max is 4,294,967,295 (~4.3 billion, 10 digits) - fits EPX requirement
	// Format as numeric string (no leading zeros needed - EPX accepts variable length)
	return fmt.Sprintf("%d", hash)
}
