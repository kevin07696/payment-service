package ports

import (
	"context"
)

// Secret represents a retrieved secret with metadata
type Secret struct {
	Value     string            // The secret value (e.g., MAC key)
	Version   string            // Secret version identifier
	Metadata  map[string]string // Additional secret metadata
	CreatedAt string            // When this version was created
}

// SecretRotationInfo contains information about secret rotation
type SecretRotationInfo struct {
	CurrentVersion  string // Currently active version
	PreviousVersion string // Previous version (for graceful rotation)
	NextRotation    string // Scheduled next rotation date (if applicable)
}

// SecretManagerAdapter defines the port for retrieving secrets from a secret management service
// Supports multiple backends: AWS Secrets Manager, GCP Secret Manager, HashiCorp Vault
// Implementation is responsible for:
//   - Authentication with the secret manager service
//   - Caching secrets appropriately (with TTL)
//   - Handling secret rotation gracefully
//   - Circuit breaking on repeated failures
type SecretManagerAdapter interface {
	// GetSecret retrieves a secret by its path/name
	// Path format depends on implementation:
	//   - AWS: "payment-service/agents/{agent_id}/mac"
	//   - GCP: "projects/{project}/secrets/{name}/versions/latest"
	//   - Vault: "secret/data/payment-service/agents/{agent_id}"
	// Returns error if:
	//   - Secret does not exist
	//   - Insufficient permissions
	//   - Network communication fails
	//   - Secret manager service is unavailable
	GetSecret(ctx context.Context, path string) (*Secret, error)

	// GetSecretVersion retrieves a specific version of a secret
	// Useful during secret rotation to access previous version
	// Returns error with same conditions as GetSecret
	GetSecretVersion(ctx context.Context, path string, version string) (*Secret, error)

	// PutSecret creates or updates a secret (admin/rotation operations)
	// Returns the new version identifier
	// Returns error if:
	//   - Insufficient permissions
	//   - Secret format is invalid
	//   - Network communication fails
	PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (version string, err error)

	// RotateSecret rotates a secret by creating a new version and marking old version for deletion
	// Implements graceful rotation: new version is created before old one is deleted
	// Returns rotation info containing both current and previous versions
	// Returns error if rotation fails at any step
	RotateSecret(ctx context.Context, path string, newValue string) (*SecretRotationInfo, error)

	// DeleteSecret permanently deletes a secret (admin operations only)
	// Use with extreme caution - this is irreversible
	// Returns error if:
	//   - Insufficient permissions
	//   - Secret does not exist
	//   - Secret is in use (if implementation has safeguards)
	DeleteSecret(ctx context.Context, path string) error
}
