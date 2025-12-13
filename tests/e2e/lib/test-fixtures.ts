import { test as base, APIRequestContext } from '@playwright/test';
import * as crypto from 'crypto';
import * as jwt from 'jsonwebtoken';
import { getConfig } from './config';
import { TestMerchant, TestService, TestContext } from './types';

/**
 * Get EPX configuration lazily from centralized config.
 * Config is validated at test runtime, not module load time.
 */
function getEPXConfig() {
  const config = getConfig();
  return {
    custNbr: config.epx.custNbr,
    merchNbr: config.epx.merchNbr,
    dbaNbr: config.epx.dbaNbr,
    terminalNbr: config.epx.terminalNbr,
    macSecret: config.epx.macSecret, // Actual MAC secret value
  };
}

/**
 * Generate RSA keypair for JWT signing
 */
function generateRSAKeyPair(): { privateKey: string; publicKey: string } {
  const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', {
    modulusLength: 2048,
    publicKeyEncoding: { type: 'spki', format: 'pem' },
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
  });
  return { privateKey, publicKey };
}

/**
 * Generate JWT token for API authentication
 */
function generateJWT(privateKey: string, serviceId: string, merchantId: string, expiresInHours = 1): string {
  const now = Math.floor(Date.now() / 1000);

  const payload = {
    iss: serviceId,
    sub: serviceId,
    iat: now,
    nbf: now,
    exp: now + (expiresInHours * 3600),
    jti: crypto.randomUUID(),
    merchant_id: merchantId,
    scopes: [
      'payments:create',
      'payments:read',
      'payments:void',
      'payments:refund',
      'payment_methods:read',
      'payment_methods:create',
      'subscriptions:manage',
      'subscriptions:read',
    ],
  };

  return jwt.sign(payload, privateKey, { algorithm: 'RS256' });
}

/**
 * Create isolated test data via direct database connection
 * Uses pg client to insert test merchant/service
 */
async function createTestContext(request: APIRequestContext): Promise<TestContext> {
  const timestamp = Date.now();
  const merchantId = crypto.randomUUID();
  const serviceId = crypto.randomUUID();
  const serviceIdentifier = `e2e-service-${timestamp}`;

  // Generate RSA keypair
  const { privateKey, publicKey } = generateRSAKeyPair();

  // Create merchant via internal admin API (if available)
  // For now, we need to use direct DB insert or have the server create it

  // Option 1: Use an admin/setup endpoint (recommended for E2E)
  // Option 2: Direct database insert via pg client
  // Option 3: Use the Go factory via a setup script

  // For this implementation, we'll try to call an internal setup endpoint
  // If that doesn't exist, tests will skip with clear message

  const setupEndpoint = '/internal/e2e/setup';
  const epxConfig = getEPXConfig();

  try {
    const response = await request.post(setupEndpoint, {
      data: {
        merchant: {
          id: merchantId,
          slug: `e2e-merchant-${timestamp}`,
          name: `E2E Test Merchant ${timestamp}`,
          cust_nbr: epxConfig.custNbr,
          merch_nbr: epxConfig.merchNbr,
          dba_nbr: epxConfig.dbaNbr,
          terminal_nbr: epxConfig.terminalNbr,
          mac_secret: epxConfig.macSecret, // Pass actual MAC secret to be stored in secret manager
          environment: 'staging',
        },
        service: {
          id: serviceId,
          service_id: serviceIdentifier,
          name: `E2E Test Service ${timestamp}`,
          public_key: publicKey,
        },
      },
    });

    if (response.ok()) {
      const data = await response.json();

      const merchant: TestMerchant = {
        id: data.merchant.id || merchantId,
        slug: data.merchant.slug,
        name: data.merchant.name,
        custNbr: epxConfig.custNbr,
        merchNbr: epxConfig.merchNbr,
        dbaNbr: epxConfig.dbaNbr,
        terminalNbr: epxConfig.terminalNbr,
      };

      const service: TestService = {
        id: data.service.id || serviceId,
        serviceId: serviceIdentifier,
        privateKey,
        publicKey,
      };

      const token = generateJWT(privateKey, serviceIdentifier, merchant.id);

      return { merchant, service, token };
    }
  } catch {
    // Setup endpoint not available, fall through to error
  }

  // If we get here, setup endpoint isn't available
  // Throw error with instructions
  throw new Error(`
E2E Test Setup Failed

The E2E tests require isolated test data for each test run.
The internal setup endpoint is not available.

Options:
1. Add /internal/e2e/setup endpoint to your server (recommended)
2. Use external setup script before running tests
3. Set TEST_MERCHANT_ID and TEST_JWT_TOKEN environment variables

See tests/e2e/README.md for details.
`);
}

/**
 * Cleanup test data after test
 */
async function cleanupTestContext(request: APIRequestContext, ctx: TestContext): Promise<void> {
  try {
    await request.post('/internal/e2e/cleanup', {
      data: {
        merchant_id: ctx.merchant.id,
        service_id: ctx.service.id,
      },
    });
  } catch {
    // Cleanup endpoint not available - data will remain
    // This is acceptable for E2E tests as they use unique IDs
  }
}

// Extend base test with fixtures
type TestFixtures = {
  testContext: TestContext;
};

export const test = base.extend<TestFixtures>({
  testContext: async ({ request }, use) => {
    // Check if using pre-configured test data
    if (process.env.TEST_MERCHANT_ID && process.env.TEST_JWT_TOKEN) {
      // Use environment-provided test data
      const epxConfig = getEPXConfig();
      const ctx: TestContext = {
        merchant: {
          id: process.env.TEST_MERCHANT_ID,
          slug: 'env-configured-merchant',
          name: 'Environment Configured Merchant',
          custNbr: epxConfig.custNbr,
          merchNbr: epxConfig.merchNbr,
          dbaNbr: epxConfig.dbaNbr,
          terminalNbr: epxConfig.terminalNbr,
        },
        service: {
          id: 'env-configured-service',
          serviceId: 'env-configured-service',
          privateKey: process.env.TEST_SERVICE_PRIVATE_KEY || '',
          publicKey: '',
        },
        token: process.env.TEST_JWT_TOKEN,
      };

      await use(ctx);
      return;
    }

    // Create isolated test data
    const ctx = await createTestContext(request);

    // Use the test context in test
    await use(ctx);

    // Cleanup after test
    await cleanupTestContext(request, ctx);
  },
});

export { expect } from '@playwright/test';
