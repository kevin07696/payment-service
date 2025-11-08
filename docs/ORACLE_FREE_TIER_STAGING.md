# Oracle Free Tier Setup for Staging Database

Use Oracle's Always Free Autonomous Database for your staging environment - it's perfect for low-traffic staging workloads and costs $0 forever.

## Why Oracle Free Tier for Staging?

✅ **Always Free** - No credit card expiration worries
✅ **20 GB Storage** - More than enough for staging
✅ **1 OCPU** - Sufficient for development/staging
✅ **PostgreSQL Compatible** - Works with pgx driver
✅ **Automatic Backups** - Built-in
✅ **No Time Limit** - Unlike trial credits

---

## Step 1: Create Oracle Cloud Account

1. Go to: https://www.oracle.com/cloud/free/
2. Click **Start for free**
3. Fill in account details:
   - Email
   - Country
   - Home Region (choose closest: e.g., US East, Canada, EU Frankfurt)
4. **Important**: Choose "Always Free" services
5. Verify email and complete signup

**Note**: No credit card required for Always Free tier

---

## Step 2: Create Autonomous Database

### 2.1 Navigate to Autonomous Database

1. Sign in to Oracle Cloud Console
2. Click hamburger menu (☰) → **Oracle Database** → **Autonomous Database**
3. Click **Create Autonomous Database**

### 2.2 Configure Database

**Basic Information:**
- **Compartment**: root (or create new compartment "payment-service-staging")
- **Display name**: `payment-service-staging-db`
- **Database name**: `paymentsvc` (max 14 chars, alphanumeric only)

**Workload Type:**
- Select: **Transaction Processing** (OLTP)

**Deployment Type:**
- Select: **Serverless**

**Configure the database:**
- **Always Free**: ✅ **CHECK THIS BOX** (critical!)
- **Choose database version**: 23ai (latest) or 19c
- **OCPU count**: 1 (fixed for Always Free)
- **Storage (TB)**: 0.02 TB (20 GB - fixed for Always Free)
- **Auto scaling**: Leave unchecked (not available for Always Free)

**Create administrator credentials:**
- **Username**: ADMIN (default, cannot change)
- **Password**: Create strong password (min 12 chars, uppercase, lowercase, number, special char)
  - Example: `PaymentSvc2025!`
  - **SAVE THIS PASSWORD** - you'll need it

**Choose network access:**
- Select: **Secure access from everywhere**
  - This allows Railway to connect
  - You can add IP allowlists later if needed

**License:**
- Select: **License Included**

3. Click **Create Autonomous Database**
4. Wait 2-3 minutes for provisioning

---

## Step 3: Download Wallet (Connection Credentials)

Oracle Autonomous Database uses wallet files for secure connections.

### 3.1 Download Wallet

1. Once database shows **Available** status, click on the database name
2. Click **Database connection** button
3. In the popup:
   - **Wallet type**: Instance Wallet
   - **Download**: Click **Download wallet**
   - **Password**: Create wallet password (can be same as DB password)
   - **Save password** - you'll need this
4. Save the `Wallet_paymentsvc.zip` file

### 3.2 Extract Wallet

```bash
cd ~/Documents/projects/payments
mkdir -p oracle-wallet
cd oracle-wallet
unzip ~/Downloads/Wallet_paymentsvc.zip
```

Files extracted:
- `tnsnames.ora` - Connection strings
- `sqlnet.ora` - SQL*Net configuration
- `cwallet.sso` - Auto-login wallet
- `ewallet.p12` - Wallet file
- `keystore.jks` - Java keystore
- `truststore.jks` - Trust store
- `ojdbc.properties` - JDBC properties

---

## Step 4: Get Connection String

### 4.1 Find Connection Strings

```bash
cat oracle-wallet/tnsnames.ora
```

You'll see multiple connection strings:
- `paymentsvc_high` - High priority, serial queries
- `paymentsvc_medium` - Medium priority, typical OLTP
- `paymentsvc_low` - Low priority, batch/reporting
- `paymentsvc_tp` - Transaction Processing (recommended for app)
- `paymentsvc_tpurgent` - Urgent/critical transactions

**Use `paymentsvc_tp` for your staging application.**

### 4.2 Extract Connection Details

From `tnsnames.ora`, find the `paymentsvc_tp` entry:

```
paymentsvc_tp = (description= (retry_count=20)(retry_delay=3)(address=(protocol=tcps)(port=1521)(host=adb.us-ashburn-1.oraclecloud.com))(connect_data=(service_name=abc123_paymentsvc_tp.adb.oraclecloud.com))(security=(ssl_server_dn_match=yes)))
```

Extract:
- **Host**: `adb.us-ashburn-1.oraclecloud.com`
- **Port**: `1521`
- **Service Name**: `abc123_paymentsvc_tp.adb.oraclecloud.com`

---

## Step 5: Test Local Connection

### 5.1 Install Oracle Instant Client (if not installed)

