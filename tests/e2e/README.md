# E2E Tests

End-to-end tests for the payment service using [Playwright](https://playwright.dev/).

## Prerequisites

1. Payment service running locally:
   ```bash
   podman-compose up -d
   ```

2. Test merchant with EPX sandbox credentials configured

3. **Required** environment variables (no defaults - fail fast if missing):
   ```bash
   # Service URL
   export SERVICE_URL="http://localhost:8081"

   # EPX sandbox credentials
   export EPX_CUST_NBR="9001"
   export EPX_MERCH_NBR="900300"
   export EPX_DBA_NBR="2"
   export EPX_TERMINAL_NBR="77"
   export EPX_MAC_SECRET_PATH="epx/staging/mac_secret"

   # Optional: Use pre-seeded test data instead of dynamic setup
   export TEST_MERCHANT_ID="your-merchant-uuid"
   export TEST_JWT_TOKEN="your-jwt-token"
   ```

## Running Tests

```bash
cd tests/e2e

# Run all tests (headless)
npm test

# Run with browser UI visible
npm run test:headed

# Run with Playwright UI (interactive)
npm run test:ui

# Debug mode (step through tests)
npm run test:debug

# View test report
npm run report
```

## Test Structure

```
tests/e2e/
├── lib/
│   ├── config.ts        # Centralized configuration (validated at runtime)
│   ├── api-client.ts    # API client for backend calls
│   ├── test-fixtures.ts # Playwright fixtures for test context
│   └── test-cards.ts    # Test card definitions
├── tests/
│   ├── browser-post-sale.spec.ts         # SALE flow tests
│   ├── browser-post-auth-capture.spec.ts # AUTH/CAPTURE flow tests
│   ├── browser-post-storage.spec.ts      # Card storage tests
│   └── browser-post-ach-storage.spec.ts  # ACH storage tests
├── playwright.config.ts
└── package.json
```

## Test Flows

### Browser Post SALE
1. Get form config from `/api/v1/payments/browser-post/form`
2. Submit card data to EPX via browser
3. EPX redirects back to callback
4. Verify transaction is APPROVED

### Browser Post AUTH + CAPTURE
1. Get form config for AUTH
2. Submit to EPX, verify AUTHORIZED status
3. Call CAPTURE endpoint
4. Verify CAPTURED status

### Browser Post AUTH + VOID
1. Get form config for AUTH
2. Submit to EPX, verify AUTHORIZED status
3. Call VOID endpoint
4. Verify VOIDED status

## CI/CD

E2E tests should run:
- On merge to main (post-deploy verification)
- Nightly (catch regressions)
- On-demand (manual trigger)

Not recommended for every PR due to:
- Slower execution (real network calls)
- Potential EPX rate limits
- Higher flakiness risk
