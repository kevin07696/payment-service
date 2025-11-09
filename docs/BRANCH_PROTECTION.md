# GitHub Branch Protection Rules

## Overview

Branch protection rules ensure code quality and deployment safety by requiring specific checks before merging to production.

**Last Updated:** 2025-11-09
**Repository:** kevin07696/payment-service

---

## main Branch Protection

### Current Configuration

✅ **Configured via GitHub API** (see `.github/workflows/ci-cd.yml` setup)

```json
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
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false
  },
  "enforce_admins": false,
  "allow_force_pushes": false,
  "allow_deletions": false
}
```

### What This Means

#### 1. Required Status Checks ✅
**Before merging to main, these checks MUST pass:**

- **Run Tests / Test**: Unit tests from develop branch
- **Build Docker Image / build**: Docker image builds successfully
- **Integration Tests (Post-Deployment Gate)**: Integration tests pass on deployed staging environment

**strict: true** means the branch must be up-to-date with main before merging (no stale branches).

#### 2. Required Pull Request Reviews ✅
- **1 approval required** from a team member
- **Stale reviews dismissed** when new commits are pushed
- **Code owner review NOT required** (optional for small teams)

#### 3. Admin Enforcement
- **Admins can bypass**: `enforce_admins: false` allows emergency hotfixes
- **No force pushes**: Prevents rewriting history
- **No branch deletion**: Protects main from accidental deletion

---

## Why These Rules Matter

### Defense in Depth
Even though develop runs all tests, main protection provides:
- **Hotfix safety**: Direct commits to main still validated
- **Merge conflict protection**: Manual resolution can introduce bugs
- **Time gap protection**: Code might sit in develop for days/weeks
- **Human review**: Code review catches logic errors tests miss

### Integration Test Gate
**Critical:** main branch cannot be updated unless integration tests passed on develop.

This ensures:
- ✅ Deployed staging service works correctly
- ✅ EPX gateway integration verified
- ✅ Database migrations successful
- ✅ Real-world API validation

### Workflow Enforcement
```
develop → staging deployment → integration tests pass
                                       ↓
                                    ✅ ALLOWED
                                       ↓
                      PR to main → review → merge → production
```

If integration tests fail on develop:
```
develop → staging deployment → integration tests fail
                                       ↓
                                    ❌ BLOCKED
                                       ↓
                         Cannot create PR to main
```

---

## Viewing Current Protection

### GitHub UI
```
Settings → Branches → main → Branch protection rules
```

### GitHub CLI
```bash
gh api repos/kevin07696/payment-service/branches/main/protection | python3 -m json.tool
```

### Quick Check
```bash
# View required status checks
gh api repos/kevin07696/payment-service/branches/main/protection/required_status_checks \
  --jq '.contexts'
```

---

## Modifying Protection Rules

### Prerequisites
- Repository admin access
- GitHub CLI installed (`gh`)

### Update Required Status Checks

```bash
# Edit the JSON file
cat > /tmp/branch-protection.json <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Run Tests / Test",
      "Build Docker Image / build",
      "Integration Tests (Post-Deployment Gate)",
      "NEW CHECK NAME HERE"
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

# Apply the changes
gh api repos/kevin07696/payment-service/branches/main/protection \
  --method PUT \
  --input /tmp/branch-protection.json
```

### Common Modifications

#### Increase Review Count
```json
"required_approving_review_count": 2  // Require 2 approvals
```

#### Require Code Owner Review
```json
"require_code_owner_reviews": true
```

#### Enforce for Admins
```json
"enforce_admins": true  // Admins must also follow rules
```

---

## Emergency Procedures

### Hotfix to Production (Bypass Protection)

**Option 1: Admin Override** (if `enforce_admins: false`)
```bash
# Admin can push directly to main
git checkout main
git pull
# Make hotfix changes
git commit -m "hotfix: Critical security patch"
git push origin main  # Bypasses protection
```

**Option 2: Temporarily Disable Protection**
```bash
# Remove protection
gh api repos/kevin07696/payment-service/branches/main/protection --method DELETE

# Push hotfix
git push origin main

# Re-enable protection
gh api repos/kevin07696/payment-service/branches/main/protection \
  --method PUT \
  --input /tmp/branch-protection.json
```

**⚠️ WARNING:** Both approaches skip automated tests. Only use for critical emergencies.

---

## Best Practices

### ✅ DO
- Keep `strict: true` to prevent stale branches
- Require at least 1 code review
- Include integration tests in required checks
- Document any exceptions or bypasses

### ❌ DON'T
- Force push to main (disabled by protection)
- Bypass protection for non-emergencies
- Remove required status checks without team discussion
- Set `enforce_admins: true` without team consensus

---

## Troubleshooting

### "Required status checks are failing"
**Problem:** Cannot merge PR because checks haven't passed.

**Solution:**
1. Check GitHub Actions on develop branch
2. Ensure all workflows completed successfully
3. Verify branch is up-to-date with main
4. Re-run failed workflows if transient failures

### "Required status check 'X' was not found"
**Problem:** Protection requires a check that doesn't exist in workflow.

**Solution:**
```bash
# List actual check names from recent runs
gh run list --branch develop --limit 1 --json name,conclusion

# Update protection to match actual names
# (See "Modifying Protection Rules" above)
```

### "This branch has not been updated with main"
**Problem:** `strict: true` requires branch to be current.

**Solution:**
```bash
git checkout develop
git pull origin main
git push origin develop
# PR will auto-update and re-run checks
```

---

## Related Documentation

- [CI/CD Pipeline](../README.md#cicd-pipeline)
- [Testing Strategy](./TESTING_STRATEGY.md)
- [GitHub Secrets Setup](./GITHUB_SECRETS_SETUP.md)
- [Contributing Guidelines](../CONTRIBUTING.md) *(if exists)*

---

## Configuration History

| Date | Change | Reason |
|------|--------|--------|
| 2025-11-09 | Initial setup | Enable CD with deployment gates |
| 2025-11-09 | Added integration tests check | Ensure staging tests pass before production |
| 2025-11-09 | Set review count to 1 | Code quality and knowledge sharing |

---

**Questions or Issues?**
Contact: Development team
Repository: https://github.com/kevin07696/payment-service
