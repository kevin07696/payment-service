import { APIRequestContext } from '@playwright/test';
import * as crypto from 'crypto';
import * as jwt from 'jsonwebtoken';
import { getConfig } from './config';
import { TestMerchant, TestService, TestContext } from './types';

/**
 * Get EPX configuration lazily from centralized config.
 * Config is validated at test runtime, not module load time.
 */
function getEPXDefaults() {
  const config = getConfig();
  return {
    custNbr: config.epx.custNbr,
    merchNbr: config.epx.merchNbr,
    dbaNbr: config.epx.dbaNbr,
    terminalNbr: config.epx.terminalNbr,
  };
}

/**
 * API client for setting up test data via backend API
 */
export class ApiClient {
  constructor(
    private request: APIRequestContext,
    private baseUrl: string
  ) {}

  /**
   * Create a complete test context with merchant, service, and JWT token
   */
  async createTestContext(): Promise<TestContext> {
    // Generate RSA keypair for service
    const { privateKey, publicKey } = generateRSAKeyPair();

    const timestamp = Date.now();
    const merchantId = crypto.randomUUID();
    const serviceId = crypto.randomUUID();

    // Create merchant via direct DB insert (requires admin endpoint or seed data)
    // For now, we'll use environment variables for pre-seeded test merchant
    const epxDefaults = getEPXDefaults();
    const merchant: TestMerchant = {
      id: process.env.TEST_MERCHANT_ID || merchantId,
      slug: `e2e-merchant-${timestamp}`,
      name: `E2E Test Merchant ${timestamp}`,
      custNbr: epxDefaults.custNbr,
      merchNbr: epxDefaults.merchNbr,
      dbaNbr: epxDefaults.dbaNbr,
      terminalNbr: epxDefaults.terminalNbr,
    };

    const service: TestService = {
      id: process.env.TEST_SERVICE_ID || serviceId,
      serviceId: `e2e-service-${timestamp}`,
      privateKey,
      publicKey,
    };

    // Generate JWT token
    const token = generateJWT(privateKey, service.serviceId, merchant.id);

    return { merchant, service, token };
  }

  /**
   * Get browser post form configuration
   */
  async getBrowserPostForm(
    token: string,
    merchantId: string,
    amount: string,
    transactionType: 'SALE' | 'AUTH' | 'STORAGE' = 'SALE',
    returnUrl?: string
  ) {
    const transactionId = crypto.randomUUID();
    const callbackUrl = returnUrl || `${this.baseUrl}/api/v1/payments/browser-post/callback`;

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: merchantId,
      amount,
      transaction_type: transactionType,
      return_url: callbackUrl,
    });

    const response = await this.request.get(
      `/api/v1/payments/browser-post/form?${params}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to get browser post form: ${response.status()} - ${body}`);
    }

    return response.json();
  }

  /**
   * List transactions for a merchant
   */
  async listTransactions(token: string, merchantId: string) {
    const response = await this.request.post(
      '/payment.v1.PaymentService/ListTransactions',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          merchant_id: merchantId,
          page_size: 10,
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to list transactions: ${response.status()} - ${body}`);
    }

    return response.json();
  }

  /**
   * Get transaction by ID
   */
  async getTransaction(token: string, transactionId: string) {
    const response = await this.request.post(
      '/payment.v1.PaymentService/GetTransaction',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to get transaction: ${response.status()} - ${body}`);
    }

    return response.json();
  }

  /**
   * Capture an authorized transaction
   */
  async captureTransaction(token: string, transactionId: string, amount?: number) {
    const response = await this.request.post(
      '/payment.v1.PaymentService/Capture',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
          ...(amount && { amount }),
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to capture transaction: ${response.status()} - ${body}`);
    }

    return response.json();
  }

  /**
   * Void a transaction
   */
  async voidTransaction(token: string, transactionId: string) {
    const response = await this.request.post(
      '/payment.v1.PaymentService/Void',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to void transaction: ${response.status()} - ${body}`);
    }

    return response.json();
  }

  /**
   * Refund a transaction
   */
  async refundTransaction(token: string, transactionId: string, amount?: number) {
    const response = await this.request.post(
      '/payment.v1.PaymentService/Refund',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
          ...(amount && { amount }),
        },
      }
    );

    if (!response.ok()) {
      const body = await response.text();
      throw new Error(`Failed to refund transaction: ${response.status()} - ${body}`);
    }

    return response.json();
  }
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
function generateJWT(privateKey: string, serviceId: string, merchantId: string): string {
  const now = Math.floor(Date.now() / 1000);

  const payload = {
    iss: serviceId,
    sub: serviceId,
    iat: now,
    nbf: now,
    exp: now + 3600, // 1 hour
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
