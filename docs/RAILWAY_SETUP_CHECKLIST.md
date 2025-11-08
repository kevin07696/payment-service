# Railway Staging Setup Checklist

Use this checklist to ensure all steps are completed for Railway deployment.

## ☑️ Pre-Deployment Checklist

### Railway Setup
- [ ] Create Railway account at https://railway.app
- [ ] Create new Railway project
- [ ] Link GitHub repository: `kevin07696/payment-service`
- [ ] Add PostgreSQL database to project
- [ ] Enable "Wait for CI" in Railway settings
- [ ] Enable "Teardown idle builds"
- [ ] Enable "Serverless" mode (optional, recommended for cost savings)
- [ ] Set health check path to `/cron/health`
- [ ] Set health check timeout to `100` seconds

### Railway Credentials
- [ ] Generate Railway token from Account Settings → Tokens
- [ ] Copy Railway Project ID from project settings
- [ ] Save both securely for GitHub secrets

### GitHub Secrets Configuration
Go to GitHub repo → Settings → Secrets and variables → Actions

Add these secrets:
- [ ] `RAILWAY_TOKEN` - Token from Railway
- [ ] `RAILWAY_PROJECT_ID` - Project ID from Railway
- [ ] `EPX_MAC_STAGING` - Value: `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y`
- [ ] `CRON_SECRET_STAGING` - Generate with: `openssl rand -base64 32`
- [ ] `CALLBACK_BASE_URL_STAGING` - Temp value: `http://localhost:8081` (update after first deploy)

## ☑️ First Deployment

### Initial Deploy
- [ ] Push code to main branch OR manually deploy from Railway dashboard
- [ ] Wait for deployment to complete
- [ ] Note your Railway app URL (e.g., `https://payment-service-staging-xxxxx.up.railway.app`)
- [ ] Test health endpoint: `curl https://your-app.up.railway.app/cron/health`

### Update Callback URL
- [ ] Update GitHub secret `CALLBACK_BASE_URL_STAGING` with actual Railway URL
- [ ] Make trivial change and push to trigger redeployment
- [ ] Verify deployment completes successfully

## ☑️ Verification

### GitHub Actions
- [ ] All workflow jobs pass:
  - [ ] ✅ Run Tests
  - [ ] ✅ Build Docker Image
  - [ ] ✅ Run Database Migrations (Staging)
  - [ ] ✅ Deploy to Railway Staging

### Railway Dashboard
- [ ] Service shows "Active" status
- [ ] Latest deployment is successful
- [ ] Environment variables are set correctly (check Variables tab)
- [ ] Database is connected and accessible

### Functional Testing
- [ ] Health endpoint responds: `curl https://your-app.up.railway.app/cron/health`
- [ ] Expected response:
  ```json
  {
    "status": "healthy",
    "time": "2025-11-07T19:30:00Z"
  }
  ```

### Payment Flow Testing (Manual)
- [ ] Test Browser Post form generation
- [ ] Test EPX Server Post transactions
- [ ] Verify callbacks are received
- [ ] Check logs for any errors

## ☑️ Post-Deployment

### Monitoring
- [ ] Check Railway logs for errors
- [ ] Monitor usage in Railway dashboard
- [ ] Set up billing alerts (if needed)
- [ ] Review cost breakdown

### Documentation
- [ ] Update team wiki with Railway URL
- [ ] Share staging credentials with India team
- [ ] Document any custom configuration

### Security
- [ ] Verify CRON_SECRET is set and working
- [ ] Test rate limiting (10 req/sec per IP)
- [ ] Confirm DB_SSL_MODE is set to "require"
- [ ] Review Railway security settings

## ☑️ Troubleshooting Reference

If something goes wrong, check:

1. **GitHub Actions failing:**
   - Check workflow logs in GitHub Actions tab
   - Verify all secrets are set correctly
   - Confirm Railway token hasn't expired

2. **Migrations failing:**
   - Check DATABASE_URL is set in Railway
   - Verify PostgreSQL service is running
   - Check migration files are valid

3. **Deployment failing:**
   - Check Railway service logs
   - Verify environment variables are set
   - Confirm health check endpoint is working

4. **Health check failing:**
   - Test endpoint manually: `curl https://your-app.up.railway.app/cron/health`
   - Check if service is actually running
   - Review Railway logs for startup errors

---

## Quick Commands

### Generate CRON_SECRET
```bash
openssl rand -base64 32
```

### Test Health Endpoint
```bash
curl https://your-app.up.railway.app/cron/health
```

### Check Railway Logs (via CLI)
```bash
npm install -g @railway/cli
railway login
railway link [PROJECT_ID]
railway logs
```

### Test EPX Connection (from logs)
Look for these log entries:
```
INFO  epx/server_post_adapter.go:108  Processing EPX Server Post transaction
INFO  epx/server_post_adapter.go:179  Successfully processed Server Post transaction
```

---

## Next Steps After Successful Staging

1. ✅ Staging is deployed and working
2. Monitor staging for 1-2 weeks
3. Fix any issues that arise
4. Set up production environment (separate Railway project)
5. Create production branch deployment workflow
6. Deploy to production with production EPX credentials

---

## Need Help?

- Full Setup Guide: [RAILWAY_SETUP.md](./RAILWAY_SETUP.md)
- Environment Config: [ENVIRONMENT_SETUP.md](./ENVIRONMENT_SETUP.md)
- Deployment Guide: [DEPLOYMENT.md](./DEPLOYMENT.md)
- Railway Docs: https://docs.railway.app
- Railway Discord: https://discord.gg/railway
