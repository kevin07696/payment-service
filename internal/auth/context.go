package auth

import (
	"context"
	"fmt"
)

// Context keys for authentication data
type contextKey string

const (
	// Authentication type keys
	AuthTypeKey     contextKey = "auth_type"
	ServiceIDKey    contextKey = "service_id"
	MerchantIDKey   contextKey = "merchant_id"
	MerchantCodeKey contextKey = "merchant_code"
	TokenJTIKey     contextKey = "token_jti"
	RequestIDKey    contextKey = "request_id"
	ClientIPKey     contextKey = "client_ip"
	ScopesKey       contextKey = "scopes"
	EnvironmentKey  contextKey = "environment"
)

// AuthType represents the type of authentication used
type AuthType string

const (
	AuthTypeJWT         AuthType = "jwt"
	AuthTypeAPIKey      AuthType = "api_key"
	AuthTypeEPXCallback AuthType = "epx_callback"
	AuthTypeInternal    AuthType = "internal"
	AuthTypeNone        AuthType = "none"
)

// AuthInfo contains authentication information from the context
type AuthInfo struct {
	Type         AuthType
	MerchantID   string
	MerchantCode string
	ServiceID    string
	TokenJTI     string
	RequestID    string
	ClientIP     string
	Scopes       []string
	Environment  string
}

// GetAuthInfo extracts authentication information from the context
func GetAuthInfo(ctx context.Context) *AuthInfo {
	info := &AuthInfo{
		Type: AuthTypeNone,
	}

	// Extract auth type
	if authType, ok := ctx.Value(AuthTypeKey).(string); ok {
		info.Type = AuthType(authType)
	}

	// Extract merchant information
	if merchantID, ok := ctx.Value(MerchantIDKey).(string); ok {
		info.MerchantID = merchantID
	}

	if merchantCode, ok := ctx.Value(MerchantCodeKey).(string); ok {
		info.MerchantCode = merchantCode
	}

	// Extract service information
	if serviceID, ok := ctx.Value(ServiceIDKey).(string); ok {
		info.ServiceID = serviceID
	}

	// Extract token information
	if tokenJTI, ok := ctx.Value(TokenJTIKey).(string); ok {
		info.TokenJTI = tokenJTI
	}

	// Extract request information
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		info.RequestID = requestID
	}

	if clientIP, ok := ctx.Value(ClientIPKey).(string); ok {
		info.ClientIP = clientIP
	}

	// Extract scopes
	if scopes, ok := ctx.Value(ScopesKey).([]string); ok {
		info.Scopes = scopes
	}

	// Extract environment
	if env, ok := ctx.Value(EnvironmentKey).(string); ok {
		info.Environment = env
	}

	return info
}

// IsAuthenticated checks if the context contains valid authentication
func IsAuthenticated(ctx context.Context) bool {
	authType, ok := ctx.Value(AuthTypeKey).(string)
	return ok && authType != "" && authType != string(AuthTypeNone)
}

// RequireMerchant checks if the context has merchant authentication
func RequireMerchant(ctx context.Context) error {
	if !IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required")
	}

	merchantID, ok := ctx.Value(MerchantIDKey).(string)
	if !ok || merchantID == "" {
		return fmt.Errorf("merchant authentication required")
	}

	return nil
}

// RequireService checks if the context has service authentication
func RequireService(ctx context.Context) error {
	if !IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required")
	}

	authType, _ := ctx.Value(AuthTypeKey).(string)
	if authType != string(AuthTypeJWT) {
		return fmt.Errorf("service authentication required")
	}

	serviceID, ok := ctx.Value(ServiceIDKey).(string)
	if !ok || serviceID == "" {
		return fmt.Errorf("valid service authentication required")
	}

	return nil
}

// RequireScope checks if the context has the required scope
func RequireScope(ctx context.Context, requiredScope string) error {
	scopes, ok := ctx.Value(ScopesKey).([]string)
	if !ok {
		return fmt.Errorf("no scopes in context")
	}

	for _, scope := range scopes {
		if scope == requiredScope {
			return nil
		}
	}

	return fmt.Errorf("missing required scope: %s", requiredScope)
}

// RequireAnyScope checks if the context has any of the required scopes
func RequireAnyScope(ctx context.Context, requiredScopes []string) error {
	scopes, ok := ctx.Value(ScopesKey).([]string)
	if !ok {
		return fmt.Errorf("no scopes in context")
	}

	scopeMap := make(map[string]bool)
	for _, scope := range scopes {
		scopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if scopeMap[required] {
			return nil
		}
	}

	return fmt.Errorf("missing any of required scopes: %v", requiredScopes)
}

// WithAuth adds authentication information to the context
func WithAuth(ctx context.Context, info *AuthInfo) context.Context {
	ctx = context.WithValue(ctx, AuthTypeKey, string(info.Type))

	if info.MerchantID != "" {
		ctx = context.WithValue(ctx, MerchantIDKey, info.MerchantID)
	}

	if info.MerchantCode != "" {
		ctx = context.WithValue(ctx, MerchantCodeKey, info.MerchantCode)
	}

	if info.ServiceID != "" {
		ctx = context.WithValue(ctx, ServiceIDKey, info.ServiceID)
	}

	if info.TokenJTI != "" {
		ctx = context.WithValue(ctx, TokenJTIKey, info.TokenJTI)
	}

	if info.RequestID != "" {
		ctx = context.WithValue(ctx, RequestIDKey, info.RequestID)
	}

	if info.ClientIP != "" {
		ctx = context.WithValue(ctx, ClientIPKey, info.ClientIP)
	}

	if len(info.Scopes) > 0 {
		ctx = context.WithValue(ctx, ScopesKey, info.Scopes)
	}

	if info.Environment != "" {
		ctx = context.WithValue(ctx, EnvironmentKey, info.Environment)
	}

	return ctx
}

// WithInternalAuth adds internal/system authentication to the context
func WithInternalAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, AuthTypeKey, string(AuthTypeInternal))
}

// IsInternalAuth checks if the context has internal/system authentication
func IsInternalAuth(ctx context.Context) bool {
	authType, ok := ctx.Value(AuthTypeKey).(string)
	return ok && authType == string(AuthTypeInternal)
}

// GetMerchantID safely extracts the merchant ID from the context
func GetMerchantID(ctx context.Context) (string, error) {
	merchantID, ok := ctx.Value(MerchantIDKey).(string)
	if !ok || merchantID == "" {
		return "", fmt.Errorf("merchant ID not found in context")
	}
	return merchantID, nil
}

// GetServiceID safely extracts the service ID from the context
func GetServiceID(ctx context.Context) (string, error) {
	serviceID, ok := ctx.Value(ServiceIDKey).(string)
	if !ok || serviceID == "" {
		return "", fmt.Errorf("service ID not found in context")
	}
	return serviceID, nil
}

// GetRequestID safely extracts the request ID from the context
func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(RequestIDKey).(string)
	return requestID
}

// GetClientIP safely extracts the client IP from the context
func GetClientIP(ctx context.Context) string {
	clientIP, _ := ctx.Value(ClientIPKey).(string)
	return clientIP
}
