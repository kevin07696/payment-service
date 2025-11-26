# Admin CLI Documentation

The admin CLI manages services, merchants, and access control for the payment service.

---

## Quick Reference

| Action | Command |
|--------|---------|
| Create service | `./admin -action=create-service` |
| Create merchant | `./admin -action=create-merchant` |
| Grant access | `./admin -action=grant-access` |
| **Generate token** | `./admin -action=generate-token -c creds.json` |
| List services | `./admin -action=list-services` |
| List merchants | `./admin -action=list-merchants` |

All commands support interactive mode (no flags) or JSON file input (`-json=file.json`).
Token generation requires credentials file (`-c` or `--credentials`).

---

## Docker Usage

After `docker-compose up -d` or `podman-compose up -d`, run commands inside the container:

```bash
# Create a service (interactive)
podman exec -it payment-server ./admin -action=create-service

# Create a service (with JSON file)
podman exec payment-server sh -c 'cat > service.json << EOF
{
  "service_id": "my-app",
  "service_name": "My Application",
  "environment": "staging",
  "generate_keypair": true
}
EOF'
podman exec payment-server ./admin -action=create-service -json=service.json

# Create a merchant (interactive)
podman exec -it payment-server ./admin -action=create-merchant

# Create a merchant (with JSON file)
podman exec payment-server sh -c 'cat > merchant.json << EOF
{
  "slug": "my-merchant",
  "name": "My Merchant LLC",
  "cust_nbr": "123456",
  "merch_nbr": "987654",
  "dba_nbr": "001",
  "terminal_nbr": "001",
  "mac_secret_path": "secrets/epx/staging/mac_secret",
  "environment": "staging"
}
EOF'
podman exec payment-server ./admin -action=create-merchant -json=merchant.json

# Grant access (interactive)
podman exec -it payment-server ./admin -action=grant-access

# List services/merchants
podman exec payment-server ./admin -action=list-services
podman exec payment-server ./admin -action=list-merchants

# Copy credentials to host
podman cp payment-server:/home/appuser/service_my-app_credentials.json .

# Generate JWT token (after copying credentials)
./bin/admin -action=generate-token -c service_my-app_credentials.json
```

**Note:** Replace `podman` with `docker` if using Docker.

---

## Token Generation

Generate JWT tokens from service credentials for API authentication. No database connection required.

### Basic Usage

```bash
# Generate token with default settings (1h expiry, all scopes)
./bin/admin -action=generate-token -c service_my-app_credentials.json

# Custom expiry duration
./bin/admin -action=generate-token -c creds.json -e 30m

# Specific scopes only
./bin/admin -action=generate-token -c creds.json -s "payments:create,payments:read"

# Output as JSON (includes metadata)
./bin/admin -action=generate-token -c creds.json -o json

# Output as curl command (ready to use)
./bin/admin -action=generate-token -c creds.json -o curl

# Show decoded claims (for verification)
./bin/admin -action=generate-token -c creds.json --decode
```

### Token Generation Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--credentials` | `-c` | Service credentials JSON file | (required) |
| `--expires` | `-e` | Token expiry duration | `1h` |
| `--scopes` | `-s` | Comma-separated scopes | all scopes |
| `--output` | `-o` | Output format: `token`, `json`, `curl` | `token` |
| `--decode` | | Show decoded claims | false |

### Available Scopes

| Scope | Description |
|-------|-------------|
| `payments:create` | Create payments (auth, capture, sale) |
| `payments:read` | View transactions |
| `payments:void` | Void payments |
| `payments:refund` | Issue refunds |
| `payment_methods:read` | View saved payment methods |
| `payment_methods:create` | Store payment methods |
| `subscriptions:manage` | Manage subscriptions |
| `subscriptions:read` | View subscriptions |

### Complete Workflow Example

```bash
# 1. Create service (saves credentials file)
podman exec -it payment-server ./admin -action=create-service

# 2. Copy credentials to host
podman cp payment-server:/home/appuser/service_my-app_credentials.json .

# 3. Generate token
TOKEN=$(./bin/admin -action=generate-token -c service_my-app_credentials.json)

# 4. Use token in API request
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"merchant_id": "your-merchant-uuid", "limit": 50}'
```

