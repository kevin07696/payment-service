// Package fixtures provides test data builders and helpers.
// Eliminates ~50 lines of duplicated pointer helper functions across test files.
package fixtures

import (
	"time"

	"github.com/google/uuid"
)

// StringPtr returns a pointer to the given string.
// This eliminates duplication of ptr, strPtr, stringPtr across test files.
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int.
func IntPtr(i int) *int {
	return &i
}

// Int32Ptr returns a pointer to the given int32.
func Int32Ptr(i int32) *int32 {
	return &i
}

// Int64Ptr returns a pointer to the given int64.
func Int64Ptr(i int64) *int64 {
	return &i
}

// BoolPtr returns a pointer to the given bool.
func BoolPtr(b bool) *bool {
	return &b
}

// Float64Ptr returns a pointer to the given float64.
func Float64Ptr(f float64) *float64 {
	return &f
}

// TimePtr returns a pointer to the given time.Time.
func TimePtr(t time.Time) *time.Time {
	return &t
}

// UUIDPtr returns a pointer to the given UUID.
func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
