package auth

import (
	"context"
)

// Context keys for authentication data
// Using unexported struct types with unique field values prevents key collisions (Go best practice)
type contextKey struct{ name string }

var (
	// Authentication type keys with unique values
	AuthTypeKey     = contextKey{"auth_type"}
	ServiceIDKey    = contextKey{"service_id"}
	MerchantIDKey   = contextKey{"merchant_id"}
	MerchantCodeKey = contextKey{"merchant_code"}
	TokenJTIKey     = contextKey{"token_jti"}
	RequestIDKey    = contextKey{"request_id"}
	ClientIPKey     = contextKey{"client_ip"}
	UserAgentKey    = contextKey{"user_agent"}
	ScopesKey       = contextKey{"scopes"}
	EnvironmentKey  = contextKey{"environment"}
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
	UserAgent    string
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

	if userAgent, ok := ctx.Value(UserAgentKey).(string); ok {
		info.UserAgent = userAgent
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

// GetClientIP safely extracts the client IP from the context
func GetClientIP(ctx context.Context) string {
	clientIP, _ := ctx.Value(ClientIPKey).(string)
	return clientIP
}

// WithAuth adds authentication information to the context
// Used by tests and internal services to set up auth context
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

	if info.UserAgent != "" {
		ctx = context.WithValue(ctx, UserAgentKey, info.UserAgent)
	}

	if len(info.Scopes) > 0 {
		ctx = context.WithValue(ctx, ScopesKey, info.Scopes)
	}

	if info.Environment != "" {
		ctx = context.WithValue(ctx, EnvironmentKey, info.Environment)
	}

	return ctx
}
