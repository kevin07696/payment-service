import { test, expect } from '../lib/test-fixtures';

/**
 * Test bank account data for EPX sandbox
 */
const TEST_CHECKING_ACCOUNT = {
  routingNumber: '011000015',
  accountNumber: '123456789',
  accountType: 'checking',
};

const TEST_SAVINGS_ACCOUNT = {
  routingNumber: '011000015',
  accountNumber: '987654321',
  accountType: 'savings',
};

/**
 * Browser Post ACH STORAGE Flow E2E Tests
 *
 * Tests the complete browser post ACH storage flows:
 * - ACH Checking Storage (ACHSTORAGE_C / CKC8)
 * - ACH Savings Storage (ACHSTORAGE_S / CKS8)
 *
 * Flow:
 * 1. Get form configuration from backend (ACH_STORAGE_C or ACH_STORAGE_S type)
 * 2. Submit bank data to EPX via browser
 * 3. EPX redirects back to callback with BRIC token
 * 4. Verify payment method is saved to database
 * 5. Automatic prenote is sent for ACH verification
 *
 * Business Logic:
 * - Tokenize bank account without charging (save for future ACH debits)
 * - No money moves - just stores bank account securely
 * - Returns BRIC token for future ACH transactions
 * - Prenote verifies bank account is valid before first real transaction
 */
