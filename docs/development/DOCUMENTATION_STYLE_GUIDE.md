# Documentation Style Guide

**Last Updated:** 2025-11-21
**Owner:** Development Team
**Purpose:** Ensure consistency, discoverability, and maintainability across all documentation

---

## Table of Contents

1. [Documentation Principles](#documentation-principles)
2. [Document Structure](#document-structure)
3. [Directory Organization](#directory-organization)
4. [Writing Standards](#writing-standards)
5. [Code Examples](#code-examples)
6. [Troubleshooting Format](#troubleshooting-format)
7. [Security Documentation](#security-documentation)
8. [Versioning Strategy](#versioning-strategy)
9. [Automation](#automation)
10. [Documentation Checklist](#documentation-checklist)
11. [Templates](#templates)

---

## Documentation Principles

### 1. Audience-First Thinking

**Every document MUST start with these three lines:**

```markdown
**Target Audience:** [Who is this for?]
**Topic:** [What does it cover?]
**Goal:** [What should they accomplish?]
```

**Examples:**

```markdown
**Target Audience:** Frontend developers integrating payment functionality
**Topic:** React integration with ConnectRPC payment APIs
**Goal:** Accept payments in a React application within 30 minutes
```

```markdown
**Target Audience:** DevOps engineers, platform administrators
**Topic:** Admin CLI for managing services and merchants
**Goal:** Set up authentication and merchant accounts securely
```

### 2. Show AND Tell

Use both code examples and explanations, but explanations must add context.

**Good:**
```markdown
### Create a Service

A service represents an application (POS, e-commerce site, mobile app) that authenticates using RSA keypairs.

```bash
# Generate RSA keypair and register service
./admin -action=create-service

# Parameters:
# --service-id: Unique identifier (e.g., acme-pos-system)
# --environment: production|staging
# --rate-limit: Requests per second (default: 1000)
```
```

**Bad:**
```markdown
### Create a Service

```bash
./admin -action=create-service
```
```

**Rule:** If a parameter isn't self-explanatory, document it inline or in a parameter table.

### 3. Practical Examples Over Pure Reference

Different document types need different approaches:

| Document Type | Approach | Example |
|---------------|----------|---------|
| **Integration Guide** | Quick Start (5 min to first success) | ADMIN_CLI.md, REACT_INTEGRATION.md |
| **API Reference** | Dictionary + curl examples with responses | API_SPECS.md |
| **Architecture Docs** | Diagrams + context + why decisions were made | AUTH.md |
| **Troubleshooting** | Issue → Root Cause → Evidence → Solution | All docs |

**API Reference Example:**

```markdown
### POST /api/v1/payment/sale

**Purpose:** Create a one-time payment transaction

**Request:**
```bash
curl -X POST https://api.example.com/v1/payment/sale \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount_cents": 10000,
    "payment_method_id": "pm_abc123",
    "idempotency_key": "sale_1234567890"
  }'
```

**Response (Success - 200):**
```json
{
  "transaction_id": "tx_xyz789",
  "status": "TRANSACTION_STATUS_APPROVED",
  "auth_code": "123456"
}
```

**Response (Declined - 200):**
```json
{
  "transaction_id": "tx_xyz790",
  "status": "TRANSACTION_STATUS_DECLINED",
  "decline_reason": "insufficient_funds"
}
```

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| amount_cents | int64 | Yes | Amount in cents (e.g., 10000 = $100.00) |
| payment_method_id | string | Yes | Tokenized payment method |
| idempotency_key | string | Yes | UUID for duplicate prevention |
```

### 4. Working Backward from User Goals

Structure documents starting with what users want to accomplish:

```markdown
## Common Tasks

**I want to...**
- → [Accept a one-time payment](#accept-payment)
- → [Set up recurring billing](#recurring-billing)
- → [Refund a transaction](#refund)
- → [Handle declined cards](#handle-declines)
```

Then provide the step-by-step path to accomplish it.

---

## Document Structure

### Standard Template

```markdown
# [Document Title]

**Target Audience:** [Who]
**Topic:** [What]
**Goal:** [Why]

---

## Overview

[2-3 sentence summary of what this document covers]

**Key Concepts:**
- Concept 1
- Concept 2
- Concept 3

---

## Quick Start (if applicable)

[5-minute path to first success]

---

## [Main Content Sections]

---

## Troubleshooting

### [Common Issue 1]
**Issue:** [What the user sees]
**Root Cause:** [Why it happens]
**Evidence:** [Log snippets, error codes]
**Solution:** [Step-by-step fix]
**Prevention:** [How to avoid]

---

## Related Documentation

- **[Doc 1](./path.md)** - Brief description
- **[Doc 2](./path.md)** - Brief description
```

### Header Hierarchy

```markdown
# H1: Document Title (only one per document)
## H2: Major Sections
### H3: Sub-sections
#### H4: Rare (only if absolutely necessary)
```

**Never skip levels** (don't go from H2 to H4).

---

## Directory Organization

### Final Structure (Post-Development)

```
docs/
├── integration/           # External developers (HOW TO USE)
│   ├── ADMIN_CLI.md      # Setting up services/merchants
│   ├── API_SPECS.md      # Complete API reference
│   ├── REACT_INTEGRATION.md
│   ├── BROWSER_POST_REFERENCE.md
│   ├── TOKEN_GENERATION.md
│   └── MODULE_INTEGRATION.md
│
├── development/          # Internal developers (HOW TO BUILD/MAINTAIN)
│   ├── AUTH.md          # Authentication architecture
│   ├── SETUP.md         # Development environment setup
│   ├── DEVELOP.md       # Development workflows
│   ├── DATABASE.md      # Schema and migrations
│   └── TESTING.md       # Test strategy
│
├── DOCUMENTATION_STYLE_GUIDE.md  # This file
└── CHANGELOG.md         # All changes (includes archived docs)
```

### Migration Plan

**Current state:**
- `docs/optimizations/` → Summarize key findings in CHANGELOG, delete directory
- `docs/refactor/` → Summarize completed work in CHANGELOG, delete directory
- `docs/archive/` → Already archived, keep as-is or delete
- `docs/reports/` → Merge into CHANGELOG, delete directory

**Rationale:**
- Two clear categories: Integration (external) vs Development (internal)
- Completed work → CHANGELOG (historical record)
- Active documentation → integration/ or development/
- Reduces navigation complexity

---

## Writing Standards

### Tone and Voice

**DO:**
- Use active voice: "Create a service" (not "A service is created")
- Use present tense: "The API returns..." (not "The API will return...")
- Be direct: "Run this command" (not "You should run this command")
- Use "we" sparingly, prefer "you" when addressing the reader

**Examples:**

✅ **Good:**
```markdown
Run the admin CLI to create a service:
```bash
./admin -action=create-service
```
The command generates an RSA keypair and returns the private key.
```

❌ **Bad:**
```markdown
The admin CLI should be run to create a service. A service will be created
and you will be given a private key that was generated by the system.
```

### Formatting

**Code Blocks:**
- Always specify language for syntax highlighting
- Include comments for non-obvious steps
- Show expected output when helpful

```markdown
```bash
# Install dependencies
go mod download

# Expected output:
# go: downloading github.com/...
```
```

**Command Parameters:**
- Use tables for 3+ parameters
- Inline lists for 1-2 parameters

**Emphasis:**
- **Bold** for UI elements, filenames, important warnings
- `Code` for commands, code references, environment variables
- *Italic* rarely (avoid unless truly necessary)

**Lists:**
- Use bullets for unordered items
- Use numbers for sequential steps
- Use checkboxes for checklists

**Links:**
- Use descriptive text: `[Admin CLI Guide](./ADMIN_CLI.md)`
- Not: `[Click here](./ADMIN_CLI.md)` or `[Link](./ADMIN_CLI.md)`

### Code References

When referencing specific code locations, use the pattern:

```markdown
The JWT validation happens in `internal/auth/jwt_utils.go:110-115`:
```

This allows readers to navigate directly to the source.

---

## Code Examples

### Bash/Shell Commands

**Pattern:**
```bash
# 1. Description of what this does
command --flag=value

# 2. Next step
another-command

# Expected output:
# ✅ Success message here
```

**Multi-step workflows:**
```bash
# Complete workflow example
# 1. Create service
./admin -action=create-service
# → Returns private key (save it!)

# 2. Create merchant
./admin -action=create-merchant
# → Stores EPX credentials

# 3. Grant access
./admin -action=grant-access
# → Service can now access merchant
```

### API Examples (curl)

**Always show:**
1. Complete request with headers
2. Success response
3. Common error responses

```bash
# Sale transaction
curl -X POST https://api.example.com/v1/payment/sale \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount_cents": 10000,
    "payment_method_id": "pm_abc123",
    "idempotency_key": "sale_1234567890"
  }'

# Response (200 OK - Approved):
{
  "transaction_id": "tx_xyz789",
  "status": "TRANSACTION_STATUS_APPROVED",
  "auth_code": "123456",
  "amount_cents": 10000
}

# Response (200 OK - Declined):
{
  "transaction_id": "tx_xyz790",
  "status": "TRANSACTION_STATUS_DECLINED",
  "decline_reason": "insufficient_funds",
  "decline_code": "05"
}

# Response (400 Bad Request):
{
  "error": "invalid_request",
  "message": "idempotency_key is required"
}
```

### Go Code Examples

```go
// Service generates JWT token for merchant
func (s *POSService) GenerateToken(merchantID string) (string, error) {
    // Load private key from environment
    privateKeyPEM, err := os.ReadFile(os.Getenv("PRIVATE_KEY_PATH"))
    if err != nil {
        return "", fmt.Errorf("failed to load private key: %w", err)
    }

    // Create JWT manager
    jwtManager, err := auth.NewJWTManager(
        privateKeyPEM,
        "acme-pos-system",  // service_id
        8 * time.Hour,       // token expiry
    )
    if err != nil {
        return "", err
    }

    // Generate token
    return jwtManager.GenerateToken(merchantID, []string{
        "payment:create",
        "payment:read",
    })
}
```

**Rules:**
- Include imports if not obvious
- Add comments for business logic
- Show error handling
- Use realistic variable names

### React/TypeScript Examples

```typescript
// Payment form with idempotency
function CheckoutForm() {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      // Generate unique idempotency key
      const idempotencyKey = `sale_${Date.now()}_${crypto.randomUUID()}`;

      const response = await paymentClient.Sale({
        amountCents: BigInt(10000), // $100.00
        paymentMethodId: 'pm_abc123',
        idempotencyKey,
      });

      if (response.status === 'approved') {
        // Handle success
        console.log('Payment approved:', response.transactionId);
      }
    } catch (error) {
      // Handle error
      console.error('Payment failed:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <button type="submit" disabled={loading}>
        {loading ? 'Processing...' : 'Pay $100.00'}
      </button>
    </form>
  );
}
```

---

## Troubleshooting Format

### Standard Template

```markdown
### [Issue Name/Error Message]

**Issue:** What the user sees (exact error message or symptom)

**Root Cause:** Technical explanation of why this happens

**Evidence:** Log snippets, error codes, symptoms
```
ERROR: column "requests_per_second" of relation "merchants" does not exist
LINE 5: is_active, status, tier, requests_per_second, burst_limit
```

**Solution:**
1. First step with command
2. Second step with expected output
3. Verification step

**Prevention:**
- How to avoid this issue in the future
- Configuration changes needed
- Best practices to follow
```

### Example

```markdown
### "Failed to validate JWT signature"

**Issue:** API returns 401 Unauthorized with message "invalid token signature"

**Root Cause:** The public key stored in the database doesn't match the private key used to sign the JWT token. This happens when:
1. Using wrong private key
2. Service was recreated but client still has old private key
3. Public key corrupted in database

**Evidence:**
```
2025-11-21 14:30:00 ERROR [auth] JWT validation failed
  service_id: acme-pos-system
  error: crypto/rsa: verification error
  public_key_fingerprint: SHA256:abc123...
```

**Solution:**

1. **Verify public key fingerprint in database:**
```bash
psql $DATABASE_URL -c "
  SELECT service_id, public_key_fingerprint
  FROM services
  WHERE service_id = 'acme-pos-system'
"
```

2. **Compare with your private key:**
```bash
openssl rsa -in keys/acme-pos-system.pem -pubout | openssl dgst -sha256
```

3. **If fingerprints don't match, recreate service:**
```bash
# Delete old service
psql $DATABASE_URL -c "DELETE FROM services WHERE service_id = 'acme-pos-system'"

# Create new service
./admin -action=create-service

# Update client application with new private key
```

**Prevention:**
- Store private keys in secure location with backups
- Use secret manager (GCP/AWS/Vault) for production
- Never recreate services unless absolutely necessary
- Document which private key belongs to which service
```

---

## Security Documentation

### Inline Security Warnings

Use inline warnings where security-sensitive actions occur:

**Pattern:**
```markdown
⚠️  **SECURITY:** [What could go wrong] [What to do instead]
```

**Examples:**

```markdown
### Private Key Storage

⚠️  **SECURITY:** Never commit private keys to Git or store in database.
Store in secret manager (GCP Secret Manager, AWS Secrets Manager, Vault).

❌ **DON'T:**
- Store private keys in database
- Commit private keys to version control
- Share private keys via email/Slack
- Store unencrypted on filesystem

✅ **DO:**
```bash
# Save with restricted permissions
cat > keys/service-private.pem <<EOF
-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----
EOF
chmod 600 keys/service-private.pem

# Add to .gitignore
echo "keys/*.pem" >> .gitignore

# For production: Use secret manager
gcloud secrets create service-private-key \
  --data-file=keys/service-private.pem
```
```

### Security Checklist Format

```markdown
**Security Checklist:**
- [ ] Private keys stored in secret manager (not filesystem)
- [ ] MAC secrets retrieved from secret manager (not environment variables)
- [ ] Database credentials rotated every 90 days
- [ ] JWT tokens expire within 8 hours
- [ ] All API calls use HTTPS/TLS
- [ ] No sensitive data logged (card numbers, CVV, passwords)
```

---

## Versioning Strategy

### Recommendation: Semantic Versioning + Date Stamping

**For the Payment Service:**

1. **Code Versioning:** Use semantic versioning (v1.0.0, v1.1.0, v2.0.0)
2. **Documentation Versioning:** Track with git tags matching code versions
3. **Current Docs:** Always reflect `main` branch (latest stable)
4. **Historical Docs:** Accessible via git tags

**Implementation:**

```bash
# When releasing v1.0.0
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Documentation at this point is frozen in history
# View old docs:
git checkout v1.0.0
cat docs/integration/API_SPECS.md
```

**CHANGELOG.md Pattern:**

```markdown
## [1.1.0] - 2025-11-21

### Added
- Admin CLI for service management (`docs/integration/ADMIN_CLI.md`)
- RSA keypair authentication for services

### Changed
- Moved admin CLI docs to integration/ (from development/)

### Deprecated
- API key authentication (use RSA keypairs instead)

### Removed
- `internal/auth/api_key.go` (dead code)

### Fixed
- Table name mismatch in admin CLI (registered_services → services)

### Security
- Private keys no longer stored in database
```

**Documentation Header (Optional):**

For rapidly changing docs, add a version indicator:

```markdown
# Admin CLI Guide

**Documentation Version:** 1.1.0 (matches service v1.1.0)
**Last Updated:** 2025-11-21
**Status:** Current
```

**Branch Strategy:**

```
main              → Current stable docs
develop           → Next version docs (breaking changes)
feature/*         → Feature-specific doc updates
docs/*            → Doc-only updates (typos, clarifications)
```

**When to Version Docs:**

| Change Type | Version Update? | Example |
|-------------|-----------------|---------|
| Typo fix | No | Fix spelling in README |
| Clarification | No | Add example to existing section |
| New feature | Yes (minor) | Add new API endpoint |
| Breaking change | Yes (major) | Change authentication method |
| Deprecation | Yes (minor) | Mark feature as deprecated |

---

## Automation

### Recommendation: Hybrid Approach

**Auto-Generate:**
1. ✅ API documentation from proto files
2. ✅ Code snippets from integration tests
3. ✅ Database schema reference
4. ✅ Changelog from git commits (curated)

**Hand-Write:**
1. ✅ Architecture explanations (why decisions were made)
2. ✅ Troubleshooting guides
3. ✅ Integration guides
4. ✅ Security best practices

### 1. Auto-Generate API Specs

**Tool:** `protoc-gen-doc` or custom script

```bash
# Install protoc-gen-doc
go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest

# Generate API docs
protoc --doc_out=./docs/integration --doc_opt=markdown,API_SPECS.md \
  proto/payment/v1/payment.proto \
  proto/subscription/v1/subscription.proto \
  proto/payment_method/v1/payment_method.proto
```

**Custom Script Pattern:**

```go
// scripts/generate_api_docs.go
package main

import (
    "fmt"
    "os"
    "text/template"
)

const apiTemplate = `# API Specification

**Auto-Generated:** {{.Timestamp}}
**Proto Version:** {{.Version}}

---

{{range .Services}}
## {{.Name}} Service

{{.Description}}

{{range .Methods}}
### {{.Name}}

**Request:**
` + "```protobuf" + `
{{.RequestType}}
` + "```" + `

**Response:**
` + "```protobuf" + `
{{.ResponseType}}
` + "```" + `

**Example:**
` + "```bash" + `
{{.ExampleCurl}}
` + "```" + `
{{end}}
{{end}}
`

// Parse proto files, extract service definitions, generate docs
```

### 2. Generate Code Snippets from Integration Tests

**Pattern:** Embed test code as documentation examples

```go
// tests/integration/examples/sale_test.go
//go:build integration
// +build integration

package examples

import (
    "testing"
)

// Example: Create a one-time payment (Sale)
//
// This example demonstrates how to create a sale transaction
// using the payment service API.
//
// Output: docs/integration/examples/sale.md
func ExamplePaymentClient_Sale() {
    // Setup
    client := testutil.NewPaymentClient()

    // Create sale
    response, err := client.Sale(ctx, &payment.SaleRequest{
        AmountCents:     10000,
        PaymentMethodId: "pm_abc123",
        IdempotencyKey:  testutil.GenerateIdempotencyKey("sale"),
    })

    if err != nil {
        panic(err)
    }

    fmt.Printf("Transaction ID: %s\n", response.TransactionId)
    fmt.Printf("Status: %s\n", response.Status)
    // Output:
    // Transaction ID: tx_xyz789
    // Status: approved
}
```

**Extract to docs:**

```bash
# Run tests with example output
go test -v -tags=integration ./tests/integration/examples > docs/integration/API_EXAMPLES.md
```

### 3. Database Schema Auto-Documentation

```bash
# Generate schema docs from migrations
#!/bin/bash

echo "# Database Schema" > docs/development/DATABASE_SCHEMA.md
echo "" >> docs/development/DATABASE_SCHEMA.md
echo "**Auto-Generated:** $(date)" >> docs/development/DATABASE_SCHEMA.md
echo "" >> docs/development/DATABASE_SCHEMA.md

for migration in internal/db/migrations/*.sql; do
    echo "## $(basename $migration)" >> docs/development/DATABASE_SCHEMA.md
    echo "" >> docs/development/DATABASE_SCHEMA.md
    echo '```sql' >> docs/development/DATABASE_SCHEMA.md
    cat "$migration" >> docs/development/DATABASE_SCHEMA.md
    echo '```' >> docs/development/DATABASE_SCHEMA.md
    echo "" >> docs/development/DATABASE_SCHEMA.md
done
```

### 4. Automation Schedule

**Trigger auto-generation on:**
- ✅ Pre-commit hook (validate docs are up to date)
- ✅ CI/CD pipeline (generate and commit if changes)
- ✅ Manual command (`make docs`)

```makefile
# Makefile
.PHONY: docs
docs: docs-api docs-schema docs-examples
	@echo "✅ Documentation generated"

docs-api:
	@echo "Generating API docs from proto files..."
	protoc --doc_out=./docs/integration --doc_opt=markdown,API_SPECS_GENERATED.md proto/**/*.proto

docs-schema:
	@echo "Generating database schema docs..."
	./scripts/generate_schema_docs.sh

docs-examples:
	@echo "Extracting code examples from tests..."
	go test -v -tags=integration ./tests/integration/examples > docs/integration/API_EXAMPLES.md

docs-validate:
	@echo "Validating documentation..."
	@./scripts/validate_docs.sh
```

**Pre-commit hook:**

```bash
# .git/hooks/pre-commit
#!/bin/bash

# Auto-generate docs
make docs

# Check for changes
if ! git diff --quiet docs/; then
    echo "❌ Documentation is out of date. Run 'make docs' and commit changes."
    exit 1
fi

echo "✅ Documentation is up to date"
```

### 5. GitHub Wiki Sync

**Recommendation:** Use a GitHub Action to sync docs to wiki

```yaml
# .github/workflows/sync-docs-to-wiki.yml
name: Sync Docs to Wiki

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'README.md'

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Checkout wiki
        uses: actions/checkout@v3
        with:
          repository: ${{ github.repository }}.wiki
          path: wiki

      - name: Sync integration docs
        run: |
          cp docs/integration/*.md wiki/
          cp README.md wiki/Home.md

      - name: Commit and push to wiki
        run: |
          cd wiki
          git config user.name "GitHub Actions"
          git config user.email "actions@github.com"
          git add .
          git commit -m "Sync from main: ${{ github.sha }}" || exit 0
          git push
```

**Manual sync script:**

```bash
#!/bin/bash
# scripts/sync_to_wiki.sh

# Clone wiki repo
git clone https://github.com/kevin07696/payment-service.wiki.git /tmp/wiki

# Copy docs
cp docs/integration/*.md /tmp/wiki/
cp README.md /tmp/wiki/Home.md

# Commit and push
cd /tmp/wiki
git add .
git commit -m "Sync documentation: $(date)"
git push

# Cleanup
rm -rf /tmp/wiki
```

---

## Documentation Checklist

### When Creating New Documentation

- [ ] Added target audience, topic, and goal at the top
- [ ] Included overview/summary (2-3 sentences)
- [ ] Added quick start or practical example within first 3 sections
- [ ] Used code blocks with syntax highlighting and comments
- [ ] Included troubleshooting section (if applicable)
- [ ] Added related documentation links at the bottom
- [ ] Checked all links work (relative paths correct)
- [ ] Ran spell check
- [ ] Validated code examples actually work
- [ ] Added to appropriate directory (integration/ or development/)
- [ ] Updated README.md with link (if it's a major guide)

### When Updating Existing Documentation

- [ ] Verified change doesn't contradict other docs
- [ ] Updated related documentation that references this doc
- [ ] Added entry to CHANGELOG.md
- [ ] Checked if code examples still work
- [ ] Updated any version numbers or dates (if applicable)
- [ ] Ran `make docs` to regenerate auto-docs (if applicable)

### When Deprecating Features

- [ ] Marked feature as deprecated in documentation
- [ ] Added migration guide (old → new approach)
- [ ] Set removal date (e.g., "Will be removed in v2.0.0")
- [ ] Added deprecation notice to CHANGELOG.md
- [ ] Updated all examples to use new approach
- [ ] Added inline warnings where deprecated feature is mentioned

### Before Release

- [ ] All documentation reviewed and up-to-date
- [ ] CHANGELOG.md reflects all changes
- [ ] README.md quick start tested end-to-end
- [ ] Integration guides tested with fresh environment
- [ ] API examples tested against actual API
- [ ] Troubleshooting section covers known issues
- [ ] Git tag created matching version
- [ ] Wiki synced (if applicable)

---

## Quality Metrics

### Measuring Documentation Health

**Recommended Metrics to Track:**

1. **Freshness**
   - ✅ Track last updated date in git metadata
   - ✅ Flag docs not updated in 6+ months
   - ✅ Correlate with code changes (if code changed but docs didn't, flag it)

2. **Completeness**
   - ✅ Every public API endpoint has example
   - ✅ Every integration guide has working code
   - ✅ Every configuration option is documented

3. **Accuracy**
   - ✅ Integration tests validate example code works
   - ✅ API examples run in CI
   - ✅ Schema docs match actual migrations

4. **Discoverability**
   - ✅ Can find answer in <3 clicks from README
   - ✅ Search functionality (GitHub wiki search)
   - ✅ Clear navigation structure

5. **User Feedback** (Future)
   - ✅ "Was this helpful?" button on wiki pages
   - ✅ Track support questions (what docs are missing?)
   - ✅ Community contributions (PRs to docs)

**Automated Checks:**

```bash
#!/bin/bash
# scripts/validate_docs.sh

echo "🔍 Validating documentation..."

# 1. Check for broken links
echo "Checking for broken links..."
find docs -name "*.md" -exec markdown-link-check {} \;

# 2. Check for TODO/FIXME
echo "Checking for TODOs..."
if grep -r "TODO\|FIXME" docs/; then
    echo "❌ Found TODOs in documentation"
    exit 1
fi

# 3. Validate code blocks
echo "Validating shell commands..."
# Extract bash blocks, check for common mistakes
grep -A 20 '```bash' docs/**/*.md | grep -E 'cd|rm -rf /' && echo "❌ Dangerous command found"

# 4. Check required headers
echo "Checking for required headers..."
for doc in docs/integration/*.md docs/development/*.md; do
    if ! grep -q "Target Audience:" "$doc"; then
        echo "❌ Missing 'Target Audience' in $doc"
        exit 1
    fi
done

echo "✅ All documentation checks passed!"
```

---

## Templates

### Integration Guide Template

```markdown
# [Feature] Integration Guide

**Target Audience:** [Developers integrating this feature]
**Topic:** [What this guide covers]
**Goal:** [What you'll be able to do after reading]

---

## Overview

[2-3 sentence summary]

**Key Concepts:**
- Concept 1
- Concept 2
- Concept 3

---

## Prerequisites

Before you begin, ensure you have:
- [ ] Requirement 1
- [ ] Requirement 2
- [ ] Requirement 3

---

## Quick Start

Get up and running in 5 minutes:

**1. [First step]**
```bash
command here
```

**2. [Second step]**
```bash
command here
```

**3. [Third step - verify it works]**
```bash
command here
# Expected output:
# ✅ Success message
```

---

## Complete Setup

### Step 1: [Title]

[Detailed explanation]

```bash
# Code example with comments
command --flag=value
```

**Output:**
```
Expected output here
```

### Step 2: [Title]

[Continue pattern...]

---

## Examples

### Example 1: [Common Use Case]

```language
// Full working example
```

### Example 2: [Another Use Case]

```language
// Full working example
```

---

## Troubleshooting

### [Common Issue 1]
**Issue:** [Symptom]
**Root Cause:** [Why]
**Evidence:** [Logs]
**Solution:** [Fix]
**Prevention:** [Avoid]

---

## Best Practices

- ✅ **DO:** [Recommendation 1]
- ✅ **DO:** [Recommendation 2]
- ❌ **DON'T:** [Anti-pattern 1]
- ❌ **DON'T:** [Anti-pattern 2]

---

## Related Documentation

- **[Doc 1](./path.md)** - Description
- **[Doc 2](./path.md)** - Description
```

### API Reference Template

```markdown
# [API] Reference

**Target Audience:** Developers calling this API
**Topic:** Complete API reference for [service/feature]
**Goal:** Understand all available endpoints and how to use them

---

## Overview

[Brief description of what this API does]

**Base URL:** `https://api.example.com`

**Authentication:** Bearer token (JWT)

**Rate Limits:** 1000 requests/second

---

## Endpoints

### [Endpoint Name]

**Purpose:** [What this endpoint does]

**Endpoint:**
```
POST /api/v1/resource
```

**Request:**
```bash
curl -X POST https://api.example.com/v1/resource \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "field": "value"
  }'
```

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| field | string | Yes | Description |

**Response (200 OK - Success):**
```json
{
  "id": "xyz",
  "status": "success"
}
```

**Response (400 Bad Request):**
```json
{
  "error": "invalid_request",
  "message": "field is required"
}
```

**Response (401 Unauthorized):**
```json
{
  "error": "unauthorized",
  "message": "invalid or expired token"
}
```

---

## Error Codes

| Code | Meaning | Resolution |
|------|---------|------------|
| 400 | Bad Request | Check request parameters |
| 401 | Unauthorized | Verify token is valid |
| 429 | Too Many Requests | Slow down, respect rate limits |
| 500 | Internal Server Error | Contact support |

---

## Related Documentation

- **[Integration Guide](./INTEGRATION.md)** - How to integrate
- **[Authentication](../development/AUTH.md)** - Auth details
```

### Troubleshooting Guide Template

```markdown
# [Feature] Troubleshooting

**Target Audience:** Users experiencing issues with [feature]
**Topic:** Common issues and solutions for [feature]
**Goal:** Resolve issues quickly without support escalation

---

## Quick Diagnostics

**Before troubleshooting, check:**
- [ ] [Most common issue 1]
- [ ] [Most common issue 2]
- [ ] [Most common issue 3]

---

## Common Issues

### [Issue 1 - Most Frequent]

**Issue:** [Exact error message or symptom users see]

**Root Cause:** [Technical explanation of why this occurs]

**Evidence:** [What to look for in logs]
```
ERROR: [log snippet]
  timestamp: 2025-11-21T14:30:00Z
  component: [component name]
  details: [relevant details]
```

**Solution:**

1. **[First step]**
```bash
command to run
# Expected output:
# [what you should see]
```

2. **[Second step]**
```bash
command to run
```

3. **[Verification]**
```bash
# Verify the fix worked
command to verify
# Expected output:
# ✅ [success indicator]
```

**Prevention:**
- [How to avoid this issue in future]
- [Configuration changes recommended]
- [Best practices to follow]

---

### [Issue 2]

[Follow same pattern...]

---

## Still Having Issues?

If none of the above solutions work:

1. **Gather diagnostic information:**
```bash
# Run diagnostic script
./scripts/diagnose.sh > diagnostic-report.txt
```

2. **Check logs:**
```bash
# View recent logs
tail -n 100 logs/application.log
```

3. **Report the issue:**
   - [Link to issue tracker]
   - Include diagnostic report
   - Include relevant log snippets
   - Describe steps to reproduce

---

## Related Documentation

- **[Setup Guide](./SETUP.md)** - Initial configuration
- **[FAQ](./FAQ.md)** - Frequently asked questions
```

---

## Maintenance Schedule

### Daily
- No automated maintenance required

### Weekly
- Run `make docs-validate` to check for broken links
- Review open documentation issues/PRs

### Monthly
- Review documentation metrics
- Update docs flagged as stale (6+ months old)
- Check for deprecated features that need doc updates

### Per Release
- Update CHANGELOG.md
- Regenerate auto-docs (`make docs`)
- Test all integration guide examples
- Sync to GitHub wiki
- Create git tag for version

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2025-11-21 | Initial style guide created | Development Team |

---

This style guide is a living document. Propose improvements via pull request or create an issue.
