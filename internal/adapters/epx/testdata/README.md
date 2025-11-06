# EPX Adapter Test Data

This directory contains test data and fixtures for EPX adapter testing.

## Test Cards

### Visa
- **4111111111111111** - Approved
- **4012888888881881** - Approved
- **4000000000000002** - Declined (Do not honor)

### Mastercard
- **5499740000000057** - Approved
- **5555555555554444** - Approved

### American Express
- **378282246310005** - Approved

### Discover
- **6011000990139424** - Approved

## Test Credentials

Default sandbox credentials (can be overridden with environment variables):

```bash
export EPX_TEST_CUST_NBR="9001"
export EPX_TEST_MERCH_NBR="900300"
export EPX_TEST_DBA_NBR="2"
export EPX_TEST_TERMINAL_NBR="77"
```

## Test Expiration Dates

Always use future dates in YYMM format:
- **1225** - December 2025
- **1226** - December 2026
- **0130** - January 2030

## Test CVV Codes

- **123** - Standard test CVV
- **999** - Can be used for testing