**Linux:**
```bash
# Download from: https://www.oracle.com/database/technologies/instant-client/downloads.html
# Or use package manager
sudo dnf install oracle-instantclient-sqlplus  # Fedora/RHEL
```

**macOS:**
```bash
brew install instantclient-basic
brew install instantclient-sqlplus
```

### 5.2 Set Environment Variables

```bash
export TNS_ADMIN=~/Documents/projects/payments/oracle-wallet
export LD_LIBRARY_PATH=/usr/lib/oracle/21/client64/lib:$LD_LIBRARY_PATH  # Adjust path
```

### 5.3 Test SQL*Plus Connection

```bash
sqlplus ADMIN@paymentsvc_tp
# Enter password when prompted
```

If connected successfully:
```sql
SQL> SELECT * FROM v$version;
SQL> EXIT;
```

---

## Step 6: Configure Go Application for Oracle

### 6.1 Install Oracle Driver

```bash
cd ~/Documents/projects/payments
go get github.com/godror/godror@latest
```

### 6.2 Update Database Connection Code

The Oracle Autonomous Database is PostgreSQL-compatible via the godror driver, but we can also use Oracle's native protocol.

**Option A: Use godror (Oracle native)**

Update `cmd/server/main.go` to detect Oracle and use godror:

```go
import (
    "github.com/godror/godror"
    _ "github.com/godror/godror"
)

func initDatabase(cfg *Config, logger *zap.Logger) (*pgxpool.Pool, error) {
    // Check if using Oracle
    if strings.Contains(cfg.DBHost, "oraclecloud.com") {
        return initOracleDatabase(cfg, logger)
    }

    // Existing PostgreSQL code
    // ...
}

func initOracleDatabase(cfg *Config, logger *zap.Logger) (*sql.DB, error) {
    // Oracle connection string
    connStr := fmt.Sprintf(`user="%s" password="%s" connectString="%s"`,
        cfg.DBUser,
        cfg.DBPassword,
        cfg.DBName, // This will be the service name from tnsnames
    )

    db, err := sql.Open("godror", connStr)
    if err != nil {
        return nil, err
    }

    return db, nil
}
```

**Option B: Use PostgreSQL Wire Protocol (Simpler)**

Oracle Autonomous Database supports PostgreSQL wire protocol on port 1522.

---

## Step 7: Set Up Railway Environment Variables

Configure Railway to connect to Oracle Free Tier database.

### 7.1 Connection String Format

For Oracle Autonomous Database:

```
DATABASE_URL=oracle://ADMIN:YOUR_PASSWORD@hostname:1521/service_name?wallet_location=/app/oracle-wallet
```

Example:
```
DATABASE_URL=oracle://ADMIN:PaymentSvc2025!@adb.us-ashburn-1.oraclecloud.com:1521/abc123_paymentsvc_tp.adb.oraclecloud.com?wallet_location=/app/oracle-wallet
```

### 7.2 Upload Wallet to Railway

Since Railway needs the wallet files, you have two options:

**Option A: Include wallet in Docker image** (Recommended)

1. Add wallet to `.dockerignore` exception:
```bash
# .dockerignore
!oracle-wallet/
```

2. Update Dockerfile:
```dockerfile
# Copy Oracle wallet
COPY oracle-wallet /app/oracle-wallet
ENV TNS_ADMIN=/app/oracle-wallet
```

**Option B: Use Railway Volumes** (if available)

Upload wallet files to Railway volume.

### 7.3 Set Railway Environment Variables

```bash
# Connect to Railway project
railway link

# Set database connection variables
railway variables set DB_HOST=adb.us-ashburn-1.oraclecloud.com
railway variables set DB_PORT=1521
railway variables set DB_USER=ADMIN
railway variables set DB_PASSWORD=PaymentSvc2025!
railway variables set DB_NAME=abc123_paymentsvc_tp.adb.oraclecloud.com
railway variables set DB_SSL_MODE=require
railway variables set TNS_ADMIN=/app/oracle-wallet
```

---

## Step 8: Update Goose Migrations for Oracle

Goose supports Oracle, but syntax may differ slightly.

### 8.1 Check Goose Oracle Support

```bash
goose -dir migrations oracle "user/password@hostname:port/service" status
```

### 8.2 Oracle-Specific Migration Considerations

**Differences from PostgreSQL:**
- Oracle uses `NUMBER` instead of `SERIAL`
- Oracle uses `VARCHAR2` instead of `VARCHAR`
- Oracle uses `DATE` or `TIMESTAMP` for datetime
- Sequences are explicit (not auto-created with SERIAL)

**Migration compatibility:**

If you want to keep PostgreSQL migrations compatible, use conditional syntax or create Oracle-specific migration files.

**Example**: Create both versions:
- `001_init.sql` (PostgreSQL)
- `001_init_oracle.sql` (Oracle)

---

## Step 9: Deploy to Railway with Oracle

