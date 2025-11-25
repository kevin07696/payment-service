package middleware

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"go.uber.org/zap"
)

// MerchantValidatorInterceptor validates that the JWT merchant_id matches the request agent_id
// CRITICAL SECURITY: This prevents a merchant from accessing another merchant's data
// by ensuring the authenticated merchant_id from the JWT token matches the agent_id
// in the request payload.
//
// Example Attack Scenario (without this validator):
//  1. Merchant A authenticates with their JWT (merchant_id = "merchant-a")
//  2. Merchant A sends a request with agent_id = "merchant-b"
//  3. Without validation, Merchant A could access Merchant B's data
//
// This interceptor prevents such attacks by validating:
//   - JWT contains a valid merchant_id claim
//   - Request contains an agent_id field
//   - merchant_id resolves to the same merchant as agent_id
//
// Architecture:
//   - Integrates with AuthInterceptor (runs after JWT validation)
//   - Uses sqlc queries for efficient database lookups
//   - Provides detailed error messages for debugging
//   - Skips validation for health check endpoints
type MerchantValidatorInterceptor struct {
	queries sqlc.Querier
	logger  *zap.Logger
}

// NewMerchantValidatorInterceptor creates a new merchant validation interceptor
func NewMerchantValidatorInterceptor(queries sqlc.Querier, logger *zap.Logger) *MerchantValidatorInterceptor {
	return &MerchantValidatorInterceptor{
		queries: queries,
		logger:  logger,
	}
}

// WrapUnary provides merchant validation for unary RPC calls
func (m *MerchantValidatorInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Skip validation for health checks and other non-merchant endpoints
		if !requiresMerchantValidation(req.Spec().Procedure) {
			return next(ctx, req)
		}

		// Extract merchant_id from JWT context (set by AuthInterceptor)
		jwtMerchantID, ok := ctx.Value(auth.MerchantIDKey).(string)
		if !ok || jwtMerchantID == "" {
			m.logger.Warn("Missing merchant_id in JWT context",
				zap.String("procedure", req.Spec().Procedure))
			return nil, connect.NewError(connect.CodeUnauthenticated,
				fmt.Errorf("missing merchant_id in authentication token"))
		}

		// Extract agent_id from request (if present)
		agentID := extractAgentID(req)
		if agentID == "" {
			// Some endpoints may not have agent_id (e.g., merchant profile endpoints)
			// In these cases, just verify the merchant_id is valid
			if err := m.validateMerchantID(ctx, jwtMerchantID); err != nil {
				m.logger.Warn("Invalid merchant_id in JWT",
					zap.String("merchant_id", jwtMerchantID),
					zap.String("procedure", req.Spec().Procedure),
					zap.Error(err))
				return nil, connect.NewError(connect.CodePermissionDenied, err)
			}
			return next(ctx, req)
		}

		// Validate that JWT merchant_id matches request agent_id
		if err := m.validateMerchantMatch(ctx, jwtMerchantID, agentID); err != nil {
			m.logger.Warn("Merchant validation failed",
				zap.String("jwt_merchant_id", jwtMerchantID),
				zap.String("request_agent_id", agentID),
				zap.String("procedure", req.Spec().Procedure),
				zap.Error(err))
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}

		return next(ctx, req)
	}
}

// WrapStreamingClient provides merchant validation for streaming client calls
func (m *MerchantValidatorInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		// For client streaming, validation happens at the HTTP layer
		return next(ctx, spec)
	}
}

// WrapStreamingHandler provides merchant validation for streaming handler calls
func (m *MerchantValidatorInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// Skip validation for health checks
		if !requiresMerchantValidation(conn.Spec().Procedure) {
			return next(ctx, conn)
		}

		// Extract merchant_id from JWT context
		jwtMerchantID, ok := ctx.Value(auth.MerchantIDKey).(string)
		if !ok || jwtMerchantID == "" {
			return connect.NewError(connect.CodeUnauthenticated,
				fmt.Errorf("missing merchant_id in authentication token"))
		}

		// Validate merchant_id exists
		if err := m.validateMerchantID(ctx, jwtMerchantID); err != nil {
			return connect.NewError(connect.CodePermissionDenied, err)
		}

		return next(ctx, conn)
	}
}

// validateMerchantID verifies that a merchant_id exists and is active
func (m *MerchantValidatorInterceptor) validateMerchantID(ctx context.Context, merchantID string) error {
	// Parse merchant UUID
	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Check if merchant exists and is active
	merchant, err := m.queries.GetMerchantByID(ctx, merchantUUID)
	if err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	if !merchant.IsActive {
		return fmt.Errorf("merchant account is inactive")
	}

	return nil
}

// validateMerchantMatch verifies that JWT merchant_id matches request agent_id
// This is the core security check that prevents cross-merchant data access
func (m *MerchantValidatorInterceptor) validateMerchantMatch(ctx context.Context, jwtMerchantID, agentID string) error {
	// First validate the JWT merchant_id exists and is active
	if err := m.validateMerchantID(ctx, jwtMerchantID); err != nil {
		return err
	}

	// Parse JWT merchant UUID
	jwtMerchantUUID, err := uuid.Parse(jwtMerchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant_id in JWT: %w", err)
	}

	// Look up merchant by agent_id (slug)
	agentMerchant, err := m.queries.GetMerchantBySlug(ctx, agentID)
	if err != nil {
		return fmt.Errorf("agent_id not found: %w", err)
	}

	// SECURITY: Verify both IDs point to the same merchant
	if jwtMerchantUUID != agentMerchant.ID {
		return fmt.Errorf("merchant_id mismatch: JWT merchant (%s) does not match requested agent (%s)",
			jwtMerchantID, agentID)
	}

	return nil
}

// extractAgentID extracts the agent_id field from a request
// This uses reflection to check common field names across different request types
func extractAgentID(req connect.AnyRequest) string {
	// Type switch on common request types
	// Each service may have different request structures
	msg := req.Any()

	// Try to extract via reflection for fields named "AgentId" or "MerchantId"
	// This is a simplified version - in production you'd want to use protobuf reflection
	// or define an interface that all requests implement

	// For now, we'll use type assertions for known request types
	// This can be extended as new services are added

	// Example for chargeback requests
	type AgentIDProvider interface {
		GetAgentId() string
	}

	if provider, ok := msg.(AgentIDProvider); ok {
		return provider.GetAgentId()
	}

	// Example for payment requests that might use merchant_id instead
	type MerchantIDProvider interface {
		GetMerchantId() string
	}

	if provider, ok := msg.(MerchantIDProvider); ok {
		return provider.GetMerchantId()
	}

	// No agent_id or merchant_id found
	return ""
}

// requiresMerchantValidation determines if an endpoint requires merchant validation
func requiresMerchantValidation(procedure string) bool {
	// Skip health checks and system endpoints
	skipProcedures := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	}

	for _, skip := range skipProcedures {
		if procedure == skip {
			return false
		}
	}

	// All other endpoints require merchant validation
	return true
}
