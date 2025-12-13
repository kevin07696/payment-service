/**
 * Shared type definitions for E2E tests
 */

// Test merchant with EPX sandbox credentials
export interface TestMerchant {
  id: string;
  slug: string;
  name: string;
  custNbr: string;
  merchNbr: string;
  dbaNbr: string;
  terminalNbr: string;
}

// Test service with RSA keypair
export interface TestService {
  id: string;
  serviceId: string;
  privateKey: string;
  publicKey: string;
}

// Complete test context
export interface TestContext {
  merchant: TestMerchant;
  service: TestService;
  token: string;
}
