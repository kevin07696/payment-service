package middleware

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// ExtractAuthContext extracts authentication information from request context
// Returns actor_id, actor_name, and request_id for audit logging
func ExtractAuthContext(ctx context.Context) (actorID, actorName, requestID pgtype.Text) {
	// Extract service ID (from JWT claims)
	if serviceID, ok := ctx.Value(ServiceIDKey).(string); ok && serviceID != "" {
		actorID = pgtype.Text{String: serviceID, Valid: true}
		actorName = pgtype.Text{String: "service:" + serviceID, Valid: true}
	}

	// Extract request ID (added by middleware)
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		requestID = pgtype.Text{String: reqID, Valid: true}
	}

	return actorID, actorName, requestID
}

// ExtractAuthType returns the authentication type from context (e.g., "jwt", "admin")
func ExtractAuthType(ctx context.Context) string {
	if authType, ok := ctx.Value(AuthTypeKey).(string); ok {
		return authType
	}
	return ""
}

// ExtractMerchantID returns the merchant ID from JWT context
func ExtractMerchantID(ctx context.Context) string {
	if merchantID, ok := ctx.Value(MerchantIDKey).(string); ok {
		return merchantID
	}
	return ""
}
