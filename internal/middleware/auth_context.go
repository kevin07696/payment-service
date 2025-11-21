package middleware

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/auth"
)

// ExtractAuthContext extracts authentication information from request context
// Returns actor_id, actor_name, and request_id for audit logging
func ExtractAuthContext(ctx context.Context) (actorID, actorName, requestID pgtype.Text) {
	// Extract service ID (from JWT claims)
	if serviceID, ok := ctx.Value(auth.ServiceIDKey).(string); ok && serviceID != "" {
		actorID = pgtype.Text{String: serviceID, Valid: true}
		actorName = pgtype.Text{String: "service:" + serviceID, Valid: true}
	}

	// Extract request ID (added by middleware)
	if reqID, ok := ctx.Value(auth.RequestIDKey).(string); ok && reqID != "" {
		requestID = pgtype.Text{String: reqID, Valid: true}
	}

	return actorID, actorName, requestID
}

// ExtractAuthType returns the authentication type from context (e.g., "jwt", "admin")
func ExtractAuthType(ctx context.Context) string {
	if authType, ok := ctx.Value(auth.AuthTypeKey).(string); ok {
		return authType
	}
	return ""
}

// ExtractMerchantID returns the merchant ID from JWT context
func ExtractMerchantID(ctx context.Context) string {
	if merchantID, ok := ctx.Value(auth.MerchantIDKey).(string); ok {
		return merchantID
	}
	return ""
}
