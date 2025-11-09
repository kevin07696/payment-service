# Secrets Architecture - Infrastructure as Code

This document describes the clean separation between service-specific secrets and infrastructure secrets.

## Separation of Concerns

### Service Repository (payment-service)
**Responsibility**: Service-specific configuration and secrets

**Secrets Stored Here**:
- `ORACLE_DB_PASSWORD` - Database password for this service
- `EPX_MAC_STAGING` - EPX MAC secret for this service's test merchant

**What It Does**:
- Passes service secrets to deployment-workflows as inputs
- Does NOT contain Oracle Cloud infrastructure credentials
- Does NOT contain container registry credentials

### Deployment Workflows Repository (deployment-workflows)
**Responsibility**: Infrastructure provisioning and deployment

**Secrets Stored Here** (Organization Level):
- `OCI_USER_OCID` - Oracle Cloud user
- `OCI_TENANCY_OCID` - Oracle Cloud tenancy
- `OCI_FINGERPRINT` - API key fingerprint
- `OCI_PRIVATE_KEY` - API private key
- `OCI_REGION` - Oracle Cloud region
- `OCI_COMPARTMENT_OCID` - Target compartment
- `OCIR_REGION` - Container registry region
- `OCIR_TENANCY_NAMESPACE` - Container registry namespace
- `OCIR_USERNAME` - Container registry username
- `OCIR_AUTH_TOKEN` - Container registry auth token
- `SSH_PUBLIC_KEY` - SSH public key for VMs
- `ORACLE_CLOUD_SSH_KEY` - SSH private key for VMs

**What It Does**:
- Uses OCI credentials to provision infrastructure via Terraform
- Creates OCI Vault for secret management
- Stores service secrets (EPX_MAC) in OCI Vault
- Seeds database with vault secret OCIDs
- Deploys containers using OCIR credentials

## Workflow Flow

```yaml
# payment-service/.github/workflows/ci-cd.yml
jobs:
  ensure-staging-infrastructure:
    uses: kevin07696/deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit  # Passes service secrets
    with:
      action: create
      environment: staging
      # Service secrets passed as inputs:
      db_password: ${{ secrets.ORACLE_DB_PASSWORD }}
      epx_mac: ${{ secrets.EPX_MAC_STAGING }}
```

```yaml
# deployment-workflows/.github/workflows/infrastructure-lifecycle.yml
# Uses organization secrets (OCI_*, OCIR_*, SSH_*)
# Receives service secrets as inputs
# Runs Terraform with both
```

## Terraform Responsibility

**terraform/oracle-staging/** provisions:

1. **OCI Vault** (Secret Manager)
   ```hcl
   resource "oci_kms_vault" "payment_vault" {
     # Created using OCI credentials from org secrets
   }
   ```

2. **Vault Secret** (EPX MAC)
   ```hcl
   resource "oci_vault_secret" "epx_mac" {
     vault_id = oci_kms_vault.payment_vault.id
     secret_content = var.epx_mac  # From service secret input
   }
   ```

3. **Database Seed**
   ```sql
   INSERT INTO agent_credentials (
     mac_secret_path  -- OCID from vault secret
   ) VALUES (
     '${vault_secret_ocid}'  # Terraform output
   );
   ```

## Secret Storage Locations

| Secret Type | Where Stored | Who Uses It |
|------------|--------------|-------------|
| Oracle Cloud Auth (OCI_*) | deployment-workflows org secrets | Terraform to provision infra |
| Container Registry (OCIR_*) | deployment-workflows org secrets | Docker push/pull |
| SSH Keys | deployment-workflows org secrets | VM access |
| DB Password | payment-service repo secret | Passed to Terraform → app config |
| EPX MAC | payment-service repo secret | Passed to Terraform → stored in vault |
| Vault OCID | Database (agent_credentials) | Service reads at runtime |
| Actual MAC Secret | OCI Vault | Service reads at runtime |

## Runtime Secret Access

**At Runtime, the Service**:
1. Queries `agent_credentials` table for merchant
2. Gets `mac_secret_path` (vault OCID)
3. Uses OCI SDK to read secret from vault
4. Uses MAC to sign EPX requests

**IAM Permissions Required**:
- Service instance needs `read` permission on vault secrets
- Granted via OCI dynamic group + policy

## Benefits

✅ **Clean Separation**: Infrastructure vs service concerns
✅ **Reusable Infrastructure**: deployment-workflows used by all services
✅ **Service Isolation**: Each service has its own secrets
✅ **Proper Secret Management**: Secrets in vault, not plaintext
✅ **IaC**: Everything defined in Terraform
✅ **No Secret Duplication**: Infrastructure secrets at org level

## Adding a New Service

1. Create new service repo (e.g., `notification-service`)
2. Add service-specific secrets to repo:
   - `ORACLE_DB_PASSWORD`
   - `EPX_MAC_STAGING` (if uses EPX)
3. Call deployment-workflows (same as payment-service)
4. Terraform creates separate vault secrets for new service
5. **No infrastructure secret duplication!**

## Security Model

- **Infrastructure Secrets**: Org-level, managed by platform team
- **Service Secrets**: Repo-level, managed by service team
- **Runtime Secrets**: Vault-stored, accessed via IAM
- **Credentials**: Never in plaintext in database
- **Audit Trail**: Vault logs all secret access
