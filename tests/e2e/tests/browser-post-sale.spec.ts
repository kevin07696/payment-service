import { test, expect } from '../lib/test-fixtures';
import { VISA_APPROVAL, formatExpDate } from '../lib/test-cards';

/**
 * Browser Post SALE Flow E2E Test
 *
 * Tests the complete browser post payment flow:
 * 1. Get form configuration from backend
 * 2. Submit card data to EPX via browser
 * 3. EPX redirects back to callback
 * 4. Verify transaction is created and approved
 */
test.describe('Browser Post SALE Flow', () => {
  test('complete SALE transaction with test card', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

    // Step 1: Get browser post form configuration
    const transactionId = crypto.randomUUID();
    const amount = '50.00';
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount,
      transaction_type: 'SALE',
      return_url: callbackUrl,
    });

    console.log(`Getting browser post form for merchant: ${testContext.merchant.id}`);

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

    console.log(`Got TAC, posting to EPX: ${formConfig.postURL}`);

    // Step 2: Build and submit form to EPX
    const card = VISA_APPROVAL;
    const expDate = formatExpDate(card);

    // Create an HTML page with auto-submitting form
    // Form fields per EPX certification sheet
    const formHtml = `
      <!DOCTYPE html>
      <html>
      <head><title>EPX Payment</title></head>
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

    // Navigate to a data URL with the form
    await page.setContent(formHtml);

    // Wait for navigation to EPX and back to callback
    console.log('Waiting for EPX redirect...');
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const finalUrl = page.url();
    console.log(`Final URL: ${finalUrl}`);

    // Step 3: Verify callback received
    expect(finalUrl).toContain('callback');

    // Extract transaction ID from URL or page content
    const urlParams = new URL(finalUrl).searchParams;
    const authCode = urlParams.get('authorization_code') || urlParams.get('AUTH_GUID');

    if (authCode) {
      console.log(`Transaction approved with auth code: ${authCode}`);
      expect(authCode).toBeTruthy();
    }

    // Step 4: Verify transaction via API
    console.log(`Verifying transaction: ${transactionId}`);

    // Wait a moment for callback processing
    await page.waitForTimeout(1000);

    const txnResponse = await request.post(
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

    expect(txnResponse.ok(), `GetTransaction failed: ${await txnResponse.text()}`).toBeTruthy();

    const transaction = await txnResponse.json();
    console.log('Transaction response:', JSON.stringify(transaction, null, 2));

    // Verify transaction was created with correct type
    expect(transaction.id).toBe(transactionId);
    expect(transaction.merchantId).toBe(testContext.merchant.id);
    expect(transaction.type).toBe('TRANSACTION_TYPE_CHARGE');
    expect(transaction.amountCents).toBe('5000'); // $50.00 = 5000 cents

    // Verify transaction was APPROVED by EPX
    expect(transaction.status).toBe('TRANSACTION_STATUS_APPROVED');
    expect(transaction.authorizationCode).toBeTruthy();

    console.log(`SALE approved with auth code: ${transaction.authorizationCode}`);
  });
});
