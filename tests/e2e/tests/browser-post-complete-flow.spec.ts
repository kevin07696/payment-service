import { test, expect } from '../lib/test-fixtures';
import { VISA_APPROVAL, formatExpDate } from '../lib/test-cards';
import { retryUntilCondition, quickRetryConfig } from '../lib/retry';

/**
 * Browser POST Complete Flow E2E Test
 *
 * This test documents and verifies the complete Browser POST payment flow:
 *
 * ┌─────────────────────────────────────────────────────────────────────────────┐
 * │                         Browser POST Payment Flow                           │
 * └─────────────────────────────────────────────────────────────────────────────┘
 *
 * 1. GET /api/v1/payments/browser-post/form  (Our Payment Service)
 *    ├── Client sends: transaction_id, merchant_id, amount, transaction_type, return_url
 *    ├── JWT Auth validates service has access to merchant
 *    ├── Service calls EPX Key Exchange internally → Gets TAC token
 *    ├── Creates PENDING transaction in database
 *    └── Returns form config: tac, postURL, tranCode, custNbr, merchNbr, etc.
 *
 * 2. POST to EPX (formConfig.postURL)  (Direct to EPX)
 *    ├── Browser submits HTML form with:
 *    │   └── TAC, TRAN_CODE, CUST_NBR, MERCH_NBR, AMOUNT, ACCOUNT_NBR, EXP_DATE, CVV2, AVS fields
 *    └── EPX processes payment and redirects via 302
 *
 * 3. GET /api/v1/payments/browser-post/callback  (EPX redirects to Our Service)
 *    ├── EPX response includes: AUTH_GUID, AUTH_RESP, AUTH_CODE, etc.
 *    ├── Validates MAC signature
 *    ├── Updates transaction status (APPROVED/DECLINED)
 *    └── Returns HTML receipt page
 *
 * 4. Verify via GetTransaction RPC  (API verification)
 *    └── Confirms transaction status, auth code, and amount
 */

/**
 * Build HTML form for EPX submission
 */
function buildEPXForm(formConfig: Record<string, string>, card: typeof VISA_APPROVAL, amount: string): string {
  const expDate = formatExpDate(card);

  return `
    <!DOCTYPE html>
    <html>
    <head><title>EPX Payment Form</title></head>
    <body>
      <form id="epxForm" method="POST" action="${formConfig.postURL}">
        <input type="hidden" name="TAC" value="${formConfig.tac}" />
        <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}" />
        <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}" />
        <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}" />
        <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}" />
        <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}" />
        <input type="hidden" name="AMOUNT" value="${amount}" />
        <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}" />
        <input type="hidden" name="ACCOUNT_NBR" value="${card.number}" />
        <input type="hidden" name="EXP_DATE" value="${expDate}" />
        <input type="hidden" name="CVV2" value="${card.cvv}" />
        <input type="hidden" name="FIRST_NAME" value="Test" />
        <input type="hidden" name="LAST_NAME" value="User" />
        <input type="hidden" name="ADDRESS" value="123 Test St" />
        <input type="hidden" name="CITY" value="TestCity" />
        <input type="hidden" name="STATE" value="TX" />
        <input type="hidden" name="ZIP_CODE" value="${card.zip}" />
      </form>
      <script>document.getElementById('epxForm').submit();</script>
    </body>
    </html>
  `;
}

