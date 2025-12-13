/**
 * Server Post Helper Functions for E2E Tests
 *
 * These helpers enable testing Server Post (direct API) flows that require a BRIC
 * (Bank Reference Identifier Code) obtained via Browser POST.
 *
 * Flow:
 * 1. Browser POST STORAGE (CCE8) → Returns BRIC (AUTH_GUID)
 * 2. Server Post uses BRIC for: Sale (CCE1), Auth (CCE2), Capture (CCE4), etc.
 */

import { Page, APIRequestContext } from '@playwright/test';
import { TestCard, formatExpDate } from './test-cards';
import { TestContext } from './types';
import { retryUntilCondition, quickRetryConfig } from './retry';

/**
 * Server Post response from our payment service
 */
export interface ServerPostResponse {
  transactionId: string;
  status: string;
  authorizationCode?: string;
  processorResponseCode?: string;
  processorResponseMessage?: string;
  bric?: string;
  amountCents?: string;
}

/**
 * Build HTML form for EPX BRIC Storage submission
 */
function buildStorageForm(formConfig: Record<string, string>, card: TestCard): string {
  const expDate = formatExpDate(card);

  return `
    <!DOCTYPE html>
    <html>
    <head><title>EPX BRIC Storage Form</title></head>
    <body>
      <form id="epxForm" method="POST" action="${formConfig.postURL}">
        <input type="hidden" name="TAC" value="${formConfig.tac}" />
        <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}" />
        <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}" />
        <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}" />
        <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}" />
        <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}" />
        <input type="hidden" name="AMOUNT" value="0.00" />
        <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}" />
        <input type="hidden" name="ACCOUNT_NBR" value="${card.number}" />
        <input type="hidden" name="EXP_DATE" value="${expDate}" />
        <input type="hidden" name="CVV2" value="${card.cvv}" />
        <input type="hidden" name="FIRST_NAME" value="Test" />
        <input type="hidden" name="LAST_NAME" value="User" />
        <input type="hidden" name="ADDRESS" value="123 N CENTRAL" />
        <input type="hidden" name="CITY" value="TestCity" />
        <input type="hidden" name="STATE" value="TX" />
        <input type="hidden" name="ZIP_CODE" value="${card.zip}" />
      </form>
      <script>document.getElementById('epxForm').submit();</script>
    </body>
    </html>
  `;
}

/**
 * Get a BRIC (tokenized card) via Browser POST Storage flow
 *
 * This is required before running Server Post transactions because
 * Server Post requires a BRIC reference, not raw card data.
 *
 * @param page - Playwright page for form submission
 * @param request - Playwright request context for API calls
 * @param testContext - Test context with auth credentials
 * @param card - Test card to tokenize
 * @returns BRIC token (AUTH_GUID from EPX)
 */
export async function getBRICviaBrowserPost(
  page: Page,
  request: APIRequestContext,
  testContext: TestContext,
  card: TestCard
): Promise<string> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
  const transactionId = crypto.randomUUID();

  // Step 1: Get form configuration for STORAGE
  const formParams = new URLSearchParams({
    transaction_id: transactionId,
    merchant_id: testContext.merchant.id,
    amount: '0.00', // Storage doesn't charge
    transaction_type: 'STORAGE',
    return_url: `${baseUrl}/api/v1/payments/browser-post/callback`,
  });

  const formResponse = await request.get(
    `${baseUrl}/api/v1/payments/browser-post/form?${formParams}`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
      },
    }
  );

  if (!formResponse.ok()) {
    const body = await formResponse.text();
    throw new Error(`Failed to get storage form: ${formResponse.status()} - ${body}`);
  }

  const formConfig = await formResponse.json();

  // Step 2: Submit storage form to EPX
  const formHtml = buildStorageForm(formConfig, card);
  await page.setContent(formHtml);

  // Wait for EPX redirect
  await page.waitForURL(/callback|return/, { timeout: 30000 });

  // Step 3: Poll until transaction has BRIC populated
  const transaction = await retryUntilCondition(
    async () => {
      const resp = await request.post(
        `${baseUrl}/payment.v1.PaymentService/GetTransaction`,
        {
          headers: {
            Authorization: `Bearer ${testContext.token}`,
            'Content-Type': 'application/json',
          },
          data: { transaction_id: transactionId },
        }
      );
      if (!resp.ok()) return null;
      return await resp.json();
    },
    (txn) => !!(txn?.processorReference || txn?.paymentMethodId),
    quickRetryConfig()
  );

  // The BRIC is stored as the processor reference or payment method token
  const bric = transaction.processorReference || transaction.paymentMethodId;

  if (!bric) {
    throw new Error(`No BRIC returned from storage transaction: ${JSON.stringify(transaction)}`);
  }

  return bric;
}

