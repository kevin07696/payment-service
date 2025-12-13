import { test, expect } from '../lib/test-fixtures';
import { VISA_APPROVAL, formatExpDate } from '../lib/test-cards';
import { retryUntilCondition, quickRetryConfig } from '../lib/retry';

/**
 * Browser Post STORAGE Flow E2E Test
 *
 * Tests the complete browser post card storage flow:
 * 1. Get form configuration from backend (STORAGE type)
 * 2. Submit card data to EPX via browser
 * 3. EPX redirects back to callback with BRIC token
 * 4. Verify payment method is saved to database
 *
 * Business Logic:
 * - Tokenize card without charging (save for future use)
 * - No money moves - just stores card securely
 * - Returns BRIC token for future transactions
 */
test.describe('Browser Post STORAGE Flow', () => {
  test('store credit card and verify payment method created', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

    // Step 1: Get browser post form configuration for STORAGE
    const transactionId = crypto.randomUUID();
    const customerId = `customer-${Date.now()}`;
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount: '0.00', // STORAGE doesn't charge
      transaction_type: 'STORAGE',
      customer_id: customerId,
      return_url: callbackUrl,
    });

    console.log(`Getting browser post form for STORAGE, merchant: ${testContext.merchant.id}`);

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
    expect(formConfig.epxTranNbr).toBeTruthy();
    expect(formConfig.tranCode).toBe('STORAGE');

    console.log(`Got TAC for STORAGE, posting to EPX: ${formConfig.postURL}`);

    // Step 2: Build and submit form to EPX
    const card = VISA_APPROVAL;
    const expDate = formatExpDate(card);

    // Create an HTML page with auto-submitting form
    const formHtml = `
      <!DOCTYPE html>
      <html>
      <head><title>EPX Card Storage</title></head>
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

    // Navigate to a data URL with the form
    await page.setContent(formHtml);

    // Wait for navigation to EPX and back to callback
    console.log('Waiting for EPX redirect...');
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const finalUrl = page.url();
    console.log(`Final URL: ${finalUrl}`);

    // Step 3: Verify callback received
    expect(finalUrl).toContain('callback');

    // Step 4: Verify payment method was created via API (poll until exists)
    console.log(`Verifying payment method for customer: ${customerId}`);

    const pmData = await retryUntilCondition(
      async () => {
        const resp = await request.post(
          `${baseUrl}/payment_method.v1.PaymentMethodService/ListPaymentMethods`,
          {
            headers: {
              Authorization: `Bearer ${testContext.token}`,
              'Content-Type': 'application/json',
              'Accept-Encoding': 'identity',
            },
            data: {
              merchant_id: testContext.merchant.id,
              customer_id: customerId,
            },
          }
        );
        if (!resp.ok()) return null;
        return await resp.json();
      },
      (data) => (data?.paymentMethods?.length ?? 0) > 0,
      quickRetryConfig()
    );
    console.log('Payment methods response:', JSON.stringify(pmData, null, 2));

    // Verify payment method was created
    const paymentMethods = pmData.paymentMethods || [];
    expect(paymentMethods.length).toBeGreaterThan(0);

    const pm = paymentMethods[0];
    console.log(`Payment method created: ${pm.id}`);

    // Verify it's a credit card with correct details
    expect(pm.paymentType).toBe('PAYMENT_METHOD_TYPE_CREDIT_CARD');
    expect(pm.customerId).toBe(customerId);
    expect(pm.merchantId).toBe(testContext.merchant.id);
    expect(pm.lastFour).toBe('0002');
    expect(pm.cardBrand).toBeTruthy();

    // Verify card is active and verified (STORAGE was approved by EPX)
    expect(pm.isActive).toBe(true);
    expect(pm.isVerified).toBe(true);

    console.log(`STORAGE approved: ${pm.cardBrand} ending in ${pm.lastFour}`);
  });

  test('store card and use for subsequent payment', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

    // Step 1: Store a card first
    const storageTransactionId = crypto.randomUUID();
    const customerId = `customer-${Date.now()}`;
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const storageParams = new URLSearchParams({
      transaction_id: storageTransactionId,
      merchant_id: testContext.merchant.id,
      amount: '0.00',
      transaction_type: 'STORAGE',
      customer_id: customerId,
      return_url: callbackUrl,
    });

    console.log('Step 1: Storing card via Browser POST STORAGE');

    const formResponse = await request.get(
      `${baseUrl}/api/v1/payments/browser-post/form?${storageParams}`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
        },
      }
    );

    expect(formResponse.ok(), `Form request failed: ${await formResponse.text()}`).toBeTruthy();

    const formConfig = await formResponse.json();
    const card = VISA_APPROVAL;
    const expDate = formatExpDate(card);

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
          <input type="hidden" name="AMOUNT" value="0.00" />
          <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}" />
          <input type="hidden" name="ACCOUNT_NBR" value="${card.number}" />
          <input type="hidden" name="EXP_DATE" value="${expDate}" />
          <input type="hidden" name="CVV2" value="${card.cvv}" />
          <input type="hidden" name="FIRST_NAME" value="Bob" />
          <input type="hidden" name="LAST_NAME" value="Johnson" />
          <input type="hidden" name="ADDRESS" value="789 Pine St" />
          <input type="hidden" name="CITY" value="Chicago" />
          <input type="hidden" name="STATE" value="IL" />
          <input type="hidden" name="ZIP_CODE" value="${card.zip}" />
        </form>
        <script>document.getElementById('epxForm').submit();</script>
      </body>
      </html>
    `;

    await page.setContent(formHtml);
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    // Step 2: Get the stored payment method (poll until exists)
    console.log('Step 2: Retrieving stored payment method');

    const pmData = await retryUntilCondition(
      async () => {
        const resp = await request.post(
          `${baseUrl}/payment_method.v1.PaymentMethodService/ListPaymentMethods`,
          {
            headers: {
              Authorization: `Bearer ${testContext.token}`,
              'Content-Type': 'application/json',
              'Accept-Encoding': 'identity',
            },
            data: {
              merchant_id: testContext.merchant.id,
              customer_id: customerId,
            },
          }
        );
        if (!resp.ok()) return null;
        return await resp.json();
      },
      (data) => (data?.paymentMethods?.length ?? 0) > 0,
      quickRetryConfig()
    );
    const paymentMethods = pmData.paymentMethods || [];
    expect(paymentMethods.length).toBeGreaterThan(0);

    const storedPaymentMethod = paymentMethods[0];
    console.log(`Found stored payment method: ${storedPaymentMethod.id}`);

    // Verify STORAGE was approved (card is active)
    expect(storedPaymentMethod.isActive).toBe(true);

    // Step 3: Use stored payment method for a SALE via Server POST
    console.log('Step 3: Using stored payment method for a payment');

    const saleTransactionId = crypto.randomUUID();
    const saleResponse = await request.post(
      `${baseUrl}/payment.v1.PaymentService/Sale`,
      {
        headers: {
          Authorization: `Bearer ${testContext.token}`,
          'Content-Type': 'application/json',
        },
        data: {
          idempotency_key: saleTransactionId,
          merchant_id: testContext.merchant.id,
          customer_id: customerId,
          payment_method_id: storedPaymentMethod.id,
          amount_cents: 2500, // $25.00
          currency: 'USD',
        },
      }
    );

    expect(saleResponse.ok(), `Sale failed: ${await saleResponse.text()}`).toBeTruthy();

    const saleData = await saleResponse.json();
    console.log('Sale response:', JSON.stringify(saleData, null, 2));

    // Verify SALE was APPROVED
    expect(saleData.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(saleData.isApproved).toBe(true);
    expect(saleData.authorizationCode).toBeTruthy();
    expect(saleData.amountCents).toBe('2500'); // $25.00

    console.log(`SALE approved with auth code: ${saleData.authorizationCode}`);
  });
});