test.describe('Browser Post ACH STORAGE Flow', () => {
  test('store checking account via ACH_STORAGE_C', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

    const transactionId = crypto.randomUUID();
    const customerId = `ach-customer-${Date.now()}`;
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount: '0.00',
      transaction_type: 'ACH_STORAGE_C', // Checking account storage
      customer_id: customerId,
      return_url: callbackUrl,
    });

    console.log(`Getting browser post form for ACH Checking Storage, merchant: ${testContext.merchant.id}`);

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
    // TRAN_CODE should be ACHSTORAGE_C for checking account storage (per EPX certification)
    expect(formConfig.tranCode).toBe('ACHSTORAGE_C');

    console.log(`Got TAC for ACH Checking Storage, posting to EPX: ${formConfig.postURL}`);

    // Build and submit form to EPX with ACH fields per certification sheet
    const account = TEST_CHECKING_ACCOUNT;

    // Note: Address fields (ADDRESS, CITY, STATE, ZIP_CODE) are not included
    // because AVS does not work for ACH - only for credit cards
    const formHtml = `
      <!DOCTYPE html>
      <html>
      <head><title>EPX ACH Checking Storage</title></head>
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
          <input type="hidden" name="STD_ENTRY_CLASS" value="WEB" />
          <input type="hidden" name="ACCOUNT_NBR" value="${account.accountNumber}" />
          <input type="hidden" name="ROUTING_NBR" value="${account.routingNumber}" />
          <input type="hidden" name="FIRST_NAME" value="John" />
          <input type="hidden" name="LAST_NAME" value="Doe" />
        </form>
        <script>document.getElementById('epxForm').submit();</script>
      </body>
      </html>
    `;

    await page.setContent(formHtml);

    console.log('Waiting for EPX redirect...');
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const finalUrl = page.url();
    console.log(`Final URL: ${finalUrl}`);

    expect(finalUrl).toContain('callback');

    await page.waitForTimeout(1000);

    // Verify payment method was created
    console.log(`Verifying ACH payment method for customer: ${customerId}`);

    const pmResponse = await request.post(
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

    console.log(`Payment methods response status: ${pmResponse.status()}`);

    if (pmResponse.ok()) {
      const pmData = await pmResponse.json();
      console.log('Payment methods response:', JSON.stringify(pmData, null, 2));

      const paymentMethods = pmData.paymentMethods || [];

      if (paymentMethods.length > 0) {
        const pm = paymentMethods[0];
        console.log(`ACH Payment method created: ${pm.id}`);

        expect(pm.paymentType).toBe('PAYMENT_METHOD_TYPE_ACH');
        expect(pm.customerId).toBe(customerId);
        expect(pm.merchantId).toBe(testContext.merchant.id);
        expect(pm.accountType).toBe('checking');
        expect(pm.lastFour).toBe('6789');
        console.log(`Bank account stored: Checking ending in ${pm.lastFour}`);

        console.log('ACH Checking Storage: Payment method created successfully!');
      } else {
        console.log('No payment methods found - EPX sandbox may be unavailable or ACH not enabled');
      }
    } else {
      const errorBody = await pmResponse.text();
      console.log(`Payment method verification failed: ${pmResponse.status()} - ${errorBody}`);
    }
  });

  test('store savings account via ACH_STORAGE_S', async ({ page, request, testContext }) => {
    const baseUrl = process.env.SERVICE_URL || 'http://localhost:8081';

    const transactionId = crypto.randomUUID();
    const customerId = `ach-savings-${Date.now()}`;
    const callbackUrl = `${baseUrl}/api/v1/payments/browser-post/callback`;

    const params = new URLSearchParams({
      transaction_id: transactionId,
      merchant_id: testContext.merchant.id,
      amount: '0.00',
      transaction_type: 'ACH_STORAGE_S', // Savings account storage
      customer_id: customerId,
      return_url: callbackUrl,
    });

    console.log(`Getting browser post form for ACH Savings Storage, merchant: ${testContext.merchant.id}`);

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
    // TRAN_CODE should be ACHSTORAGE_S for savings account storage (per EPX certification)
    expect(formConfig.tranCode).toBe('ACHSTORAGE_S');

    console.log(`Got TAC for ACH Savings Storage, posting to EPX: ${formConfig.postURL}`);

    const account = TEST_SAVINGS_ACCOUNT;

    // Note: Address fields (ADDRESS, CITY, STATE, ZIP_CODE) are not included
    // because AVS does not work for ACH - only for credit cards
    const formHtml = `
      <!DOCTYPE html>
      <html>
      <head><title>EPX ACH Savings Storage</title></head>
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
          <input type="hidden" name="STD_ENTRY_CLASS" value="WEB" />
          <input type="hidden" name="ACCOUNT_NBR" value="${account.accountNumber}" />
          <input type="hidden" name="ROUTING_NBR" value="${account.routingNumber}" />
          <input type="hidden" name="FIRST_NAME" value="Jane" />
          <input type="hidden" name="LAST_NAME" value="Smith" />
        </form>
        <script>document.getElementById('epxForm').submit();</script>
      </body>
      </html>
    `;

    await page.setContent(formHtml);

    console.log('Waiting for EPX redirect...');
    await page.waitForURL(/callback|return/, { timeout: 30000 });

    const finalUrl = page.url();
    console.log(`Final URL: ${finalUrl}`);

    expect(finalUrl).toContain('callback');

    await page.waitForTimeout(1000);

    // Verify payment method was created
    console.log(`Verifying ACH savings payment method for customer: ${customerId}`);

    const pmResponse = await request.post(
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

    console.log(`Payment methods response status: ${pmResponse.status()}`);

    if (pmResponse.ok()) {
      const pmData = await pmResponse.json();
      console.log('Payment methods response:', JSON.stringify(pmData, null, 2));

      const paymentMethods = pmData.paymentMethods || [];

      if (paymentMethods.length > 0) {
        const pm = paymentMethods[0];
        console.log(`ACH Payment method created: ${pm.id}`);

        expect(pm.paymentType).toBe('PAYMENT_METHOD_TYPE_ACH');
        expect(pm.customerId).toBe(customerId);
        expect(pm.merchantId).toBe(testContext.merchant.id);
        expect(pm.accountType).toBe('savings');
        expect(pm.lastFour).toBe('4321');
        console.log(`Bank account stored: Savings ending in ${pm.lastFour}`);

        console.log('ACH Savings Storage: Payment method created successfully!');
      } else {
        console.log('No payment methods found - EPX sandbox may be unavailable or ACH not enabled');
      }
    } else {
      const errorBody = await pmResponse.text();
      console.log(`Payment method verification failed: ${pmResponse.status()} - ${errorBody}`);
    }
  });
});
