package fixtures

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// ServiceBuilder provides fluent API for building test services.
type ServiceBuilder struct {
	service *sqlc.Service
}

// NewService creates a new service builder with sensible defaults.
func NewService() *ServiceBuilder {
	now := time.Now()
	return &ServiceBuilder{
		service: &sqlc.Service{
			ID:                   uuid.New(),
			ServiceID:            "test-service",
			ServiceName:          "Test Service",
			PublicKey:            "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----",
			PublicKeyFingerprint: "SHA256:abcdef1234567890",
			Environment:          "sandbox",
			RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
			BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
			IsActive:             pgtype.Bool{Bool: true, Valid: true},
			CreatedAt: pgtype.Timestamptz{
				Time:  now,
				Valid: true,
			},
			UpdatedAt: pgtype.Timestamptz{
				Time:  now,
				Valid: true,
			},
		},
	}
}

func (b *ServiceBuilder) WithID(id uuid.UUID) *ServiceBuilder {
	b.service.ID = id
	return b
}

func (b *ServiceBuilder) WithServiceID(serviceID string) *ServiceBuilder {
	b.service.ServiceID = serviceID
	return b
}

func (b *ServiceBuilder) WithServiceName(name string) *ServiceBuilder {
	b.service.ServiceName = name
	return b
}

func (b *ServiceBuilder) WithPublicKey(publicKey string) *ServiceBuilder {
	b.service.PublicKey = publicKey
	return b
}

func (b *ServiceBuilder) WithPublicKeyFingerprint(fingerprint string) *ServiceBuilder {
	b.service.PublicKeyFingerprint = fingerprint
	return b
}

func (b *ServiceBuilder) WithEnvironment(env string) *ServiceBuilder {
	b.service.Environment = env
	return b
}

func (b *ServiceBuilder) WithRateLimit(requestsPerSecond, burstLimit int32) *ServiceBuilder {
	b.service.RequestsPerSecond = pgtype.Int4{Int32: requestsPerSecond, Valid: true}
	b.service.BurstLimit = pgtype.Int4{Int32: burstLimit, Valid: true}
	return b
}

func (b *ServiceBuilder) Active() *ServiceBuilder {
	b.service.IsActive = pgtype.Bool{Bool: true, Valid: true}
	return b
}

func (b *ServiceBuilder) Inactive() *ServiceBuilder {
	b.service.IsActive = pgtype.Bool{Bool: false, Valid: true}
	return b
}

func (b *ServiceBuilder) WithCreatedBy(createdBy uuid.UUID) *ServiceBuilder {
	b.service.CreatedBy = pgtype.UUID{Bytes: createdBy, Valid: true}
	return b
}

func (b *ServiceBuilder) Build() sqlc.Service {
	return *b.service
}

// Convenience functions for common service scenarios

// ActiveService creates an active service with given ID.
func ActiveService(serviceID string) sqlc.Service {
	return NewService().
		WithServiceID(serviceID).
		Active().
		Build()
}

// InactiveService creates an inactive service with given ID.
func InactiveService(serviceID string) sqlc.Service {
	return NewService().
		WithServiceID(serviceID).
		Inactive().
		Build()
}
