package converters

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestToNullableText(t *testing.T) {
	t.Run("nil pointer returns invalid", func(t *testing.T) {
		result := ToNullableText(nil)
		assert.False(t, result.Valid)
	})

	t.Run("valid string pointer returns valid Text", func(t *testing.T) {
		str := "test"
		result := ToNullableText(&str)
		assert.True(t, result.Valid)
		assert.Equal(t, "test", result.String)
	})

	t.Run("empty string returns valid Text", func(t *testing.T) {
		str := ""
		result := ToNullableText(&str)
		assert.True(t, result.Valid)
		assert.Equal(t, "", result.String)
	})
}

func TestToNullableUUID(t *testing.T) {
	t.Run("nil pointer returns invalid", func(t *testing.T) {
		result := ToNullableUUID(nil)
		assert.False(t, result.Valid)
	})

	t.Run("valid UUID string returns valid UUID", func(t *testing.T) {
		uuidStr := "550e8400-e29b-41d4-a716-446655440000"
		result := ToNullableUUID(&uuidStr)
		assert.True(t, result.Valid)
		expected, _ := uuid.Parse(uuidStr)
		assert.Equal(t, expected, uuid.UUID(result.Bytes))
	})

	t.Run("invalid UUID string returns invalid", func(t *testing.T) {
		invalidStr := "not-a-uuid"
		result := ToNullableUUID(&invalidStr)
		assert.False(t, result.Valid)
	})
}

func TestToNullableUUIDFromUUID(t *testing.T) {
	t.Run("nil pointer returns invalid", func(t *testing.T) {
		result := ToNullableUUIDFromUUID(nil)
		assert.False(t, result.Valid)
	})

	t.Run("valid UUID pointer returns valid UUID", func(t *testing.T) {
		id := uuid.New()
		result := ToNullableUUIDFromUUID(&id)
		assert.True(t, result.Valid)
		assert.Equal(t, id, uuid.UUID(result.Bytes))
	})
}
