// Package e2e provides handlers for E2E test setup and teardown.
// These endpoints are only available in non-production environments.
package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"go.uber.org/zap"
)

// Handler provides E2E test setup/cleanup endpoints
type Handler struct {
	queries       sqlc.Querier
	secretManager ports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewHandler creates a new E2E handler
func NewHandler(queries sqlc.Querier, secretManager ports.SecretManagerAdapter, logger *zap.Logger) *Handler {
	return &Handler{
		queries:       queries,
		secretManager: secretManager,
		logger:        logger,
	}
}

// SetupRequest contains the test data to create
type SetupRequest struct {
	Merchant MerchantRequest `json:"merchant"`
	Service  ServiceRequest  `json:"service"`
}

// MerchantRequest contains merchant data
type MerchantRequest struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	CustNbr     string `json:"cust_nbr"`
	MerchNbr    string `json:"merch_nbr"`
	DbaNbr      string `json:"dba_nbr"`
	TerminalNbr string `json:"terminal_nbr"`
	MacSecret   string `json:"mac_secret"` // Actual MAC secret value (stored in secret manager)
	Environment string `json:"environment"`
}

// ServiceRequest contains service data
type ServiceRequest struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// SetupResponse contains the created test data IDs
type SetupResponse struct {
	Merchant MerchantResponse `json:"merchant"`
	Service  ServiceResponse  `json:"service"`
}

// MerchantResponse contains created merchant info
type MerchantResponse struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
}

// ServiceResponse contains created service info
type ServiceResponse struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
}