/**
 * Execute a Server Post Sale using a BRIC
 *
 * @param request - Playwright request context
 * @param testContext - Test context with auth credentials
 * @param bric - BRIC token from storage
 * @param amount - Amount in dollars (e.g., "1.00")
 * @returns Server Post response
 */
export async function serverPostSale(
  request: APIRequestContext,
  testContext: TestContext,
  bric: string,
  amount: string
): Promise<ServerPostResponse> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
  const transactionId = crypto.randomUUID();

  // Use the Sale RPC with the BRIC as payment token
  const response = await request.post(
    `${baseUrl}/payment.v1.PaymentService/Sale`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
        'Content-Type': 'application/json',
      },
      data: {
        merchant_id: testContext.merchant.id,
        payment_token: bric, // BRIC from browser post storage
        amount_cents: Math.round(parseFloat(amount) * 100),
        currency: 'USD',
        idempotency_key: transactionId,
      },
    }
  );

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Server Post Sale failed: ${response.status()} - ${body}`);
  }

  const result = await response.json();

  return {
    transactionId: result.transaction?.id || transactionId,
    status: result.transaction?.status || 'UNKNOWN',
    authorizationCode: result.transaction?.authorizationCode,
    processorResponseCode: result.transaction?.processorResponseCode,
    processorResponseMessage: result.transaction?.processorResponseMessage,
    bric: result.transaction?.processorReference,
    amountCents: result.transaction?.amountCents,
  };
}

/**
 * Execute a Server Post Auth-only using a BRIC
 *
 * @param request - Playwright request context
 * @param testContext - Test context with auth credentials
 * @param bric - BRIC token from storage
 * @param amount - Amount in dollars (e.g., "50.00")
 * @returns Server Post response
 */
export async function serverPostAuth(
  request: APIRequestContext,
  testContext: TestContext,
  bric: string,
  amount: string
): Promise<ServerPostResponse> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
  const transactionId = crypto.randomUUID();

  // Use Authorize RPC with the BRIC as payment token
  const response = await request.post(
    `${baseUrl}/payment.v1.PaymentService/Authorize`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
        'Content-Type': 'application/json',
      },
      data: {
        merchant_id: testContext.merchant.id,
        payment_token: bric, // BRIC from browser post storage
        amount_cents: Math.round(parseFloat(amount) * 100),
        currency: 'USD',
        idempotency_key: transactionId,
      },
    }
  );

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Server Post Auth failed: ${response.status()} - ${body}`);
  }

  const result = await response.json();

  return {
    transactionId: result.transaction?.id || transactionId,
    status: result.transaction?.status || 'UNKNOWN',
    authorizationCode: result.transaction?.authorizationCode,
    processorResponseCode: result.transaction?.processorResponseCode,
    processorResponseMessage: result.transaction?.processorResponseMessage,
    bric: result.transaction?.processorReference,
    amountCents: result.transaction?.amountCents,
  };
}

/**
 * Execute a Server Post Capture on an authorized transaction
 *
 * @param request - Playwright request context
 * @param testContext - Test context with auth credentials
 * @param transactionId - Transaction ID to capture
 * @param amount - Amount in dollars (optional, for partial capture)
 * @returns Server Post response
 */