### 9.1 Update Dockerfile for Oracle

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install Oracle Instant Client dependencies
RUN apk add --no-cache libaio libnsl libc6-compat

# Copy wallet
COPY oracle-wallet /app/oracle-wallet

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN go build -o /app/bin/server ./cmd/server

FROM alpine:latest

WORKDIR /app

# Install Oracle runtime dependencies
RUN apk add --no-cache libaio libnsl libc6-compat

# Copy binary and wallet
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/oracle-wallet /app/oracle-wallet

# Set Oracle environment
ENV TNS_ADMIN=/app/oracle-wallet

EXPOSE 8080 8081

CMD ["/app/server"]
```

### 9.2 Deploy

```bash
railway up --detach
railway logs --follow
```

---

## Step 10: Create Database Schema

### 10.1 Connect to Oracle and Create User

Oracle's ADMIN user is for administration. Create a separate user for your app:

```sql
-- Connect as ADMIN
sqlplus ADMIN@paymentsvc_tp

-- Create app user
CREATE USER payment_service IDENTIFIED BY "AppUser2025!";

-- Grant permissions
GRANT CONNECT, RESOURCE TO payment_service;
GRANT UNLIMITED TABLESPACE TO payment_service;
GRANT CREATE SESSION TO payment_service;
GRANT CREATE TABLE TO payment_service;
GRANT CREATE SEQUENCE TO payment_service;
GRANT CREATE TRIGGER TO payment_service;

EXIT;
```

### 10.2 Run Migrations as App User

```bash
# Update DB_USER in Railway
railway variables set DB_USER=payment_service
railway variables set DB_PASSWORD=AppUser2025!

# Or run locally first
export DB_USER=payment_service
export DB_PASSWORD=AppUser2025!
goose -dir migrations oracle "payment_service/AppUser2025!@paymentsvc_tp" up
```

---

## Monitoring and Management

### Check Database Status

```bash
# Via SQL*Plus
sqlplus ADMIN@paymentsvc_tp

SQL> SELECT * FROM v$pdbs;
SQL> SELECT tablespace_name, bytes/1024/1024 as MB FROM user_ts_quotas;
```

### View Metrics in Oracle Cloud

1. Go to Oracle Cloud Console
2. Navigate to Autonomous Database → your database
3. Click **Performance Hub** for real-time metrics

### Backup Management

Oracle Autonomous Database automatically backs up your database daily. Backups are retained for 60 days.

---

## Cost Management

**Always Free Tier Limits:**
- 2 Autonomous Databases (you're using 1)
- 20 GB storage per database
- 1 OCPU per database
- Always free - no expiration

**Monitoring usage:**
1. Oracle Cloud Console → Billing & Cost Management
2. Check "Always Free Resources" status
3. Ensure database shows "Always Free" label

**Warning Signs:**
- If you accidentally upgrade to paid tier, you'll see charges
- Monitor that "Always Free" checkbox stays enabled

---

## Troubleshooting

### Issue: "Wallet not found"

**Solution:**
```bash
# Verify wallet is in Docker image
railway run ls -la /app/oracle-wallet

# Verify TNS_ADMIN is set
railway run env | grep TNS_ADMIN
```

### Issue: "ORA-12154: TNS:could not resolve the connect identifier"

**Solution:**
Check `tnsnames.ora` exists and service name matches:
```bash
railway run cat /app/oracle-wallet/tnsnames.ora
```

### Issue: "Connection timeout"

**Solution:**
1. Check network access is set to "Secure access from everywhere"
2. Verify Railway can reach Oracle Cloud (test with curl):
```bash
railway run nc -zv adb.us-ashburn-1.oraclecloud.com 1521
```

### Issue: "ORA-01017: invalid username/password"

**Solution:**
- Verify password doesn't have special shell characters
- Try escaping password in connection string
- Reset password in Oracle Cloud Console

---

## Alternative: PostgreSQL Wire Protocol

Oracle Autonomous Database supports PostgreSQL wire protocol on port **1522**.

### Using pgx with Oracle

```bash
# Set environment variable
railway variables set DATABASE_URL="postgresql://ADMIN:password@host:1522/service_name?sslmode=require"
```

**Pros:**
- No Oracle driver needed
- Use existing pgx code
- Simpler deployment

**Cons:**
- Limited to PostgreSQL-compatible features
- May not support all Oracle-specific features

---

## Summary

You now have:

✅ Oracle Free Tier Autonomous Database (Always Free)
✅ 20 GB storage for staging
✅ Automatic backups
✅ Wallet-based secure connection
✅ Railway deployment configured

**Next Steps:**
1. Test connection locally
2. Deploy to Railway
3. Run migrations
4. Verify application works with Oracle

**Cost**: $0/month forever (Always Free)

**Resources:**
- Oracle Free Tier: https://www.oracle.com/cloud/free/
- Oracle Autonomous Database Docs: https://docs.oracle.com/en/cloud/paas/autonomous-database/
- godror Driver: https://github.com/godror/godror
