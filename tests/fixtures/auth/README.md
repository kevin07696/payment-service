# Test Service Authentication

This directory contains **ephemeral** test service credentials generated dynamically.

## Security Notice

**⚠️ IMPORTANT: This directory is in `.gitignore` to prevent committing secrets.**

The `test_services.json` file contains RSA private keys used for JWT authentication in integration tests. These keys should NEVER be committed to version control.

## Generating Test Keys

Before running integration tests, generate fresh test service keys:

```bash
./scripts/generate_test_keys.sh
```

This will create `test_services.json` with 3 test services, each having:
- Unique service ID
- 2048-bit RSA key pair
- SHA-256 public key fingerprint

## File Structure

```
tests/fixtures/auth/
├── README.md              # This file
└── test_services.json     # Generated keys (in .gitignore)
```

## Integration Test Usage

Integration tests automatically load test services from `test_services.json`:

```go
services, err := testutil.LoadTestServices()
if err != nil {
    // Error includes instructions to run ./scripts/generate_test_keys.sh
    t.Fatal(err)
}

// Use first service to generate JWT
token, err := testutil.GenerateJWT(
    services[0].PrivateKeyPEM,
    services[0].ServiceID,
    merchantID,
    1*time.Hour,
)
```

## CI/CD Setup

In CI/CD pipelines, add this step before running tests:

```yaml
- name: Generate test keys
  run: ./scripts/generate_test_keys.sh
```

## Troubleshooting

### Error: "test service keys not found"

**Solution:** Run `./scripts/generate_test_keys.sh` to generate keys.

### Error: "permission denied"

**Solution:** Make the script executable:
```bash
chmod +x scripts/generate_test_keys.sh
```

### Keys already exist

The script will prompt before overwriting existing keys. Answer `y` to regenerate or `n` to use existing keys.

## Security Benefits

1. **No secrets in Git**: Private keys are never committed
2. **Fresh keys per environment**: Each developer/CI run gets unique keys
3. **Test isolation**: Keys are only for testing, not production
4. **Clear audit trail**: Logs show when keys were generated

## Related Files

- `scripts/generate_test_keys.sh` - Shell script to generate keys
- `scripts/generate_test_keys/generate_test_keys.go` - Go key generator
- `tests/integration/testutil/auth_helpers.go` - JWT helper functions
- `.gitignore` - Excludes `test_services.json` from Git
