/**
 * Retry utilities for E2E tests
 *
 * Provides exponential backoff retry logic for operations that may take time
 * to propagate or may be temporarily unavailable.
 */

export interface RetryConfig {
  maxAttempts: number;
  initialDelayMs: number;
  maxDelayMs: number;
  backoffMultiplier: number;
}

/**
 * Default retry configuration for standard operations
 * 10 attempts, 100ms-5s delay range
 */
export function defaultRetryConfig(): RetryConfig {
  return {
    maxAttempts: 10,
    initialDelayMs: 100,
    maxDelayMs: 5000,
    backoffMultiplier: 2,
  };
}

/**
 * Quick retry configuration for fast operations
 * 5 attempts, 50ms-500ms delay range
 */
export function quickRetryConfig(): RetryConfig {
  return {
    maxAttempts: 5,
    initialDelayMs: 50,
    maxDelayMs: 500,
    backoffMultiplier: 2,
  };
}

/**
 * Slow retry configuration for external APIs or slow operations
 * 15 attempts, 500ms-10s delay range
 */
export function slowRetryConfig(): RetryConfig {
  return {
    maxAttempts: 15,
    initialDelayMs: 500,
    maxDelayMs: 10000,
    backoffMultiplier: 1.5,
  };
}

/**
 * Retry a function until it returns a truthy result or max attempts reached.
 *
 * @param fn - Async function to retry
 * @param config - Retry configuration
 * @returns The result of the function when it succeeds
 * @throws Error if max attempts exceeded
 *
 * @example
 * ```ts
 * const transaction = await retryUntil(
 *   async () => {
 *     const result = await api.getTransaction(id);
 *     return result.status === 'APPROVED' ? result : null;
 *   },
 *   defaultRetryConfig()
 * );
 * ```
 */
export async function retryUntil<T>(
  fn: () => Promise<T | null | undefined>,
  config: RetryConfig = defaultRetryConfig()
): Promise<T> {
  let delay = config.initialDelayMs;
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= config.maxAttempts; attempt++) {
    try {
      const result = await fn();
      if (result) {
        return result;
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }

    if (attempt < config.maxAttempts) {
      await sleep(delay);
      delay = Math.min(delay * config.backoffMultiplier, config.maxDelayMs);
    }
  }

  throw lastError || new Error(`Retry exhausted after ${config.maxAttempts} attempts`);
}

/**
 * Retry a function until a condition is met.
 *
 * @param fn - Async function to retry
 * @param condition - Condition function that returns true when done
 * @param config - Retry configuration
 * @returns The result of the function when condition is met
 *
 * @example
 * ```ts
 * const transaction = await retryUntilCondition(
 *   () => api.getTransaction(id),
 *   (result) => result.status === 'APPROVED',
 *   defaultRetryConfig()
 * );
 * ```
 */
export async function retryUntilCondition<T>(
  fn: () => Promise<T>,
  condition: (result: T) => boolean,
  config: RetryConfig = defaultRetryConfig()
): Promise<T> {
  let delay = config.initialDelayMs;
  let lastResult: T | undefined;
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= config.maxAttempts; attempt++) {
    try {
      lastResult = await fn();
      if (condition(lastResult)) {
        return lastResult;
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }

    if (attempt < config.maxAttempts) {
      await sleep(delay);
      delay = Math.min(delay * config.backoffMultiplier, config.maxDelayMs);
    }
  }

  if (lastError) {
    throw lastError;
  }
  if (lastResult !== undefined) {
    return lastResult; // Return last result even if condition wasn't met
  }
  throw new Error(`Retry exhausted after ${config.maxAttempts} attempts`);
}

/**
 * Wait for a condition to become true within a timeout.
 *
 * @param condition - Async condition function
 * @param timeoutMs - Maximum time to wait
 * @param intervalMs - Check interval
 * @returns true if condition was met, false if timeout
 *
 * @example
 * ```ts
 * const ready = await waitForCondition(
 *   async () => {
 *     const resp = await fetch(healthUrl);
 *     return resp.ok;
 *   },
 *   30000, // 30 second timeout
 *   1000   // check every second
 * );
 * ```
 */
export async function waitForCondition(
  condition: () => Promise<boolean>,
  timeoutMs: number,
  intervalMs: number = 1000
): Promise<boolean> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    try {
      if (await condition()) {
        return true;
      }
    } catch {
      // Ignore errors and keep trying
    }
    await sleep(intervalMs);
  }

  return false;
}

/**
 * Wait for a server health endpoint to respond.
 *
 * @param healthUrl - URL to the health endpoint
 * @param timeoutMs - Maximum time to wait (default: 30s)
 * @returns true if server is ready, false if timeout
 *
 * @example
 * ```ts
 * const ready = await waitForServer('http://localhost:8081/cron/health');
 * if (!ready) {
 *   throw new Error('Server did not start in time');
 * }
 * ```
 */
export async function waitForServer(
  healthUrl: string,
  timeoutMs: number = 30000
): Promise<boolean> {
  return waitForCondition(
    async () => {
      try {
        const response = await fetch(healthUrl);
        return response.ok;
      } catch {
        return false;
      }
    },
    timeoutMs,
    1000
  );
}

/**
 * Sleep for a specified duration.
 */
function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}
