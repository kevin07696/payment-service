/**
 * Test card data for EPX sandbox
 */

export interface TestCard {
  number: string;
  expMonth: string;
  expYear: string;
  cvv: string;
  zip: string;
  cardType: string;
}

// Current year + 1 for valid expiration
const nextYear = (new Date().getFullYear() + 1).toString();

/**
 * EPX Sandbox approval test card - ends in 0002
 * Per EPX certification sheet documentation
 */
export const VISA_APPROVAL: TestCard = {
  number: '4000000000000002',
  expMonth: '12',
  expYear: '27', // Fixed future year for EPX sandbox
  cvv: '123',
  zip: '10001',
  cardType: 'visa',
};

/**
 * Mastercard approval test card
 */
export const MASTERCARD_APPROVAL: TestCard = {
  number: '5555555555554444',
  expMonth: '12',
  expYear: nextYear,
  cvv: '123',
  zip: '12345',
  cardType: 'mastercard',
};

/**
 * Amex approval test card
 */
export const AMEX_APPROVAL: TestCard = {
  number: '378282246310005',
  expMonth: '12',
  expYear: nextYear,
  cvv: '1234',
  zip: '12345',
  cardType: 'amex',
};

/**
 * Discover approval test card
 */
export const DISCOVER_APPROVAL: TestCard = {
  number: '6011111111111117',
  expMonth: '12',
  expYear: nextYear,
  cvv: '123',
  zip: '12345',
  cardType: 'discover',
};

/**
 * Visa decline test card - triggers decline based on amount
 * See EPX documentation for amount-based decline codes
 */
export const VISA_DECLINE: TestCard = {
  number: '4000000000000002',
  expMonth: '12',
  expYear: nextYear,
  cvv: '123',
  zip: '12345',
  cardType: 'visa',
};

/**
 * Format expiration date for EPX form (YYMM format)
 */
export function formatExpDate(card: TestCard): string {
  return card.expYear.slice(-2) + card.expMonth.padStart(2, '0');
}

/**
 * Format expiration date for display (MM/YY format)
 */
export function formatExpDateDisplay(card: TestCard): string {
  return `${card.expMonth.padStart(2, '0')}/${card.expYear.slice(-2)}`;
}
