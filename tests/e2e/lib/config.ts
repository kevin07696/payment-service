/**
 * E2E Test Configuration
 *
 * Centralized configuration for e2e tests.
 * Environment variables are validated when getConfig() is called,
 * NOT at module load time - so this can be safely imported anywhere.
 */

export interface E2EConfig {
  serviceURL: string;
  epx: {
    custNbr: string;
    merchNbr: string;
    dbaNbr: string;
    terminalNbr: string;
    macSecret: string; // Actual MAC secret value for sandbox
  };
}

let cachedConfig: E2EConfig | null = null;

/**
 * Get validated E2E test configuration.
 * Call this at test runtime, not at module load time.
 * Throws with clear error messages if required env vars are missing.
 */
export function getConfig(): E2EConfig {
  // Return cached config if already validated
  if (cachedConfig) {
    return cachedConfig;
  }

  const missing: string[] = [];

  // Required environment variables
  const serviceURL = process.env.SERVICE_URL;
  const epxCustNbr = process.env.EPX_CUST_NBR;
  const epxMerchNbr = process.env.EPX_MERCH_NBR;
  const epxDbaNbr = process.env.EPX_DBA_NBR;
  const epxTerminalNbr = process.env.EPX_TERMINAL_NBR;
  const epxMacSecret = process.env.EPX_SANDBOX_MAC;

  if (!serviceURL) missing.push('SERVICE_URL');
  if (!epxCustNbr) missing.push('EPX_CUST_NBR');
  if (!epxMerchNbr) missing.push('EPX_MERCH_NBR');
  if (!epxDbaNbr) missing.push('EPX_DBA_NBR');
  if (!epxTerminalNbr) missing.push('EPX_TERMINAL_NBR');
  if (!epxMacSecret) missing.push('EPX_SANDBOX_MAC');

  if (missing.length > 0) {
    throw new Error(
      `Missing required environment variables for E2E tests: ${missing.join(', ')}\n` +
      `\nSet these before running tests:\n` +
      `  export SERVICE_URL="http://localhost:8081"\n` +
      `  export EPX_CUST_NBR="9001"\n` +
      `  export EPX_MERCH_NBR="900300"\n` +
      `  export EPX_DBA_NBR="2"\n` +
      `  export EPX_TERMINAL_NBR="77"\n` +
      `  export EPX_SANDBOX_MAC="your-sandbox-mac-secret"`
    );
  }

  cachedConfig = {
    serviceURL: serviceURL!,
    epx: {
      custNbr: epxCustNbr!,
      merchNbr: epxMerchNbr!,
      dbaNbr: epxDbaNbr!,
      terminalNbr: epxTerminalNbr!,
      macSecret: epxMacSecret!,
    },
  };

  return cachedConfig;
}

