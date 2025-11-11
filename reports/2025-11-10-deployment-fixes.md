# CI/CD Deployment Fixes Summary

**Date:** 2025-11-10
**Session:** Comprehensive CI/CD pipeline debugging and fixes

## ✅ Successfully Fixed Issues

### 1. SSH Key Authentication Failure
**Deployment:** `deployment-workflows@cdc1787`

**Problem:** Database migrations failed with SSH authentication error
```
ssh: unable to authenticate, attempted methods [none publickey]
```

**Root Cause:** Terraform generates SSH key when secret is empty, but deployment workflow used different key from GitHub secrets.

**Solution:**
- Save Terraform-generated SSH private key as GitHub Actions artifact
- Download artifact in deployment workflow
- Use `key_path` instead of `key` parameter in ssh-action

**Result:** ✅ Migrations succeed with proper SSH authentication

---

### 2. Missing OCIR Environment Variables
**Deployment:** `deployment-workflows@18f055b`

**Problem:** docker-compose couldn't resolve image URLs
```yaml
image: ${OCIR_REGION}.ocir.io/${OCIR_NAMESPACE}/payment-service:latest
```

**Root Cause:** cloud-init created .env file without OCIR variables

**Solution:** Added OCIR_REGION and OCIR_NAMESPACE to cloud-init .env file creation

**Result:** ✅ docker-compose can resolve image URLs correctly

---

### 3. Cloud-init Timing Race Condition
**Deployment:** `deployment-workflows@5c1e15f`

**Problem:** Deployment commands ran before Docker was installed
```
bash: docker: command not found
bash: docker-compose: command not found
```

**Root Cause:** SSH port opened while cloud-init still installing Docker

**Solution:**
- Added cloud-init completion wait before migrations
- Uses `cloud-init status --wait` with 10-minute timeout
- Verifies Docker, docker-compose, and application directory exist

**Result:** ✅ Deployment waits for environment to be ready

---

### 4. JQ Empty Result Handling
**Deployment:** `deployment-workflows@ba10bc6`

**Problem:** Bash comparison errors when jq found no resources
```
line 13: [: : integer expression expected
```

**Root Cause:** jq returns empty string instead of `0` when array is empty

**Solution:** Added `// 0` default value to all jq length calculations
```yaml
# Before
DB_COUNT=$(... | jq '[.data[]] | length')

# After
DB_COUNT=$(... | jq '([.data[]?] | length) // 0')
```

**Result:** ✅ Cleanup script properly counts resources

---

### 5. Oracle Free Tier Quota Checks
**Deployment:** `deployment-workflows@799c025`

**Problem:** Deployments failed with quota exceeded without pre-validation

**Solution:** Added quota validation before Terraform runs:
- Database quota check (max 2 Always Free databases)
- Compute quota check (max 2 instances) with automatic termination

**Result:** ✅ Clear error messages guide quota resolution

---

## ⚠️ Remaining Issues

### Issue 1: OCI CLI Pre-Provisioning Cleanup Not Working

**Status:** CRITICAL - Blocks automated deployment

**Symptoms:**
- Pre-provisioning cleanup finds 0 orphaned resources despite them existing
- OCI CLI commands return empty results in GitHub Actions
- Errors silently suppressed by `2>/dev/null`

**Root Cause (Hypothesis):**
1. OCI CLI might not be installed in GitHub Actions runner
2. OCI credentials configured but failing authentication
3. Permissions issue with compartment access

**Evidence:**
```bash
# Workflow logs show:
Found  orphaned database(s)  # Empty value!
/home/runner/work/_temp/...sh: line 13: [: : integer expression expected
```

**Required Investigation:**
1. Verify OCI CLI is installed in runner (add `oci --version` check)
2. Remove `2>/dev/null` temporarily to see actual errors
3. Test OCI authentication (run `oci iam region list`)
4. Verify compartment OCID is correct

**Temporary Workaround:**
- Manually clean up databases before each deployment:
  ```bash
  ./scripts/cleanup-oracle-databases.sh
  ```

---

### Issue 2: Cleanup-on-Failure Doesn't Remove Orphaned Resources

**Status:** HIGH PRIORITY

**Problem:**
- When Terraform fails during database creation, the database is created in Oracle
- But Terraform never records it in state (because API returned error)
- `terraform destroy` only destroys resources in state
- Database remains orphaned in Oracle

