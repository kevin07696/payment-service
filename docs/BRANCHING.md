# Branching & Deployment

## Quick Reference

| Branch | Environment | Platform | Deploy | Purpose |
|--------|-------------|----------|--------|---------|
| `main` | Production | Google Cloud Run | After approval | Production-ready code |
| `develop` | Staging | Oracle Cloud | Automatic | Integration testing |
| `feature/*` | Local | - | No | New features |
| `bugfix/*` | Local | - | No | Bug fixes |
| `hotfix/*` | Local | - | No | Emergency production fixes |

## Daily Workflow

### Feature Development

```bash
# Start from develop
git checkout develop && git pull

# Create feature branch
git checkout -b feature/payment-refund

# Work and commit
git add . && git commit -m "feat: Add refund functionality"
git push origin feature/payment-refund

# Create PR: feature/payment-refund → develop
# After merge: automatically deploys to staging
```

### Deploy to Staging

```bash
# Quick fixes: work directly on develop
git checkout develop
git add . && git commit -m "fix: Quick bug fix"
git push origin develop
# Automatically deploys to staging
```

### Deploy to Production

```bash
# Create PR on GitHub: develop → main
# Get team approval
# Merge PR
# Approve deployment in GitHub Actions
# Deploys to production
```

### Hotfix

```bash
# Create from main
git checkout main && git pull
git checkout -b hotfix/critical-security-fix

# Fix and push
git add . && git commit -m "fix: Security patch"
git push origin hotfix/critical-security-fix

# Create PR → main, get approval, merge
# After deploy: merge back to develop
git checkout develop && git merge hotfix/critical-security-fix && git push
```

## CI/CD Pipeline

```text
develop branch:
  Push → Tests → Build → Deploy staging → Integration tests → Keep running

main branch:
  PR → Tests → Build → Wait approval → Deploy production → Smoke tests → Cleanup staging
```

### Deployment Stages

**develop (staging):**
1. Unit tests
2. Build Docker image
3. Deploy to Oracle Cloud staging (automatic)
4. Run integration tests (deployment gate)
5. Keep staging running

**main (production):**
1. Unit tests (requires integration tests passed on develop)
2. Build Docker image
3. Wait for manual approval
4. Deploy to Google Cloud Run
5. Run smoke tests
6. Cleanup staging

## Branch Protection

### main Branch

**Required before merge:**
- ✅ Unit tests passed
- ✅ Build successful
- ✅ Integration tests passed (from develop)
- ✅ 1 code review approval
- ✅ Branch up-to-date with main

**Enforced:**
- ❌ No force pushes
- ❌ No deletions
- ⚠️ Admins can bypass (emergency hotfixes only)

### develop Branch

**Required before merge:**
- ✅ Unit tests passed
- ❌ No force pushes
- ❌ No deletions

## Environment Configuration

### Staging (Oracle Cloud)

**EPX:** Sandbox (`https://secure.epxuap.com`)
**Database:** Oracle Autonomous Database
**Logging:** Debug level
**URL:** `http://{ORACLE_CLOUD_HOST}:8081`

**Secrets:** OCI credentials, OCIR, database password, EPX sandbox credentials

### Production (Google Cloud Run)

**EPX:** Production (`https://secure.epxnow.com`)
**Database:** Cloud SQL PostgreSQL
**Logging:** Info level
**URL:** `https://payment-service-xxx.a.run.app`

**Secrets:** GCP service account, Cloud SQL, EPX production credentials

## Rollback

### Staging

```bash
git checkout develop
git revert HEAD  # or specific commit hash
git push origin develop
# Staging automatically redeploys
```

### Production

```bash
# Option 1: Git revert
git checkout main
git revert HEAD
git push origin main
# Production redeploys after approval

# Option 2: Cloud Run console
# Find previous revision → Route traffic to previous revision
```

## Branch Protection Setup

### Automated (via GitHub Actions)

```yaml
# Run once in GitHub UI:
Actions → Repository Setup → Run workflow
  ☑ Configure branch protection rules
  Required reviewers: 1
→ Run workflow
```

### View Current Protection

```bash
# GitHub UI
Settings → Branches → main → Branch protection rules

# GitHub CLI
gh api repos/kevin07696/payment-service/branches/main/protection | jq
```

### Modify Protection

```bash
# Update required checks
cat > /tmp/branch-protection.json <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Run Tests / Test",
      "Build Docker Image / build",
      "Integration Tests (Post-Deployment Gate)"
    ]
  },
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true
  },
  "enforce_admins": false,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF

gh api repos/kevin07696/payment-service/branches/main/protection \
  --method PUT --input /tmp/branch-protection.json
```

## Monitoring

### Check Deployment Status

```bash
# Staging health
curl http://${ORACLE_CLOUD_HOST}:8081/cron/health

# Production health
curl https://payment-service-xxx.a.run.app/cron/health

# View logs
# Staging: SSH to Oracle Cloud, docker-compose logs -f
# Production: gcloud run services logs read payment-service --region us-central1
```

### GitHub Actions

```
https://github.com/kevin07696/payment-service/actions
```

## Troubleshooting

### "Required status checks are failing"

```bash
# Check workflow runs
gh run list --branch develop --limit 1

# Re-run failed workflows
gh run rerun RUN_ID
```

### "Branch not up-to-date with main"

```bash
git checkout develop
git pull origin main
git push origin develop
# PR auto-updates and re-runs checks
```

### Hotfix Bypassing Protection

```bash
# Option 1: Admin override (if enforce_admins: false)
git push origin main  # As admin

# Option 2: Temporarily disable protection
gh api repos/kevin07696/payment-service/branches/main/protection --method DELETE
git push origin main
gh api repos/kevin07696/payment-service/branches/main/protection \
  --method PUT --input /tmp/branch-protection.json
```

⚠️ **Emergency use only** - bypasses automated tests

## Best Practices

**Commits:**
- Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`
- Keep commits focused
- Reference issues: `fix: Resolve timeout (#123)`

**Pull Requests:**
- Fill out PR template
- Link related issues
- Wait for CI before merge
- Delete feature branches after merge

**Testing:**
- Write tests for new features
- Test locally before pushing
- Verify in staging before production
- Run integration tests: `go test -tags=integration ./...`

**Deployment:**
- Always deploy to staging first
- Test thoroughly in staging
- Deploy to production during low-traffic hours
- Monitor logs after deployment

## References

- CI/CD workflow: `.github/workflows/ci-cd.yml`
- Staging setup: `docs/ORACLE_FREE_TIER_STAGING.md`
- Production setup: `docs/GCP_PRODUCTION_SETUP.md`
- Secrets configuration: `docs/SECRETS.md`
- Testing: `docs/TESTING.md`