test.describe('Browser POST Complete Flow', () => {
  test('documents and verifies complete SALE flow with all verification steps', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
    const transactionId = crypto.randomUUID();
    const amount = '42.50';
    const amountCents = '4250';

    console.log('═══════════════════════════════════════════════════════════════');
    console.log('           BROWSER POST COMPLETE FLOW TEST                      ');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log(`Transaction ID: ${transactionId}`);
    console.log(`Merchant ID: ${testContext.merchant.id}`);
    console.log(`Amount: $${amount}`);
    console.log('');

    // ════════════════════════════════════════════════════════════════════════
    // STEP 1: Get Form Configuration (includes internal Key Exchange)
    // ════════════════════════════════════════════════════════════════════════
    console.log('STEP 1: GET /api/v1/payments/browser-post/form');
    console.log('────────────────────────────────────────────────────────────────');
    console.log('  → Sending request with JWT authentication');
    console.log('  → Service performs EPX Key Exchange internally');
    console.log('  → Creates PENDING transaction in database');

    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const formParams = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount,
      transaction_type: 'SALE',
      return_url: callbackUrl,
    });

    const formResponse = await request.get(
      `${baseUrl}/api/v1/payments/browser-post/form?${formParams}`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
        },
      }
    );

    expect(formResponse.ok(), `Form request failed: ${await formResponse.text()}`).toBeTruthy();

    const formConfig = await formResponse.json();

    // Verify all required form fields are present
    console.log('  ✓ Form configuration received');
    console.log(`    - TAC: ${formConfig.tac?.substring(0, 20)}...`);
    console.log(`    - POST URL: ${formConfig.postURL}`);
    console.log(`    - TRAN_CODE: ${formConfig.tranCode}`);
    console.log(`    - EPX TRAN_NBR: ${formConfig.epxTranNbr}`);

    expect(formConfig.tac).toBeTruthy();
    expect(formConfig.postURL).toBeTruthy();
    expect(formConfig.tranCode).toBeTruthy();
    expect(formConfig.epxTranNbr).toBeTruthy();
    expect(formConfig.custNbr).toBeTruthy();
    expect(formConfig.merchNbr).toBeTruthy();
    expect(formConfig.dbaName).toBeTruthy();
    expect(formConfig.terminalNbr).toBeTruthy();
    expect(formConfig.industryType).toBeTruthy();

    console.log('');

    // ════════════════════════════════════════════════════════════════════════
    // STEP 2: Submit Payment Form to EPX
    // ════════════════════════════════════════════════════════════════════════
    console.log('STEP 2: POST to EPX (Browser Form Submission)');
    console.log('────────────────────────────────────────────────────────────────');
    console.log(`  → Submitting card to: ${formConfig.postURL}`);
    console.log('  → Card: VISA ending in 0002 (test approval card)');

    const card = VISA_APPROVAL;
    const formHtml = buildEPXForm(formConfig, card, amount);

    await page.setContent(formHtml);

    console.log('  → Waiting for EPX processing and redirect...');

    // Wait for EPX to redirect back to our callback
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const callbackUrlReceived = page.url();
    console.log('  ✓ EPX redirect received');
    console.log(`    - Callback URL: ${callbackUrlReceived.substring(0, 80)}...`);
    console.log('');

    // ════════════════════════════════════════════════════════════════════════
    // STEP 3: Callback Processing (automatic via redirect)
    // ════════════════════════════════════════════════════════════════════════
    console.log('STEP 3: Callback Processing');
    console.log('────────────────────────────────────────────────────────────────');
    console.log('  → EPX redirected to /api/v1/payments/browser-post/callback');
    console.log('  → Service validates MAC signature');
    console.log('  → Updates transaction from PENDING to APPROVED/DECLINED');
    console.log('  → Returns HTML receipt page');

    // Check if callback URL indicates success
    expect(callbackUrlReceived).toContain('callback');

    // Check page content for receipt or error
    const pageContent = await page.content();
    const hasApproval = pageContent.toLowerCase().includes('approved') ||
                        pageContent.toLowerCase().includes('success') ||
                        pageContent.toLowerCase().includes('receipt');

    console.log(`  ✓ Receipt page rendered (approval indicators: ${hasApproval})`);
    console.log('');

    // ════════════════════════════════════════════════════════════════════════
    // STEP 4: API Verification
    // ════════════════════════════════════════════════════════════════════════
    console.log('STEP 4: API Verification (GetTransaction)');
    console.log('────────────────────────────────────────────────────────────────');
    console.log('  → Calling GetTransaction to verify database state');

    // Poll until transaction is approved
    const transaction = await retryUntilCondition(
      async () => {
        const resp = await request.post(
          `${baseUrl}/payment.v1.PaymentService/GetTransaction`,
          {
            headers: {
              Authorization: `Bearer ${testContext.token}`,
              'Content-Type': 'application/json',
              'Accept-Encoding': 'identity',
            },
            data: {
              transaction_id: transactionId,
            },
          }
        );
        if (!resp.ok()) return null;
        return await resp.json();
      },
      (txn) => txn?.status === 'TRANSACTION_STATUS_APPROVED',
      quickRetryConfig()
    );

    console.log('  ✓ Transaction retrieved from database');
    console.log(`    - ID: ${transaction.id}`);
    console.log(`    - Status: ${transaction.status}`);
    console.log(`    - Type: ${transaction.type}`);
    console.log(`    - Amount: ${transaction.amountCents} cents`);
    console.log(`    - Auth Code: ${transaction.authorizationCode || 'N/A'}`);
    console.log(`    - Merchant ID: ${transaction.merchantId}`);

    // Verify transaction details
    expect(transaction.id).toBe(transactionId);
    expect(transaction.merchantId).toBe(testContext.merchant.id);
    expect(transaction.type).toBe('TRANSACTION_TYPE_CHARGE');
    expect(transaction.amountCents).toBe(amountCents);

    // Verify EPX approval
    expect(transaction.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(transaction.authorizationCode).toBeTruthy();

    console.log('');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('                    TEST PASSED                                 ');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log(`✓ SALE transaction ${transactionId} completed successfully`);
    console.log(`✓ Amount: $${amount} (${amountCents} cents)`);
    console.log(`✓ Authorization Code: ${transaction.authorizationCode}`);
    console.log('');
  });

  test('verifies form endpoint returns all required fields for EPX submission', async ({ request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
    const transactionId = crypto.randomUUID();

    console.log('Testing form endpoint field completeness...');

    const formParams = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount: '10.00',
      transaction_type: 'SALE',
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

    expect(formResponse.ok(), `Form request failed: ${await formResponse.text()}`).toBeTruthy();

    const formConfig = await formResponse.json();

    // Required fields for EPX Browser POST form
    const requiredFields = [
      'tac',           // TAC token from Key Exchange
      'postURL',       // EPX submission URL
      'tranCode',      // SALE, AUTH, STORAGE, etc.
      'custNbr',       // EPX customer number
      'merchNbr',      // EPX merchant number
      'dbaName',       // DBA number
      'terminalNbr',   // Terminal number
      'industryType',  // E for e-commerce
    ];

    // Optional but useful fields
    const optionalFields = [
      'transactionId', // Our transaction ID
      'epxTranNbr',    // Hashed EPX transaction number
      'expiresAt',     // TAC expiration timestamp
      'merchantName',  // Display name
    ];

    console.log('Required fields:');
    for (const field of requiredFields) {
      const value = formConfig[field];
      const status = value ? '✓' : '✗';
      console.log(`  ${status} ${field}: ${value ? (typeof value === 'string' && value.length > 30 ? value.substring(0, 30) + '...' : value) : 'MISSING'}`);
      expect(formConfig[field], `Missing required field: ${field}`).toBeTruthy();
    }

    console.log('Optional fields:');
    for (const field of optionalFields) {
      const value = formConfig[field];
      const status = value ? '✓' : '○';
      console.log(`  ${status} ${field}: ${value ?? 'not provided'}`);
    }

    // Note: Some internal fields may be present but are not required for form submission
    // The critical thing is that all REQUIRED fields are present
    const internalFields = ['redirectURL', 'returnUrl', 'merchantId'];
    console.log('Internal fields (informational only):');
    for (const field of internalFields) {
      const status = formConfig[field] ? 'present' : 'absent';
      console.log(`  ○ ${field}: ${status}`);
    }

    console.log('');
    console.log('Form field validation passed!');
  });

  test('verifies transaction state transitions through flow', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
    const transactionId = crypto.randomUUID();
    const amount = '25.00';

    console.log('Testing transaction state transitions...');
    console.log(`Transaction ID: ${transactionId}`);

    // Step 1: Get form (creates PENDING transaction)
    const formParams = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount,
      transaction_type: 'SALE',
      return_url: `${baseUrl}/api/v1/payments/browser-post/callback`,
    });

    const formResponse = await request.get(
      `${baseUrl}/api/v1/payments/browser-post/form?${formParams}`,
      {
        headers: { Authorization: `Bearer ${testContext.token}` },
      }
    );

    expect(formResponse.ok()).toBeTruthy();
    const formConfig = await formResponse.json();

    // Verify PENDING state
    console.log('After form request:');
    const pendingTxn = await request.post(
      `${baseUrl}/payment.v1.PaymentService/GetTransaction`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
          'Content-Type': 'application/json',
        },
        data: { transaction_id: transactionId },
      }
    );

    expect(pendingTxn.ok(), `GetTransaction for pending failed: ${await pendingTxn.text()}`).toBeTruthy();
    const pendingState = await pendingTxn.json();
    console.log(`  Full response: ${JSON.stringify(pendingState, null, 2)}`);

    // Transaction should be in PENDING state after form request (before EPX submission)
    // Note: When status is PENDING (default/zero value), protobuf/JSON may omit the field
    // So we check for either missing status OR explicit PENDING status
    const isPending = !pendingState.status ||
                      pendingState.status === 'TRANSACTION_STATUS_PENDING' ||
                      pendingState.status === 'TRANSACTION_STATUS_UNSPECIFIED';
    console.log(`  Status: ${pendingState.status || '(omitted - PENDING)'}`);
    expect(isPending, `Expected PENDING status, got: ${pendingState.status}`).toBeTruthy();

    // Also verify no auth code yet (not processed by EPX)
    expect(pendingState.authorizationCode).toBeFalsy();

    // Step 2: Submit to EPX
    const card = VISA_APPROVAL;
    const formHtml = buildEPXForm(formConfig, card, amount);
    await page.setContent(formHtml);
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    // Verify APPROVED state (poll until approved)
    console.log('After EPX callback:');
    const approvedState = await retryUntilCondition(
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
      (txn) => txn?.status === 'TRANSACTION_STATUS_APPROVED',
      quickRetryConfig()
    );
    console.log(`  Status: ${approvedState.status}`);
    console.log(`  Auth Code: ${approvedState.authorizationCode}`);

    expect(approvedState.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(approvedState.authorizationCode).toBeTruthy();

    console.log('');
    console.log('State transitions verified:');
    console.log('  PENDING → APPROVED ✓');
  });
});
