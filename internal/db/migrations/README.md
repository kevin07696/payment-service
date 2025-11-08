# Database Migrations

This directory contains database schema migrations managed by [Goose](https://github.com/pressly/goose).

## How It Works

- **Automatic via CI/CD**: Migrations run automatically as a separate GitHub Actions job before deployment
- **Pre-deployment**: Migrations complete successfully before the new app version deploys
- **Version tracking**: Goose tracks which migrations have been applied in the `goose_db_version` table
- **Idempotent**: Safe to run multiple times - already-applied migrations are skipped
- **Fast startup**: App starts immediately without waiting for migrations

## Creating New Migrations

### Using Goose CLI

```bash
# Install goose CLI (optional, for local development)
go install github.com/pressly/goose/v3/cmd/goose@latest

# Create a new SQL migration
goose -dir internal/db/migrations create add_new_table sql

# Create a new Go migration (for complex logic)
goose -dir internal/db/migrations create complex_migration go
```

### Manual Creation

Create a file following this pattern: `NNNNN_description.sql`

```sql
-- +goose Up
-- +goose StatementBegin
-- Your migration SQL here
CREATE TABLE example (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback SQL here
DROP TABLE IF EXISTS example;
-- +goose StatementEnd
```

## Migration Naming Convention

- Format: `NNNNN_description.sql` (e.g., `00002_add_users_table.sql`)
- Use sequential 5-digit numbers: `00001`, `00002`, `00003`, etc.
- Use descriptive names: `add_users_table`, `add_email_index`, `remove_legacy_columns`

## Running Migrations Locally

### Using Goose CLI Directly
```bash
# Run all pending migrations
goose -dir internal/db/migrations postgres "postgresql://localhost:5432/payment_service?sslmode=disable" up

# Rollback last migration
goose -dir internal/db/migrations postgres "postgresql://localhost:5432/payment_service?sslmode=disable" down

# Check migration status
goose -dir internal/db/migrations postgres "postgresql://localhost:5432/payment_service?sslmode=disable" status
```

### Via Fly.io Proxy (for staging database)
```bash
# Connect to staging database
flyctl proxy 5432 -a kevin07696-payment-service-staging-db

# In another terminal, run migrations
goose -dir internal/db/migrations postgres "postgresql://postgres:PASSWORD@localhost:5432/payment_service" up
```

## Migration Best Practices

1. **Keep migrations small**: One logical change per migration
2. **Test both up and down**: Ensure rollbacks work
3. **Use transactions**: Wrap DDL in transactions when possible (goose does this automatically)
4. **Avoid data changes**: Use separate migrations for schema vs data changes
5. **Be backward compatible**: New code should work with old schema (for zero-downtime deploys)
6. **Document breaking changes**: Add comments for migrations that require code changes

## Example Migrations

### Adding a Table
```sql
-- +goose Up
CREATE TABLE payments (
    id SERIAL PRIMARY KEY,
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS payments;
```

### Adding a Column
```sql
-- +goose Up
ALTER TABLE payments ADD COLUMN merchant_id UUID REFERENCES merchants(id);

-- +goose Down
ALTER TABLE payments DROP COLUMN IF EXISTS merchant_id;
```

### Adding an Index
```sql
-- +goose Up
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_payments_status;
DROP INDEX IF EXISTS idx_payments_created_at;
```

## Deployment Flow

1. **Development**: Create migration, test locally with goose CLI
2. **Commit**: Add migration file to git with code changes
3. **Push**: Push to `main` branch
4. **CI/CD Pipeline**:
   - ✅ Run tests
   - ✅ Build Docker image
   - ✅ **Run migrations** (separate job, fails if migrations fail)
   - ✅ Deploy app (only if migrations succeeded)
5. **Verify**: Check GitHub Actions logs for migration success

## Troubleshooting

### Migration Failed During CI/CD

Check the GitHub Actions logs:
- Go to: `https://github.com/kevin07696/payment-service/actions`
- Click on the failed workflow run
- Check the "Run Database Migrations (Staging)" job logs

The deployment will be blocked if migrations fail (app won't deploy).

### Fixing Failed Migrations

1. **Fix the migration file** in your repository
2. **Manual fix via Fly.io**:
   ```bash
   # Connect to database
   flyctl proxy 5432 -a kevin07696-payment-service-staging-db

   # Check goose version table
   psql -h localhost -U postgres -d payment_service
   SELECT * FROM goose_db_version;

   # If needed, manually mark migration as not applied
   DELETE FROM goose_db_version WHERE version_id = XXXXX;
   ```

### Reset All Migrations (DANGEROUS - Staging Only)

```bash
goose -dir internal/db/migrations postgres "DATABASE_URL" reset
```

This drops all tables and re-runs all migrations from scratch.

---

## Current Migrations

- `000_init_schema.sql` - Initial placeholder migration
- `001_customer_payment_methods.sql` - Payment methods and customer data tables
- `002_transactions.sql` - Transaction records and audit trail
- `003_chargebacks.sql` - Chargeback management system
- `004_agent_credentials.sql` - Agent authentication credentials
- `005_soft_delete_cleanup.sql` - Soft delete support across tables
- `006_pg_cron_jobs.sql.optional` - Optional pg_cron scheduled jobs for subscriptions
- `007_webhook_subscriptions.sql` - Outbound webhook system for merchant notifications