---

## Local Usage

### Prerequisites

1. Build the CLI:
   ```bash
   go build -o bin/admin ./cmd/admin
   ```

2. Set database connection (choose one):
   ```bash
   # Option A: Environment variable
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"

   # Option B: Use -db flag with each command
   ./bin/admin -action=list-services -db="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"
   ```

### Commands

```bash
# Create a service (interactive)
./bin/admin -action=create-service

# Create a service (with JSON file)
./bin/admin -action=create-service -json=service.json

# Create a merchant (interactive)
./bin/admin -action=create-merchant

# Create a merchant (with JSON file)
./bin/admin -action=create-merchant -json=merchant.json

# Grant access (interactive)
./bin/admin -action=grant-access

# List services/merchants
./bin/admin -action=list-services
./bin/admin -action=list-merchants
```

---

## JSON File Formats

### service.json

```json
{
  "service_id": "my-app",
  "service_name": "My Application",
  "environment": "staging",
  "requests_per_second": 1000,
  "burst_limit": 2000,
  "generate_keypair": true
}
```

### merchant.json

```json
{
  "slug": "my-merchant",
  "name": "My Merchant LLC",
  "cust_nbr": "123456",
  "merch_nbr": "987654",
  "dba_nbr": "001",
  "terminal_nbr": "001",
  "mac_secret_path": "secrets/epx/staging/mac_secret",
  "environment": "staging",
  "tier": "standard",
  "requests_per_second": 100
}
```

---

## Output Files

After creating entities, credentials files are saved:

| Entity | File | Contents |
|--------|------|----------|
| Service | `service_<id>_credentials.json` | RSA private key for JWT signing |
| Merchant | `merchant_<slug>_info.json` | Merchant configuration |

**Important:** Keep the service credentials file secure - the private key is only shown once!

---

## Complete Setup Example

### Docker

```bash
# 1. Start containers
podman-compose up -d

# 2. Create service
podman exec -it payment-server ./admin -action=create-service
# Enter: my-app, My Application, staging, defaults...

# 3. Create merchant
podman exec -it payment-server ./admin -action=create-merchant
# Enter: my-merchant, My Merchant, 123456, 987654, 001, 001...

# 4. Grant access
podman exec -it payment-server ./admin -action=grant-access
# Enter: my-app, my-merchant

# 5. Copy credentials
podman cp payment-server:/home/appuser/service_my-app_credentials.json .
```

### Local

```bash
# 1. Start PostgreSQL and run migrations
docker run -d --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15-alpine
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"
goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# 2. Build CLI
go build -o bin/admin ./cmd/admin

# 3. Create service
./bin/admin -action=create-service

# 4. Create merchant
./bin/admin -action=create-merchant

# 5. Grant access
./bin/admin -action=grant-access
```

---

## Scopes

When granting access, these scopes are assigned:

| Scope | Description |
|-------|-------------|
| `payment:create` | Create payment transactions |
| `payment:read` | Read payment transactions |
| `payment:update` | Update payment status |
| `payment:refund` | Refund payments |
| `subscription:manage` | Manage subscriptions |
| `payment_method:manage` | Manage saved payment methods |

---

## Troubleshooting

### "relation services does not exist"

**Cause:** Database migrations haven't run yet.

**Solution (Docker):** Wait for container to be healthy:
```bash
# Check if ready
curl http://localhost:8081/cron/health
```

**Solution (Local):** Run migrations:
```bash
goose -dir internal/db/migrations postgres "$DATABASE_URL" up
```

### "Database URL not provided"

**Solution:** Set `DATABASE_URL` or use `-db` flag:
```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"
# or
./bin/admin -action=list-services -db="postgres://..."
```

### Service already exists

**Solution:** List existing services or delete and recreate:
```bash
./bin/admin -action=list-services
# If needed, delete via psql:
psql $DATABASE_URL -c "DELETE FROM services WHERE service_id = 'my-service'"
```

---

## Related Documentation

- [SETUP.md](../development/SETUP.md) - Environment setup
- [AUTH.md](../development/AUTH.md) - Authentication architecture
- [API_SPECS.md](./API_SPECS.md) - API documentation
