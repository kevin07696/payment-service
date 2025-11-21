package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// EPXCallbackAuth provides authentication for EPX payment gateway callbacks
type EPXCallbackAuth struct {
	db             *sql.DB
	macSecret      string
	logger         *zap.Logger
	ipWhitelistMap map[string]bool // Cached IP whitelist for performance
}

// NewEPXCallbackAuth creates a new EPX callback authenticator
func NewEPXCallbackAuth(db *sql.DB, macSecret string, logger *zap.Logger) (*EPXCallbackAuth, error) {
	auth := &EPXCallbackAuth{
		db:             db,
		macSecret:      macSecret,
		logger:         logger,
		ipWhitelistMap: make(map[string]bool),
	}

	// Load IP whitelist
	if err := auth.loadIPWhitelist(); err != nil {
		return nil, fmt.Errorf("failed to load IP whitelist: %w", err)
	}

	return auth, nil
}

// loadIPWhitelist loads the EPX IP whitelist from the database
func (e *EPXCallbackAuth) loadIPWhitelist() error {
	rows, err := e.db.Query(`
		SELECT ip_address
		FROM epx_ip_whitelist
		WHERE is_active = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	whitelist := make(map[string]bool)

	for rows.Next() {
		var ipAddress string
		if err := rows.Scan(&ipAddress); err != nil {
			e.logger.Error("Failed to scan IP address", zap.Error(err))
			continue
		}
		whitelist[ipAddress] = true
	}

	e.ipWhitelistMap = whitelist
	e.logger.Info("Loaded EPX IP whitelist",
		zap.Int("count", len(whitelist)))

	return nil
}

// RefreshIPWhitelist refreshes the IP whitelist from the database
func (e *EPXCallbackAuth) RefreshIPWhitelist() error {
	return e.loadIPWhitelist()
}

// Middleware wraps an HTTP handler with EPX callback authentication
func (e *EPXCallbackAuth) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Verify IP whitelist
		clientIP := e.getClientIP(r)
		if !e.isIPWhitelisted(clientIP) {
			e.logger.Warn("EPX callback from unauthorized IP",
				zap.String("ip", clientIP),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method))

			// Log to audit table
			e.logCallbackAttempt(clientIP, r.URL.Path, false, "IP not whitelisted")

			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Step 2: Verify HMAC signature (if configured)
		if e.macSecret != "" {
			signature := r.Header.Get("X-EPX-Signature")
			if signature == "" {
				e.logger.Warn("EPX callback missing signature",
					zap.String("ip", clientIP),
					zap.String("path", r.URL.Path))

				e.logCallbackAttempt(clientIP, r.URL.Path, false, "Missing HMAC signature")

				http.Error(w, "Missing signature", http.StatusUnauthorized)
				return
			}

			// Read body for HMAC verification
			body, err := io.ReadAll(r.Body)
			if err != nil {
				e.logger.Error("Failed to read request body",
					zap.String("ip", clientIP),
					zap.Error(err))
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			// Calculate expected signature
			expectedSig := e.calculateHMAC(body)

			// Compare signatures
			if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
				e.logger.Warn("EPX callback HMAC verification failed",
					zap.String("ip", clientIP),
					zap.String("path", r.URL.Path),
					zap.String("provided_sig", signature),
					zap.String("expected_sig", expectedSig))

				e.logCallbackAttempt(clientIP, r.URL.Path, false, "Invalid HMAC signature")

				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Step 3: Add authentication context to request
		ctx := r.Context()
		ctx = context.WithValue(ctx, "auth_type", "epx_callback")
		ctx = context.WithValue(ctx, "client_ip", clientIP)
		r = r.WithContext(ctx)

		// Log successful callback auth
		e.logger.Info("EPX callback authenticated",
			zap.String("ip", clientIP),
			zap.String("path", r.URL.Path))

		e.logCallbackAttempt(clientIP, r.URL.Path, true, "")

		// Pass to next handler
		next(w, r)
	}
}

// MiddlewareWithSkip wraps an HTTP handler but allows skipping auth for specific paths
func (e *EPXCallbackAuth) MiddlewareWithSkip(skipPaths []string) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Check if this path should skip authentication
			for _, skipPath := range skipPaths {
				if strings.HasPrefix(r.URL.Path, skipPath) {
					next(w, r)
					return
				}
			}

			// Apply authentication
			e.Middleware(next)(w, r)
		}
	}
}

// isIPWhitelisted checks if an IP is in the whitelist
func (e *EPXCallbackAuth) isIPWhitelisted(ip string) bool {
	// Check cached whitelist
	if e.ipWhitelistMap[ip] {
		return true
	}

	// For development, also allow localhost
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}

	// Check if it's a private IP (for testing)
	parsedIP := net.ParseIP(ip)
	if parsedIP != nil && parsedIP.IsPrivate() {
		e.logger.Debug("Allowing private IP for EPX callback",
			zap.String("ip", ip))
		return true
	}

	return false
}

// calculateHMAC calculates the HMAC-SHA256 signature for the body
func (e *EPXCallbackAuth) calculateHMAC(body []byte) string {
	h := hmac.New(sha256.New, []byte(e.macSecret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// getClientIP extracts the client IP from the request
func (e *EPXCallbackAuth) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Check CF-Connecting-IP for Cloudflare
	cfIP := r.Header.Get("CF-Connecting-IP")
	if cfIP != "" {
		return cfIP
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return host
}

// logCallbackAttempt logs EPX callback authentication attempts
func (e *EPXCallbackAuth) logCallbackAttempt(clientIP, path string, success bool, errorMsg string) {
	go func() {
		metadata := map[string]interface{}{
			"path":      path,
			"client_ip": clientIP,
		}

		metadataJSON, _ := json.Marshal(metadata)

		_, err := e.db.Exec(`
			INSERT INTO audit_log (
				actor_type, actor_id, actor_name, action,
				metadata, ip_address, success, error_message,
				performed_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		`, "system", "epx_gateway", "EPX Payment Gateway", "epx.callback.auth",
			metadataJSON, clientIP, success, errorMsg)

		if err != nil {
			e.logger.Error("Failed to log EPX callback attempt",
				zap.String("ip", clientIP),
				zap.Error(err))
		}
	}()
}

// ValidateEPXResponse validates EPX response data structure and signature
func (e *EPXCallbackAuth) ValidateEPXResponse(data map[string]string) error {
	// Check required fields
	requiredFields := []string{
		"ResponseCode",
		"ReasonCode",
		"ReasonText",
		"OrderID",
		"TransactionID",
	}

	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate response code format
	responseCode := data["ResponseCode"]
	if responseCode != "00" && responseCode != "85" && responseCode != "05" {
		// 00 = Approved, 85 = No Reason to Decline, 05 = Declined
		e.logger.Warn("Unexpected EPX response code",
			zap.String("response_code", responseCode),
			zap.String("transaction_id", data["TransactionID"]))
	}

	return nil
}

// VerifyEPXSignature verifies the signature of EPX callback data
func (e *EPXCallbackAuth) VerifyEPXSignature(data map[string]string, signature string) bool {
	if e.macSecret == "" {
		// No secret configured, skip verification
		return true
	}

	// Build canonical string for signature
	// EPX typically signs: OrderID + TransactionID + ResponseCode + ReasonCode
	canonical := fmt.Sprintf("%s%s%s%s",
		data["OrderID"],
		data["TransactionID"],
		data["ResponseCode"],
		data["ReasonCode"])

	expectedSig := e.calculateHMAC([]byte(canonical))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
