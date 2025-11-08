# Staging Environment Lifecycle Management

**Smart resource optimization** - Auto-destroy staging after production deploy to minimize Oracle Cloud usage.

## Philosophy

Staging environments are only needed for testing before production deployment. Once code is merged to `main` and deployed to production, staging can be destroyed to:

✅ **Minimize resource usage** (even though it's free)
✅ **Prevent hitting Always Free limits** if running multiple projects
✅ **Keep infrastructure clean** - only what you need, when you need it
✅ **Cost optimization** - good practice even with free tier

---

## Automated Lifecycle

### 🟢 Auto-Create Staging

**Triggers automatically when:**
- Push to `develop` branch
- Staging infrastructure doesn't exist

**What happens:**
1. Terraform checks if staging exists
2. If not, creates all infrastructure (database, compute, networking)
3. Deploys application
4. Posts summary to workflow

**Time:** ~10-12 minutes (infrastructure + deployment)

### 🔴 Auto-Destroy Staging

**Triggers automatically when:**
- Production deployment succeeds (merge to `main`)
- Staging infrastructure exists

**What happens:**
1. Waits for production deployment to complete
2. Terraform destroys all staging resources
3. Posts comment on merged PR
4. Adds summary to workflow

**Time:** ~3-5 minutes

**Resources destroyed:**
- Compute instance (saves 4 ARM cores, 24GB RAM)
- Autonomous Database (saves 20GB storage)
- VCN networking components

---

## Manual Controls

### Via GitHub Actions UI

Go to: **Actions** → **Staging Lifecycle Management** → **Run workflow**

**Available actions:**

#### 1. Create
Creates staging infrastructure from scratch.

**Use when:**
- You want to test on staging before merging to develop
- You manually destroyed staging and want it back
- Starting fresh after long break

**Time:** ~10 minutes

#### 2. Destroy
Destroys all staging infrastructure.

**Use when:**
- You're done testing and don't need staging anymore
- Temporarily not working on the project
- Want to minimize resource usage

**Time:** ~3-5 minutes

#### 3. Recreate
Destroys and recreates staging (fresh start).

**Use when:**
- Staging is in a bad state
- Want to reset to clean slate
- Testing infrastructure changes

**Time:** ~13-17 minutes (destroy + create)

### Optional Reason Field

When running manually, you can add a reason:
```
Reason: "Testing new database schema"
Reason: "Done with feature development"
Reason: "Project on hold for 2 weeks"
```

This gets logged in workflow summaries for tracking.

---

## Typical Workflow

### Development Cycle

```bash
# 1. Start working on a feature
git checkout develop
git pull

# Push triggers auto-create if staging doesn't exist
git push origin develop

# Staging auto-created (~10 min)
# Deploy happens automatically
```

```bash
# 2. Test on staging
curl http://STAGING_IP:8081/cron/health

# Make changes, push to develop
# Auto-deploys to staging each time
```

```bash
# 3. Ready for production
git checkout main
git merge develop
git push origin main

# Production deploys
# Staging auto-destroyed after success (~3 min)
```

```bash
# 4. Back to development
git checkout develop
# ... make changes ...
git push origin develop

# Staging auto-created again if needed
```

---

## State Diagram

```
┌─────────────────────────────────────────────────────────┐
│                  STAGING LIFECYCLE                      │
└─────────────────────────────────────────────────────────┘

    [Start]
       │
       ▼
    [No Staging]
       │
       │  Push to develop
       ▼
    [Create Staging] ──────────────┐
       │                           │
       ▼                           │
    [Staging Active]               │
       │                           │
       │  Deploy to develop        │
       ▼                           │
    [Test & Iterate] ──────────────┘
       │
       │  Merge to main
       ▼
    [Deploy Production]
       │
       │  Production success
       ▼
    [Destroy Staging]
       │
       ▼
    [No Staging] ◄─────────────────┘
```

---

## Cost Savings

Even though Oracle Always Free tier is $0, this approach provides:

### Resource Allocation Benefits

| Scenario | With Auto-Destroy | Without Auto-Destroy |
|----------|-------------------|---------------------|
| **Active Development** | Staging exists ~40% of time | Staging exists 100% of time |
| **Production Stable** | Staging destroyed | Staging idle (wasted) |
| **Multiple Projects** | Use same free tier across projects | Limits consumed per project |
| **Always Free Limits** | Available for other uses | Locked to one project |

### Oracle Always Free Limits

Oracle provides (per account):
- **2** Autonomous Databases (20GB each)
- **4** Ampere A1 cores + **24GB** RAM total

**With auto-destroy:**
- ✅ Can run 2 projects (staging for one, production for another)
- ✅ Limits available when needed
- ✅ Clean resource management

**Without auto-destroy:**
- ⚠️ One staging consumes 1 database + half compute
- ⚠️ Limits locked even when not in use
- ⚠️ Can't use for other projects/experiments

---

## Workflow Configuration

The lifecycle is managed by `.github/workflows/staging-lifecycle.yml`:

### Triggers

```yaml
# Auto-destroy after production deploy
workflow_run:
  workflows: ["CI/CD Pipeline"]
  types: [completed]
  branches: [main]

# Auto-create for develop branch
workflow_run:
  workflows: ["CI/CD Pipeline"]
  types: [completed]
  branches: [develop]

# Manual controls
workflow_dispatch:
  inputs:
    action: [create, destroy, recreate]
    reason: (optional)
```

### Customization

You can disable auto-destroy by commenting out the trigger:

```yaml
# .github/workflows/staging-lifecycle.yml

# Comment this section to disable auto-destroy
# auto-destroy-after-production:
#   name: Auto-destroy staging after production deploy
#   ...
```

Or disable auto-create:

```yaml
# Comment this section to disable auto-create
# auto-create-for-develop:
#   name: Auto-create staging for develop branch
#   ...
```

---

## Notifications

### Workflow Summaries

Every lifecycle action creates a workflow summary showing:
- Action performed (create/destroy/recreate)
- Reason (if manual)
- Timestamp
- Resources affected
- Next steps

### PR Comments

After auto-destroy on production deploy, a comment is posted to the merged PR:

```
🗑️ Staging Environment Destroyed

Staging infrastructure has been automatically destroyed after
successful production deployment.

Reason: Production deployment successful on `main` branch
Destroyed at: 2025-11-07T12:34:56Z

To recreate staging:
- Go to Actions → Staging Lifecycle Management
- Click "Run workflow"
- Select action: `create`

Or push to `develop` branch and it will auto-create.
```

---

## Troubleshooting

### Staging Not Auto-Creating

**Check:**
1. Push was to `develop` branch
2. CI/CD workflow completed successfully
3. GitHub Actions has proper permissions

**Manual fix:**
```bash
# Go to GitHub Actions
Actions → Staging Lifecycle Management → Run workflow
Select: create
```

### Staging Not Auto-Destroying

**Check:**
1. Production deployment succeeded
2. Workflow completed without errors
3. Terraform state is accessible

**Manual fix:**
```bash
# Go to GitHub Actions
Actions → Staging Lifecycle Management → Run workflow
Select: destroy
```

### Staging in Weird State

**Solution:** Recreate from scratch

```bash
# Go to GitHub Actions
Actions → Staging Lifecycle Management → Run workflow
Select: recreate
Reason: "Reset to clean state"
```

### Want to Keep Staging After Production Deploy

Disable auto-destroy in `.github/workflows/staging-lifecycle.yml`

Or use manual destroy only:
```yaml
# Keep only workflow_dispatch, remove workflow_run
```

---

## Best Practices

### 1. Trust the Automation

✅ Let staging auto-create when you push to develop
✅ Let staging auto-destroy after production deploy
✅ Don't manually manage unless needed

### 2. Use Manual Controls When Needed

✅ Manual `create` before starting work (optional)
✅ Manual `destroy` when taking a break
✅ Manual `recreate` when debugging issues

### 3. Add Reasons for Manual Actions

```
✅ "Testing database migrations"
✅ "Project on hold for 2 weeks"
✅ "Resetting to clean state"
❌ "test"
❌ (blank)
```

### 4. Monitor Workflow Logs

Check Actions tab regularly to see:
- When staging was created/destroyed
- Any errors in lifecycle
- Resource usage patterns

---

## Advanced: Scheduled Teardowns

Want to auto-destroy staging after X days of inactivity?

Add to `.github/workflows/staging-lifecycle.yml`:

```yaml
on:
  schedule:
    # Destroy staging every Sunday at midnight
    - cron: '0 0 * * 0'

jobs:
  scheduled-cleanup:
    name: Scheduled staging cleanup
    runs-on: ubuntu-latest
    environment: staging

    steps:
      # Check last commit to develop
      # If > 7 days old, destroy staging
      # ...
```

---

## Summary

**Automatic lifecycle management:**
- ✅ Creates staging when you need it (push to develop)
- ✅ Destroys staging when you don't (production deploy)
- ✅ Manual controls available anytime
- ✅ Minimizes resource usage
- ✅ Good practice even with free tier

**Total cost:** Still **$0.00/month** - just more efficient! 🚀

---

## Related Documentation

- **Terraform Setup**: [terraform/README.md](../terraform/README.md)
- **Branching Strategy**: [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)
- **Quick Start**: [SETUP_STAGING.md](../SETUP_STAGING.md)
