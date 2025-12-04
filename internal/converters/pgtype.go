package converters

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ToNullableText converts a string pointer to pgtype.Text
// Returns invalid Text if pointer is nil
func ToNullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ToNullableUUID converts a UUID string pointer to pgtype.UUID
// Returns invalid UUID if pointer is nil or string cannot be parsed
func ToNullableUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{Valid: false}
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// ToNullableUUIDFromUUID converts a UUID pointer to pgtype.UUID
// Returns invalid UUID if pointer is nil
func ToNullableUUIDFromUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
