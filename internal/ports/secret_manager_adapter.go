package ports

import "context"

// Secret represents a retrieved secret with metadata.
type Secret struct {
	Value     string
	Version   string
	Metadata  map[string]string
	CreatedAt string
}

// SecretRotationInfo contains information about secret rotation.
type SecretRotationInfo struct {
	CurrentVersion  string
	PreviousVersion string
	NextRotation    string
}

// SecretManagerAdapter defines the port for retrieving secrets from a secret management service.
type SecretManagerAdapter interface {
	GetSecret(ctx context.Context, path string) (*Secret, error)
	GetSecretVersion(ctx context.Context, path string, version string) (*Secret, error)
	PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (version string, err error)
	RotateSecret(ctx context.Context, path string, newValue string) (*SecretRotationInfo, error)
	DeleteSecret(ctx context.Context, path string) error
}