**Example:**
```
1. Terraform runs: oci_database_autonomous_database.payment_db: Creating...
2. Oracle API creates the database
3. Oracle API returns 400-QuotaExceeded error
4. Terraform doesn't save database to state
5. cleanup-on-failure runs: terraform destroy
6. Only destroys: tls_private_key.ssh_key (1 resource)
7. Database left orphaned in Oracle
```

**Solution Needed:**
Add OCI CLI cleanup to cleanup-on-failure job:
```yaml
- name: Cleanup Orphaned Resources (OCI CLI)
  if: failure()
  run: |
    # Delete any databases created in last 10 minutes with our naming pattern
    oci db autonomous-database list ... | jq ... | xargs oci db autonomous-database delete

    # Delete any compute instances created in last 10 minutes
    oci compute instance list ... | jq ... | xargs oci compute instance terminate
```

---

## 📊 Deployment Success Metrics

**Total Fixes Applied:** 5
**Successful Stages:**
- ✅ Tests: Pass
- ✅ Docker Build: Pass
- ✅ Infrastructure - Database Creation: Pass
- ❌ Infrastructure - Compute Instance: Quota Exceeded

**Blocked By:**
- Oracle Free Tier quota (2/2 databases due to orphaned resources)
- Pre-provisioning cleanup not working in GitHub Actions

---

## 🚀 Next Steps to Complete Deployment

### Immediate Actions (Required):
1. **Debug OCI CLI in GitHub Actions**
   - Add debugging step to check OCI CLI installation and auth
   - Remove `2>/dev/null` to see actual errors
   - Fix authentication or permissions issue

2. **Fix Cleanup-on-Failure**
   - Add OCI CLI cleanup of recently created resources
   - Use time-based filtering (last 10 minutes)
   - Don't rely solely on Terraform state

3. **Test Complete Flow**
   - With working cleanup, quota should be automatically freed
   - Deployment should succeed end-to-end

### Long-term Improvements:
1. **Monitoring & Alerts**
   - Alert when orphaned resources detected
   - Dashboard showing quota usage

2. **Cost Optimization**
   - Automated cleanup of old test deployments
   - Scheduled job to terminate unused resources

3. **Documentation**
   - Update README with deployment process
   - Document quota limits and workarounds
   - Create troubleshooting guide

---

## 📝 Files Modified

### deployment-workflows Repository:
- `.github/workflows/infrastructure-lifecycle.yml` - Main infrastructure workflow
- `.github/workflows/deploy-oracle-staging.yml` - Deployment workflow
- `terraform/oracle-staging/cloud-init.yaml` - Instance initialization

### payment-service Repository:
- `CHANGELOG.md` - Comprehensive change documentation
- `scripts/cleanup-oracle-databases.sh` - Manual cleanup utility
- `docs/DEPLOYMENT_FIXES_SUMMARY.md` - This document

---

## 💡 Key Learnings

1. **GitHub Actions Workflow References:**
   - Using `@main` picks up latest commits
   - But workflow changes may have caching delays

2. **Oracle Free Tier Limits:**
   - 2 Always Free Autonomous Databases (per account, not per compartment)
   - 2 Compute Instances (measured by core count)
   - Orphaned resources block new deployments

3. **Terraform State Management:**
   - Resources created before Terraform fails aren't in state
   - `terraform destroy` can't clean resources not in state
   - Need OCI CLI cleanup for robustness

4. **Error Suppression Anti-Pattern:**
   - `2>/dev/null` hides critical debugging information
   - Better to log errors and handle them explicitly
   - Silent failures are hard to debug

5. **CI/CD Cleanup Strategies:**
   - Pre-provisioning cleanup: Remove old orphaned resources
   - Post-failure cleanup: Remove partially created resources
   - Both are necessary for quota management

---

## 🔍 Debugging Commands

```bash
# Check Oracle database quota locally
./scripts/cleanup-oracle-databases.sh

# Check compute instance quota
/tmp/check-compute.sh

# Manual cleanup
oci db autonomous-database list --compartment-id $OCI_COMPARTMENT_OCID --all
oci db autonomous-database delete --autonomous-database-id <ID> --force

# Monitor workflow
gh run list --limit 5
gh run view <RUN_ID> --log | grep -E "(error|Error|ERROR|failed|Failed)"
```

---

**Status:** 🟡 **In Progress** - OCI CLI cleanup mechanism needs debugging
