# Integration Tests

Integration tests for payment-service that run against deployed service instances.

## Overview

These tests validate the service's integration with external systems (database, EPX, etc.) by making HTTP requests to a deployed service.

**Key characteristic**: Tests run against a DEPLOYED service URL, not localhost.

## Structure

```
tests/integration/
├── merchant/           # Merchant API tests
├── payment/            # Payment processing tests
├── epx/               # EPX adapter integration tests
└── testutil/          # Shared test utilities
    ├── config.go      # Configuration management
    ├── client.go      # HTTP client wrapper
    └── setup.go       # Test setup helpers
```

## Running Tests

### Prerequisites

- Deployed service (staging or local)
- Database with seed data applied
- EPX sandbox credentials

### Environment Variables

```bash
# Service URL (required)
export SERVICE_URL="http://your-service-url.com"

# EPX test credentials (required for payment tests)
export EPX_MAC_STAGING="2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
export EPX_CUST_NBR="9001"
export EPX_MERCH_NBR="900300"
export EPX_DBA_NBR="2"
export EPX_TERMINAL_NBR="77"
```

### Run Tests

```bash
# Against deployed staging
export SERVICE_URL="http://your-staging-service.com"
go test ./tests/integration/... -v -tags=integration

# Against local service
export SERVICE_URL="http://localhost:8080"
go test ./tests/integration/... -v -tags=integration

# Run specific test package
go test ./tests/integration/merchant -v -tags=integration

# Run with timeout
go test ./tests/integration/... -v -tags=integration -timeout=10m
```

## Writing Tests

### Test Template

```go
// +build integration

package yourpackage_test

import (
	"testing"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYourFeature(t *testing.T) {
	cfg, client := testutil.Setup(t)

	// Make API request
	resp, err := client.Do("GET", "/api/v1/endpoint", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, 200, resp.StatusCode)
}
```

### Best Practices

1. **Use build tags**: All integration tests should have `// +build integration`
2. **Use testutil.Setup()**: Always use the setup helper for consistency
3. **Test deployed service**: Tests should work against any deployed instance
4. **Use seed data**: Rely on test merchant from `003_agent_credentials.sql`
5. **Clean assertions**: Use testify for clear test assertions

## CI/CD Integration

These tests run automatically in GitHub Actions after staging deployment:

```yaml
integration-tests:
  needs: deploy-staging
  runs-on: ubuntu-latest
  env:
    SERVICE_URL: ${{ needs.deploy-staging.outputs.service_url }}
    EPX_MAC_STAGING: ${{ secrets.EPX_MAC_STAGING }}
  steps:
    - run: go test ./tests/integration/... -v -tags=integration
```

**This acts as a deployment gate** - production deployment only happens if integration tests pass.

## Test Data

Integration tests use the test merchant seeded in staging via `internal/db/seeds/staging/003_agent_credentials.sql`:

- **Agent ID**: `test-merchant-staging`
- **EPX Credentials**: Sandbox credentials (CUST_NBR: 9001, etc.)
- **MAC Secret**: Stored in database or OCI Vault (staging)

## Troubleshooting

### "Failed to load test configuration"
**Solution**: Ensure all environment variables are set

### "connect: connection refused"
**Solution**: Verify service URL is correct and service is running

### "API error (status 404)"
**Solution**: Check that migrations and seed data have been applied

### Tests pass locally but fail in CI
**Solution**: Verify GitHub Secrets are configured correctly
