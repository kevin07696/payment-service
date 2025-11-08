# Terraform Staging Lifecycle

Quick reference for managing staging infrastructure lifecycle.

## Quick Commands

### Check if Staging Exists

```bash
cd terraform
terraform init
terraform plan

# Exit codes:
# 0 = No changes (staging exists and matches config)
# 1 = Error
# 2 = Changes needed (staging doesn't exist or differs from config)
```

### Create Staging

**Via GitHub Actions (Recommended):**
```
Actions → Staging Lifecycle Management → Run workflow
Action: create
```

**Locally:**
```bash
cd terraform
terraform init
terraform apply
```

### Destroy Staging

**Via GitHub Actions (Recommended):**
```
Actions → Staging Lifecycle Management → Run workflow
Action: destroy
```

**Locally:**
```bash
cd terraform
terraform init
terraform destroy
```

### Recreate Staging (Fresh Start)

**Via GitHub Actions (Recommended):**
```
Actions → Staging Lifecycle Management → Run workflow
Action: recreate
```

**Locally:**
```bash
cd terraform
terraform destroy
terraform apply
```

---

## Automatic Lifecycle

### When Staging is Created

✅ **First push to `develop` branch**
- Checks if staging exists
- If not, creates it automatically
- ~10 minutes

✅ **Manual trigger**
- Via GitHub Actions UI
- Select "create" action

### When Staging is Destroyed

✅ **After successful production deployment**
- Merge to `main` triggers production deploy
- Production succeeds → staging destroyed
- ~3-5 minutes

✅ **Manual trigger**
- Via GitHub Actions UI
- Select "destroy" action

---

## State Management

Terraform tracks infrastructure state. State determines:
- What resources exist
- What resources need to be created/updated/destroyed

### View Current State

```bash
cd terraform
terraform show
```

### List Resources in State

```bash
terraform state list
```

### Refresh State

```bash
terraform refresh
```

---

## Cost Analysis

### Resources When Active

| Resource | Cores | Memory | Storage | Always Free |
|----------|-------|--------|---------|-------------|
| Autonomous DB | 1 | - | 20GB | ✅ Yes |
| Compute | 4 ARM | 24GB | 50GB | ✅ Yes |
| VCN | - | - | - | ✅ Yes |

### Resources When Destroyed

| Resource | Cores | Memory | Storage | Cost |
|----------|-------|--------|---------|------|
| None | 0 | 0 | 0 | $0 |

### Active vs Destroyed

```
Development Phase (2 weeks):
├── Week 1: Staging active    = 4 cores, 24GB, 20GB DB
└── Week 2: Staging active    = 4 cores, 24GB, 20GB DB

Production Deploy:
└── Staging destroyed         = 0 cores, 0 GB

Maintenance Phase (2 weeks):
├── Week 3: No staging        = 0 cores (saved)
└── Week 4: No staging        = 0 cores (saved)

Next Feature Development:
└── Staging auto-created      = 4 cores, 24GB, 20GB DB
```

**Resource utilization:**
- Without auto-destroy: 100% (always on)
- With auto-destroy: ~50% (only when developing)

---

## Lifecycle Events

### Event Log Example

```
2025-11-07 10:00 - Push to develop
2025-11-07 10:01 - Staging creation triggered
2025-11-07 10:12 - Staging created (4 cores, 24GB, 20GB DB)
2025-11-07 10:15 - Application deployed

... development work ...

2025-11-14 14:00 - PR merged to main
2025-11-14 14:01 - Production deployment started
2025-11-14 14:08 - Production deployment succeeded
2025-11-14 14:09 - Staging destruction triggered
2025-11-14 14:14 - Staging destroyed (resources freed)

... no active development ...

2025-11-21 09:00 - Push to develop (new feature)
2025-11-21 09:01 - Staging creation triggered (auto)
2025-11-21 09:12 - Staging created again
```

---

## Troubleshooting

### Terraform State Locked

**Error:** "Error acquiring the state lock"

**Fix:**
```bash
# Wait a few minutes (another process might be running)
# Or force unlock (only if sure no other process is running)
terraform force-unlock LOCK_ID
```

### Resources Already Exist

**Error:** "Resource already exists"

**Fix:**
```bash
# Import existing resources
terraform import oci_core_instance.payment_instance ocid1.instance...
terraform import oci_database_autonomous_database.payment_db ocid1.database...
```

### State Drift

**Issue:** Real resources don't match Terraform state

**Fix:**
```bash
# Refresh state to match reality
terraform refresh

# Or destroy and recreate
terraform destroy
terraform apply
```

### Can't Destroy Resources

**Error:** "Resource still in use" or "Dependencies exist"

**Fix:**
```bash
# Force destroy
terraform destroy -auto-approve

# Or manually delete from Oracle Cloud Console first
```

---

## Manual Override

Want to keep staging running even after production deploy?

### Option 1: Disable Auto-Destroy Workflow

Edit `.github/workflows/staging-lifecycle.yml`:

```yaml
# Comment out the auto-destroy job
# auto-destroy-after-production:
#   name: Auto-destroy staging after production deploy
#   ...
```

### Option 2: Only Use Manual Controls

Remove all `workflow_run` triggers, keep only `workflow_dispatch`.

### Option 3: Tag Resources as Persistent

Add to `terraform/compute.tf`:

```hcl
resource "oci_core_instance" "payment_instance" {
  # ... existing config ...

  freeform_tags = {
    "Lifecycle" = "Persistent"
    "AutoDestroy" = "false"
  }
}
```

Then update workflow to check tags before destroying.

---

## Best Practices

### 1. Let Automation Work

✅ Trust auto-create on develop push
✅ Trust auto-destroy after production deploy
❌ Don't manually intervene unless needed

### 2. Use Manual Controls for Special Cases

✅ Manual create: Before major feature work
✅ Manual destroy: Taking a break from project
✅ Manual recreate: Fixing infrastructure issues

### 3. Check Workflow Logs

Monitor:
- Creation time (~10 min is normal)
- Destruction time (~3-5 min is normal)
- Any errors or warnings

### 4. Verify State After Changes

```bash
# After create
terraform show | grep "lifecycle-state"
# Should show: AVAILABLE, RUNNING

# After destroy
terraform show
# Should show: No resources
```

---

## Related

- **Full Documentation**: [docs/STAGING_LIFECYCLE.md](../docs/STAGING_LIFECYCLE.md)
- **Terraform Setup**: [README.md](./README.md)
- **Branching Strategy**: [docs/BRANCHING_STRATEGY.md](../docs/BRANCHING_STRATEGY.md)

---

**Remember:** Staging is ephemeral. Create when needed, destroy when not. 🔄