// Setup handles POST /internal/e2e/setup
// Creates isolated test merchant and service for E2E tests
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	// Only allow in non-production environments
	env := os.Getenv("APP_ENV")
	if env == "production" || env == "prod" {
		http.Error(w, "E2E setup not available in production", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode setup request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Parse or generate merchant ID
	merchantID, err := uuid.Parse(req.Merchant.ID)
	if err != nil {
		merchantID = uuid.New()
	}

	// Generate unique secret path for this test merchant
	macSecretPath := fmt.Sprintf("e2e/merchants/%s/mac", merchantID.String())

	// Store MAC secret in secret manager
	if req.Merchant.MacSecret != "" && h.secretManager != nil {
		_, err = h.secretManager.PutSecret(ctx, macSecretPath, req.Merchant.MacSecret, map[string]string{
			"merchant_id": merchantID.String(),
			"environment": "e2e-test",
		})
		if err != nil {
			h.logger.Error("Failed to store MAC secret", zap.Error(err))
			http.Error(w, "Failed to store MAC secret", http.StatusInternalServerError)
			return
		}
	}

	// Create merchant
	merchant, err := h.queries.CreateMerchant(ctx, sqlc.CreateMerchantParams{
		ID:            merchantID,
		Slug:          req.Merchant.Slug,
		Name:          req.Merchant.Name,
		CustNbr:       req.Merchant.CustNbr,
		MerchNbr:      req.Merchant.MerchNbr,
		DbaNbr:        req.Merchant.DbaNbr,
		TerminalNbr:   req.Merchant.TerminalNbr,
		MacSecretPath: macSecretPath,
		Environment:   req.Merchant.Environment,
		IsActive:      true,
	})
	if err != nil {
		h.logger.Error("Failed to create E2E test merchant", zap.Error(err))
		http.Error(w, "Failed to create merchant", http.StatusInternalServerError)
		return
	}

	// Parse or generate service ID
	serviceID, err := uuid.Parse(req.Service.ID)
	if err != nil {
		serviceID = uuid.New()
	}

	// Generate public key fingerprint
	fingerprint := generateFingerprint([]byte(req.Service.PublicKey))

	// Create service with proper pgtype fields
	service, err := h.queries.CreateService(ctx, sqlc.CreateServiceParams{
		ID:                   serviceID,
		ServiceID:            req.Service.ServiceID,
		ServiceName:          req.Service.Name,
		PublicKey:            req.Service.PublicKey,
		PublicKeyFingerprint: fingerprint,
		Environment:          req.Merchant.Environment,
		RequestsPerSecond:    pgtype.Int4{Int32: 1000, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 2000, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		// Cleanup merchant on failure
		if cleanupErr := h.queries.HardDeleteMerchant(ctx, merchantID); cleanupErr != nil {
			h.logger.Warn("Failed to cleanup merchant after service creation failure",
				zap.String("merchant_id", merchantID.String()),
				zap.Error(cleanupErr))
		}
		h.logger.Error("Failed to create E2E test service", zap.Error(err))
		http.Error(w, "Failed to create service", http.StatusInternalServerError)
		return
	}

	// Default scopes for E2E tests
	scopes := []string{
		"payments:create",
		"payments:read",
		"payments:void",
		"payments:refund",
		"payment_methods:read",
		"payment_methods:create",
		"subscriptions:manage",
		"subscriptions:read",
	}

	// Grant service access to merchant
	_, err = h.queries.GrantServiceAccess(ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  serviceID,
		MerchantID: merchantID,
		Scopes:     scopes,
		ExpiresAt:  pgtype.Timestamptz{}, // No expiry
	})
	if err != nil {
		// Cleanup on failure
		if cleanupErr := h.queries.HardDeleteService(ctx, serviceID); cleanupErr != nil {
			h.logger.Warn("Failed to cleanup service after grant access failure",
				zap.String("service_id", serviceID.String()),
				zap.Error(cleanupErr))
		}
		if cleanupErr := h.queries.HardDeleteMerchant(ctx, merchantID); cleanupErr != nil {
			h.logger.Warn("Failed to cleanup merchant after grant access failure",
				zap.String("merchant_id", merchantID.String()),
				zap.Error(cleanupErr))
		}
		h.logger.Error("Failed to grant E2E service access", zap.Error(err))
		http.Error(w, "Failed to grant access", http.StatusInternalServerError)
		return
	}

	h.logger.Info("E2E test data created",
		zap.String("merchant_id", merchant.ID.String()),
		zap.String("service_id", service.ServiceID),
	)

	// Return created IDs
	resp := SetupResponse{
		Merchant: MerchantResponse{
			ID:   merchant.ID.String(),
			Slug: merchant.Slug,
		},
		Service: ServiceResponse{
			ID:        service.ID.String(),
			ServiceID: service.ServiceID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// CleanupRequest contains IDs to cleanup
type CleanupRequest struct {
	MerchantID string `json:"merchant_id"`
	ServiceID  string `json:"service_id"`
}

// Cleanup handles POST /internal/e2e/cleanup
// Removes test data created by Setup
func (h *Handler) Cleanup(w http.ResponseWriter, r *http.Request) {
	// Only allow in non-production environments
	env := os.Getenv("APP_ENV")
	if env == "production" || env == "prod" {
		http.Error(w, "E2E cleanup not available in production", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode cleanup request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Revoke service access first
	if req.ServiceID != "" && req.MerchantID != "" {
		serviceID, err1 := uuid.Parse(req.ServiceID)
		merchantID, err2 := uuid.Parse(req.MerchantID)
		if err1 == nil && err2 == nil {
			if err := h.queries.RevokeServiceAccess(ctx, sqlc.RevokeServiceAccessParams{
				ServiceID:  serviceID,
				MerchantID: merchantID,
			}); err != nil {
				h.logger.Warn("Failed to revoke service access during cleanup",
					zap.String("service_id", req.ServiceID),
					zap.String("merchant_id", req.MerchantID),
					zap.Error(err))
			}
		}
	}

	// Delete service
	if req.ServiceID != "" {
		if serviceID, err := uuid.Parse(req.ServiceID); err == nil {
			if err := h.queries.HardDeleteService(ctx, serviceID); err != nil {
				h.logger.Warn("Failed to delete service during cleanup",
					zap.String("service_id", req.ServiceID),
					zap.Error(err))
			}
		}
	}

	// Delete merchant
	if req.MerchantID != "" {
		if merchantID, err := uuid.Parse(req.MerchantID); err == nil {
			if err := h.queries.HardDeleteMerchant(ctx, merchantID); err != nil {
				h.logger.Warn("Failed to delete merchant during cleanup",
					zap.String("merchant_id", req.MerchantID),
					zap.Error(err))
			}
		}
	}

	h.logger.Info("E2E test data cleaned up",
		zap.String("merchant_id", req.MerchantID),
		zap.String("service_id", req.ServiceID),
	)

	w.WriteHeader(http.StatusNoContent)
}

// generateFingerprint creates a SHA256 fingerprint of the public key.
func generateFingerprint(publicKeyPEM []byte) string {
	h := sha256.New()
	h.Write(publicKeyPEM)
	return fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]
}