export async function serverPostCapture(
  request: APIRequestContext,
  testContext: TestContext,
  transactionId: string,
  amount?: string
): Promise<ServerPostResponse> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

  const requestData: Record<string, string | number> = {
    transaction_id: transactionId,
  };

  if (amount) {
    requestData.amount_cents = Math.round(parseFloat(amount) * 100);
  }

  const response = await request.post(
    `${baseUrl}/payment.v1.PaymentService/Capture`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
        'Content-Type': 'application/json',
      },
      data: requestData,
    }
  );

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Server Post Capture failed: ${response.status()} - ${body}`);
  }

  const result = await response.json();

  return {
    transactionId: result.transaction?.id || transactionId,
    status: result.transaction?.status || 'UNKNOWN',
    authorizationCode: result.transaction?.authorizationCode,
    processorResponseCode: result.transaction?.processorResponseCode,
    processorResponseMessage: result.transaction?.processorResponseMessage,
    bric: result.transaction?.processorReference,
    amountCents: result.transaction?.amountCents,
  };
}

/**
 * Execute a Server Post Refund on a captured transaction
 *
 * @param request - Playwright request context
 * @param testContext - Test context with auth credentials
 * @param transactionId - Transaction ID to refund
 * @param amount - Amount in dollars (optional, for partial refund)
 * @returns Server Post response
 */
export async function serverPostRefund(
  request: APIRequestContext,
  testContext: TestContext,
  transactionId: string,
  amount?: string
): Promise<ServerPostResponse> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

  const requestData: Record<string, string | number> = {
    transaction_id: transactionId,
  };

  if (amount) {
    requestData.amount_cents = Math.round(parseFloat(amount) * 100);
  }

  const response = await request.post(
    `${baseUrl}/payment.v1.PaymentService/Refund`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
        'Content-Type': 'application/json',
      },
      data: requestData,
    }
  );

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Server Post Refund failed: ${response.status()} - ${body}`);
  }

  const result = await response.json();

  return {
    transactionId: result.refundTransaction?.id || transactionId,
    status: result.refundTransaction?.status || 'UNKNOWN',
    authorizationCode: result.refundTransaction?.authorizationCode,
    processorResponseCode: result.refundTransaction?.processorResponseCode,
    processorResponseMessage: result.refundTransaction?.processorResponseMessage,
    bric: result.refundTransaction?.processorReference,
    amountCents: result.refundTransaction?.amountCents,
  };
}

/**
 * Execute a Server Post Void on a transaction
 *
 * @param request - Playwright request context
 * @param testContext - Test context with auth credentials
 * @param transactionId - Transaction ID to void
 * @returns Server Post response
 */
export async function serverPostVoid(
  request: APIRequestContext,
  testContext: TestContext,
  transactionId: string
): Promise<ServerPostResponse> {
  const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

  const response = await request.post(
    `${baseUrl}/payment.v1.PaymentService/Void`,
    {
      headers: {
        Authorization: `Bearer ${testContext.token}`,
        'Content-Type': 'application/json',
      },
      data: {
        transaction_id: transactionId,
      },
    }
  );

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Server Post Void failed: ${response.status()} - ${body}`);
  }

  const result = await response.json();

  return {
    transactionId: result.transaction?.id || transactionId,
    status: result.transaction?.status || 'UNKNOWN',
    authorizationCode: result.transaction?.authorizationCode,
    processorResponseCode: result.transaction?.processorResponseCode,
    processorResponseMessage: result.transaction?.processorResponseMessage,
    bric: result.transaction?.processorReference,
    amountCents: result.transaction?.amountCents,
  };
}

/**
 * EPX Amount-based response triggers for testing
 * Per EPX documentation, specific amounts trigger specific response codes
 */
export const EPXAmountTriggers = {
  /** $1.00 = Approval (00) */
  APPROVAL: '1.00',
  /** $1.05 = Decline (05) */
  DECLINE_05: '1.05',
  /** $1.14 = Invalid card number (14) */
  INVALID_CARD: '1.14',
  /** $1.51 = Not sufficient funds (51) */
  NSF: '1.51',
  /** $1.54 = Expired card (54) */
  EXPIRED_CARD: '1.54',
  /** $1.61 = Exceeds withdrawal limit (61) */
  WITHDRAWAL_LIMIT: '1.61',
} as const;
