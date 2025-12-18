import { test, expect } from '../lib/test-fixtures';
import { VISA_APPROVAL, formatExpDate } from '../lib/test-cards';
import { retryUntilCondition, quickRetryConfig } from '../lib/retry';

/**
 * Browser Post AUTH + CAPTURE Flow E2E Test
 *
 * Tests the two-step payment flow:
 * 1. AUTH - Authorize the card (hold funds)
 * 2. CAPTURE - Capture the authorized amount
 */
test.describe('Browser Post AUTH + CAPTURE Flow', () => {
  test('AUTH then full CAPTURE', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
    const transactionId = crypto.randomUUID();
    const amount = '75.00';
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    // Step 1: Get browser post form for AUTH
    console.log('Getting browser post form for AUTH...');

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount,
      transaction_type: 'AUTH',
      return_url: callbackUrl,
    });

    const formResponse = await request.get(
      `${baseUrl}/api/v1/payments/browser-post/form?${params}`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
        },
      }
    );

    expect(formResponse.ok(), `Form request failed: ${await formResponse.text()}`).toBeTruthy();

    const formConfig = await formResponse.json();
    expect(formConfig.tac).toBeTruthy();
    expect(formConfig.postURL).toBeTruthy();

    // Step 2: Submit AUTH to EPX
    const card = VISA_APPROVAL;
    const expDate = formatExpDate(card);

    // Credit card form includes AVS fields (ADDRESS, CITY, STATE, ZIP_CODE)
    const formHtml = `
      <!DOCTYPE html>
      <html>
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
          <input type="hidden" name="FIRST_NAME" value="John" />
          <input type="hidden" name="LAST_NAME" value="Doe" />
          <input type="hidden" name="ADDRESS" value="123 Main St" />
          <input type="hidden" name="CITY" value="New York" />
          <input type="hidden" name="STATE" value="NY" />
          <input type="hidden" name="ZIP_CODE" value="${card.zip}" />
        </form>
        <script>document.getElementById('epxForm').submit();</script>
      </body>
      </html>
    `;

    await page.setContent(formHtml);

    console.log('Waiting for EPX AUTH redirect...');
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const finalUrl = page.url();
    console.log(`AUTH callback URL: ${finalUrl}`);

    // Step 3: Verify AUTH transaction was created (poll until approved)
    const transaction = await retryUntilCondition(
      async () => {
        const resp = await request.post(
          `${baseUrl}/payment.v1.PaymentService/GetTransaction`,
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
        if (!resp.ok()) return null;
        return await resp.json();
      },
      (txn) => txn?.status === 'TRANSACTION_STATUS_APPROVED',
      quickRetryConfig()
    );
    console.log('AUTH transaction response:', JSON.stringify(transaction, null, 2));

    // Verify AUTH transaction
    expect(transaction.id).toBe(transactionId);
    expect(transaction.merchantId).toBe(testContext.merchant.id);
    expect(transaction.type).toBe('TRANSACTION_TYPE_AUTH');
    expect(transaction.amountCents).toBe('7500'); // $75.00 = 7500 cents

    // Verify AUTH was APPROVED by EPX
    expect(transaction.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(transaction.authorizationCode).toBeTruthy();

    console.log(`AUTH approved with auth code: ${transaction.authorizationCode}`);

    // Step 4: CAPTURE the authorized transaction via API
    console.log('Capturing authorized transaction...');
    const captureResponse = await request.post(
      `${baseUrl}/payment.v1.PaymentService/Capture`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
          idempotency_key: crypto.randomUUID(),
        },
      }
    );

    expect(captureResponse.ok(), `Capture failed: ${await captureResponse.text()}`).toBeTruthy();

    const captureResult = await captureResponse.json();
    console.log('CAPTURE result:', JSON.stringify(captureResult, null, 2));

    // Verify CAPTURE was APPROVED
    expect(captureResult.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(captureResult.isApproved).toBe(true);

    console.log('AUTH + CAPTURE flow completed successfully!');
  });

  test('AUTH then partial CAPTURE', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';
    const transactionId = crypto.randomUUID();
    const authAmount = '100.00';
    const captureAmountCents = 5000; // $50.00 in cents
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    // AUTH for $100
    console.log('Getting browser post form for AUTH ($100)...');

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount: authAmount,
      transaction_type: 'AUTH',
      return_url: callbackUrl,
    });

    const formResponse = await request.get(
      `${baseUrl}/api/v1/payments/browser-post/form?${params}`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
        },
      }
    );

    expect(formResponse.ok()).toBeTruthy();
    const formConfig = await formResponse.json();

    const card = VISA_APPROVAL;
    const expDate = formatExpDate(card);

    // Credit card form includes AVS fields (ADDRESS, CITY, STATE, ZIP_CODE)
    const formHtml = `
      <!DOCTYPE html>
      <html>
      <body>
        <form id="epxForm" method="POST" action="${formConfig.postURL}">
          <input type="hidden" name="TAC" value="${formConfig.tac}" />
          <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}" />
          <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}" />
          <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}" />
          <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}" />
          <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}" />
          <input type="hidden" name="AMOUNT" value="${authAmount}" />
          <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}" />
          <input type="hidden" name="ACCOUNT_NBR" value="${card.number}" />
          <input type="hidden" name="EXP_DATE" value="${expDate}" />
          <input type="hidden" name="CVV2" value="${card.cvv}" />
          <input type="hidden" name="FIRST_NAME" value="Jane" />
          <input type="hidden" name="LAST_NAME" value="Smith" />
          <input type="hidden" name="ADDRESS" value="456 Oak Ave" />
          <input type="hidden" name="CITY" value="Los Angeles" />
          <input type="hidden" name="STATE" value="CA" />
          <input type="hidden" name="ZIP_CODE" value="${card.zip}" />
        </form>
        <script>document.getElementById('epxForm').submit();</script>
      </body>
      </html>
    `;

    await page.setContent(formHtml);
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    // Verify AUTH (poll until approved)
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
      (txn) => txn?.status === 'TRANSACTION_STATUS_APPROVED',
      quickRetryConfig()
    );
    console.log('AUTH transaction created:', transaction.id);

    // Verify AUTH was APPROVED
    expect(transaction.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(transaction.authorizationCode).toBeTruthy();
    expect(transaction.amountCents).toBe('10000'); // $100.00

    console.log(`AUTH approved with auth code: ${transaction.authorizationCode}`);

    // Partial capture for $50
    console.log('Capturing partial amount ($50)...');
    const captureResponse = await request.post(
      `${baseUrl}/payment.v1.PaymentService/Capture`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
          'Content-Type': 'application/json',
        },
        data: {
          transaction_id: transactionId,
          amount_cents: captureAmountCents,
          idempotency_key: crypto.randomUUID(),
        },
      }
    );

    expect(captureResponse.ok(), `Partial Capture failed: ${await captureResponse.text()}`).toBeTruthy();

    const captureResult = await captureResponse.json();
    console.log('Partial CAPTURE result:', JSON.stringify(captureResult, null, 2));

    // Verify partial CAPTURE was APPROVED
    expect(captureResult.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(captureResult.isApproved).toBe(true);
    expect(captureResult.amountCents).toBe('5000'); // $50.00 partial capture

    console.log('Partial CAPTURE completed successfully!');
  });
});
