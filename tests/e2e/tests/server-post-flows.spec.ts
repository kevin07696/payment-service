/**
 * Server Post Flow E2E Tests
 *
 * These tests verify Server Post (direct API) payment flows using real EPX sandbox.
 *
 * PREREQUISITE: Server Post requires a BRIC (tokenized card reference) which is
 * obtained via Browser POST Storage flow. Each test first tokenizes a card,
 * then uses that BRIC for Server Post transactions.
 *
 * Test Coverage:
 * - BRIC Storage -> Sale -> Refund flow
 * - Auth -> Capture flow (full and partial)
 * - Auth -> Void flow
 * - EPX amount-based response triggers
 *
 * EPX Response Code Triggers (per EPX certification docs):
 * - $1.00 = Approval (00)
 * - $1.05 = Decline (05)
 * - $1.14 = Invalid card (14)
 * - $1.51 = NSF (51)
 */

import { test, expect } from '../lib/test-fixtures';
import { VISA_APPROVAL } from '../lib/test-cards';
import {
  getBRICviaBrowserPost,
  serverPostSale,
  serverPostAuth,
  serverPostCapture,
  serverPostRefund,
  serverPostVoid,
  EPXAmountTriggers,
} from '../lib/server-post-helpers';

test.describe('Server Post Flows', () => {
  test.describe('BRIC Storage and Sale Flow', () => {
    test('complete flow: Storage -> Sale -> Refund', async ({ page, request, testContext }) => {
      console.log('═══════════════════════════════════════════════════════════════');
      console.log('        SERVER POST: STORAGE → SALE → REFUND FLOW              ');
      console.log('═══════════════════════════════════════════════════════════════');
      console.log(`Merchant ID: ${testContext.merchant.id}`);
      console.log('');

      // Step 1: Get BRIC via Browser POST Storage
      console.log('STEP 1: BRIC Storage (Browser POST)');
      console.log('────────────────────────────────────────────────────────────────');
      console.log('  → Tokenizing card via Browser POST STORAGE flow');

      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);

      console.log(`  ✓ BRIC obtained: ${bric.substring(0, 20)}...`);
      expect(bric).toBeTruthy();
      console.log('');

      // Step 2: Server Post Sale using BRIC
      console.log('STEP 2: Server Post Sale (CCE1)');
      console.log('────────────────────────────────────────────────────────────────');
      console.log(`  → Creating sale with BRIC for $${EPXAmountTriggers.APPROVAL}`);

      const saleResponse = await serverPostSale(request, testContext, bric, EPXAmountTriggers.APPROVAL);

      console.log(`  ✓ Sale response received`);
      console.log(`    - Transaction ID: ${saleResponse.transactionId}`);
      console.log(`    - Status: ${saleResponse.status}`);
      console.log(`    - Auth Code: ${saleResponse.authorizationCode || 'N/A'}`);
      console.log(`    - Processor Code: ${saleResponse.processorResponseCode || 'N/A'}`);

      expect(saleResponse.status).toBe('TRANSACTION_STATUS_APPROVED');
      expect(saleResponse.authorizationCode).toBeTruthy();
      console.log('');

      // Step 3: Server Post Refund
      console.log('STEP 3: Server Post Refund (CCE9)');
      console.log('────────────────────────────────────────────────────────────────');
      console.log(`  → Refunding transaction ${saleResponse.transactionId}`);

      const refundResponse = await serverPostRefund(request, testContext, saleResponse.transactionId);

      console.log(`  ✓ Refund response received`);
      console.log(`    - Refund Transaction ID: ${refundResponse.transactionId}`);
      console.log(`    - Status: ${refundResponse.status}`);
      console.log(`    - Processor Code: ${refundResponse.processorResponseCode || 'N/A'}`);

      expect(refundResponse.status).toMatch(/APPROVED|REFUNDED|PENDING/);
      console.log('');

      console.log('═══════════════════════════════════════════════════════════════');
      console.log('                    TEST PASSED                                 ');
      console.log('═══════════════════════════════════════════════════════════════');
    });

    test('partial refund flow', async ({ page, request, testContext }) => {
      console.log('Testing partial refund flow...');

      // Get BRIC
      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      console.log(`BRIC obtained: ${bric.substring(0, 20)}...`);

      // Sale for $10.00
      const saleResponse = await serverPostSale(request, testContext, bric, '10.00');
      expect(saleResponse.status).toBe('TRANSACTION_STATUS_APPROVED');
      console.log(`Sale approved: ${saleResponse.transactionId}`);

      // Partial refund for $3.00
      const refundResponse = await serverPostRefund(request, testContext, saleResponse.transactionId, '3.00');
      expect(refundResponse.status).toMatch(/APPROVED|REFUNDED|PENDING/);
      console.log(`Partial refund: ${refundResponse.transactionId} ($3.00 of $10.00)`);

      console.log('Partial refund flow completed successfully!');
    });
  });

  test.describe('Auth and Capture Flow', () => {
    test('complete flow: Auth -> Full Capture', async ({ page, request, testContext }) => {
      console.log('═══════════════════════════════════════════════════════════════');
      console.log('          SERVER POST: AUTH → CAPTURE FLOW                      ');
      console.log('═══════════════════════════════════════════════════════════════');

      // Get BRIC
      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      console.log(`BRIC obtained: ${bric.substring(0, 20)}...`);

      // Auth for $50.00
      console.log('STEP 1: Authorization (CCE2)');
      const authResponse = await serverPostAuth(request, testContext, bric, '50.00');

      console.log(`  - Transaction ID: ${authResponse.transactionId}`);
      console.log(`  - Status: ${authResponse.status}`);
      console.log(`  - Auth Code: ${authResponse.authorizationCode || 'N/A'}`);

      expect(authResponse.status).toBe('TRANSACTION_STATUS_AUTHORIZED');
      expect(authResponse.authorizationCode).toBeTruthy();

      // Capture full amount
      console.log('');
      console.log('STEP 2: Full Capture (CCE4)');
      const captureResponse = await serverPostCapture(request, testContext, authResponse.transactionId);

      console.log(`  - Status: ${captureResponse.status}`);
      console.log(`  - Amount: ${captureResponse.amountCents} cents`);

      expect(captureResponse.status).toBe('TRANSACTION_STATUS_APPROVED');

      console.log('');
      console.log('Auth → Full Capture flow completed successfully!');
    });

    test('partial capture flow', async ({ page, request, testContext }) => {
      console.log('Testing partial capture flow...');

      // Get BRIC
      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);

      // Auth for $100.00
      const authResponse = await serverPostAuth(request, testContext, bric, '100.00');
      expect(authResponse.status).toBe('TRANSACTION_STATUS_AUTHORIZED');
      console.log(`Auth approved for $100.00: ${authResponse.transactionId}`);

      // Capture only $60.00
      const captureResponse = await serverPostCapture(request, testContext, authResponse.transactionId, '60.00');
      expect(captureResponse.status).toBe('TRANSACTION_STATUS_APPROVED');
      console.log(`Partial capture for $60.00: ${captureResponse.amountCents} cents captured`);

      console.log('Partial capture flow completed successfully!');
    });
  });

  test.describe('Auth and Void Flow', () => {
    test('complete flow: Auth -> Void', async ({ page, request, testContext }) => {
      console.log('═══════════════════════════════════════════════════════════════');
      console.log('           SERVER POST: AUTH → VOID FLOW                        ');
      console.log('═══════════════════════════════════════════════════════════════');

      // Get BRIC
      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      console.log(`BRIC obtained: ${bric.substring(0, 20)}...`);

      // Auth for $25.00
      console.log('STEP 1: Authorization (CCE2)');
      const authResponse = await serverPostAuth(request, testContext, bric, '25.00');

      expect(authResponse.status).toBe('TRANSACTION_STATUS_AUTHORIZED');
      console.log(`  Auth approved: ${authResponse.transactionId}`);

      // Void the auth
      console.log('');
      console.log('STEP 2: Void (CCE7)');
      const voidResponse = await serverPostVoid(request, testContext, authResponse.transactionId);

      console.log(`  - Status: ${voidResponse.status}`);

      expect(voidResponse.status).toMatch(/VOIDED|REVERSED|APPROVED/);

      console.log('');
      console.log('Auth → Void flow completed successfully!');
    });
  });

  test.describe('EPX Amount-Based Response Triggers', () => {
    test('$1.00 triggers approval (00)', async ({ page, request, testContext }) => {
      console.log('Testing $1.00 → Approval (00)...');

      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      const response = await serverPostSale(request, testContext, bric, EPXAmountTriggers.APPROVAL);

      console.log(`Response: ${response.status} (processor: ${response.processorResponseCode})`);

      expect(response.status).toBe('TRANSACTION_STATUS_APPROVED');
      expect(response.processorResponseCode).toBe('00');
    });

    test('$1.05 triggers decline (05)', async ({ page, request, testContext }) => {
      console.log('Testing $1.05 → Decline (05)...');

      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      const response = await serverPostSale(request, testContext, bric, EPXAmountTriggers.DECLINE_05);

      console.log(`Response: ${response.status} (processor: ${response.processorResponseCode})`);

      expect(response.status).toBe('TRANSACTION_STATUS_DECLINED');
      expect(response.processorResponseCode).toBe('05');
    });

    test('$1.51 triggers NSF (51)', async ({ page, request, testContext }) => {
      console.log('Testing $1.51 → NSF (51)...');

      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      const response = await serverPostSale(request, testContext, bric, EPXAmountTriggers.NSF);

      console.log(`Response: ${response.status} (processor: ${response.processorResponseCode})`);

      expect(response.status).toBe('TRANSACTION_STATUS_DECLINED');
      expect(response.processorResponseCode).toBe('51');
    });
  });

  test.describe('BRIC Reuse', () => {
    test('same BRIC can be used for multiple transactions', async ({ page, request, testContext }) => {
      console.log('Testing BRIC reuse across multiple transactions...');

      // Get one BRIC
      const bric = await getBRICviaBrowserPost(page, request, testContext, VISA_APPROVAL);
      console.log(`BRIC obtained: ${bric.substring(0, 20)}...`);

      // Use it for three separate sales
      const sale1 = await serverPostSale(request, testContext, bric, '1.00');
      expect(sale1.status).toBe('TRANSACTION_STATUS_APPROVED');
      console.log(`Sale 1 approved: ${sale1.transactionId}`);

      const sale2 = await serverPostSale(request, testContext, bric, '2.00');
      expect(sale2.status).toBe('TRANSACTION_STATUS_APPROVED');
      console.log(`Sale 2 approved: ${sale2.transactionId}`);

      const sale3 = await serverPostSale(request, testContext, bric, '3.00');
      expect(sale3.status).toBe('TRANSACTION_STATUS_APPROVED');
      console.log(`Sale 3 approved: ${sale3.transactionId}`);

      // Verify all three are unique transactions
      expect(sale1.transactionId).not.toBe(sale2.transactionId);
      expect(sale2.transactionId).not.toBe(sale3.transactionId);

      console.log('BRIC reuse test passed - same token used for 3 transactions!');
    });
  });
});
