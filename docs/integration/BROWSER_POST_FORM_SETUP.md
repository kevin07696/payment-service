# Browser Post Reference

**Target Audience:** Developers implementing EPX Browser Post payment forms
**Topic:** Complete reference for building Browser Post payment forms
**Goal:** Understand how to construct and submit Browser Post forms with TAC authentication

---

## Overview

Browser Post is EPX's PCI-compliant payment method where card data is submitted directly from the user's browser to EPX, never touching your backend servers. This document provides complete examples and field reference for implementing Browser Post forms.

**Flow:**
1. Backend calls payment service to get form configuration (includes TAC token)
2. Frontend builds HTML form with card input fields
3. User enters card details
4. Form submits directly to EPX
5. EPX processes payment and redirects back to your callback URL

**Security:** Card data never touches your servers, reducing PCI compliance scope.

---

## Table of Contents

1. [Getting Form Configuration](#getting-form-configuration)
2. [Complete HTML Form Examples](#complete-html-form-example)
   - [Basic Payment Form](#basic-payment-form)
   - [Credit Card Storage Form](#credit-card-storage-form)
   - [Bank Account Storage Form (ACH)](#bank-account-storage-form-ach)
3. [JavaScript Example](#javascript-example)
4. [Field Reference](#field-reference)
5. [Transaction Types](#transaction-types)
6. [Test Cards](#test-cards)
7. [Common Issues](#common-issues)

---

## Getting Form Configuration

### Step 1: Backend Calls Payment Service

Your backend must first request form configuration from the payment service:

**Endpoint:** `GET /api/v1/payments/browser-post/form`

**Query Parameters:**
- `transaction_id` (UUID) - Frontend-generated unique transaction ID (**must be valid UUID format**)
- `merchant_id` (UUID) - Your merchant identifier (**must be valid UUID format**)
- `amount` (string) - Payment amount as decimal (e.g., "99.99"). Use "0.00" for storage transactions.
- `transaction_type` (string) - Transaction type:
  - `SALE` - Auth + capture (credit card)
  - `AUTH` - Authorization only (credit card)
  - `STORAGE` - Save card as BRIC token
  - `ACH_STORAGE_C` - Save checking account (triggers prenote verification)
  - `ACH_STORAGE_S` - Save savings account (triggers prenote verification)
- `customer_id` (string, optional) - Customer identifier (required for STORAGE/ACH_STORAGE to save payment method)
- `return_url` (string) - URL where EPX will redirect after processing

**Example Request:**
```javascript
// Backend (Node.js example)
const transactionId = generateUUID();
const formConfigUrl = `http://localhost:8081/api/v1/payments/browser-post/form?` +
  `transaction_id=${transactionId}&` +
  `merchant_id=${merchantId}&` +
  `amount=99.99&` +
  `transaction_type=SALE&` +
  `return_url=${encodeURIComponent('https://yourapp.com/payment/callback')}`;

const response = await fetch(formConfigUrl);
const formConfig = await response.json();
```

**Response:**
```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440000",
  "epxTranNbr": "1234567890",
  "tac": "abc123xyz456",
  "expiresAt": 1642445100,
  "postURL": "https://services.epxuap.com/browserpost/",
  "custNbr": "9001",
  "merchNbr": "900300",
  "dbaName": "2",
  "terminalNbr": "77",
  "redirectURL": "https://yourapp.com/payment/callback?transaction_id=550e8400...",
  "merchantId": "01234567-89ab-cdef-0123-456789abcdef",
  "merchantName": "ACME Corporation"
}
```

**Important:** TAC tokens expire in 15 minutes. Generate a new one for each payment attempt.

### Step 2: Pass Configuration to Frontend

Send the form configuration to your frontend (via API response, render in template, etc.):

```javascript
// Frontend receives config
const formConfig = await fetchFormConfig(transactionId, amount);
```

---

## Complete HTML Form Example

### Basic Payment Form

This example shows a complete, working Browser Post form:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Payment</title>
  <style>
    .payment-form { max-width: 400px; margin: 50px auto; }
    .form-group { margin-bottom: 15px; }
    label { display: block; margin-bottom: 5px; font-weight: bold; }
    input { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
    button { width: 100%; padding: 10px; background: #4CAF50; color: white; border: none; border-radius: 4px; cursor: pointer; }
    button:hover { background: #45a049; }
    .row { display: flex; gap: 10px; }
    .row .form-group { flex: 1; }
  </style>
</head>
<body>
  <div class="payment-form">
    <h2>Payment Details</h2>

    <!-- This form submits directly to EPX -->
    <form id="payment-form" action="https://services.epxuap.com/browserpost/" method="POST">

      <!-- EPX Authentication (hidden - TAC from Key Exchange) -->
      <input type="hidden" name="TAC" value="abc123xyz456">

      <!-- Merchant Credentials (from form config response) -->
      <input type="hidden" name="CUST_NBR" value="9001">
      <input type="hidden" name="MERCH_NBR" value="900300">
      <input type="hidden" name="DBA_NBR" value="2">
      <input type="hidden" name="TERMINAL_NBR" value="77">

      <!-- Transaction Details (hidden) -->
      <input type="hidden" name="TRAN_NBR" value="1234567890">
      <input type="hidden" name="TRAN_CODE" value="SALE"> <!-- SALE, AUTH, or STORAGE -->
      <input type="hidden" name="AMOUNT" value="99.99">
      <input type="hidden" name="INDUSTRY_TYPE" value="E"> <!-- E=E-commerce -->

      <!-- Custom Data (optional, echoed back in callback) -->
      <input type="hidden" name="USER_DATA_1" value="customer_id=456">
      <input type="hidden" name="USER_DATA_2" value="order_id=ORDER-12345">

      <!-- Card Details (user enters these) -->
      <div class="form-group">
        <label for="card_number">Card Number</label>
        <input type="text" id="card_number" name="ACCOUNT_NBR" placeholder="4111111111111111" required maxlength="16">
      </div>

      <div class="row">
        <div class="form-group">
          <label for="exp_date">Expiration (MMYY)</label>
          <input type="text" id="exp_date" name="EXP_DATE" placeholder="1225" required maxlength="4">
        </div>
        <div class="form-group">
          <label for="cvv">CVV</label>
          <input type="text" id="cvv" name="CVV2" placeholder="123" required maxlength="4">
        </div>
      </div>

      <!-- Cardholder Name (separate fields per EPX spec) -->
      <div class="row">
        <div class="form-group">
          <label for="first_name">First Name</label>
          <input type="text" id="first_name" name="FIRST_NAME" placeholder="John" required>
        </div>
        <div class="form-group">
          <label for="last_name">Last Name</label>
          <input type="text" id="last_name" name="LAST_NAME" placeholder="Doe" required>
        </div>
      </div>

      <!-- Billing Address (EPX field names) -->
      <div class="form-group">
        <label for="address">Billing Address</label>
        <input type="text" id="address" name="ADDRESS" placeholder="123 Main St" required>
      </div>

      <div class="row">
        <div class="form-group">
          <label for="city">City</label>
          <input type="text" id="city" name="CITY" placeholder="New York" required>
        </div>
        <div class="form-group">
          <label for="state">State</label>
          <input type="text" id="state" name="STATE" placeholder="NY" required maxlength="2">
        </div>
      </div>

      <div class="form-group">
        <label for="zip">ZIP Code</label>
        <input type="text" id="zip" name="ZIP_CODE" placeholder="10001" required maxlength="10">
      </div>

      <button type="submit">Pay $99.99</button>
    </form>
  </div>
</body>
</html>
```

**What Happens:**
1. User fills out card details
2. User clicks "Pay $99.99"
3. Browser submits form directly to EPX (`https://services.epxuap.com/browserpost/`)
4. EPX processes payment
5. EPX redirects browser to your `REDIRECT_URL` with results

---

### Credit Card Storage Form

This example shows a complete form for storing a credit card as a BRIC token for future payments (subscriptions, recurring charges, etc.):

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Save Payment Method</title>
  <style>
    .card-form { max-width: 450px; margin: 50px auto; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
    .form-group { margin-bottom: 15px; }
    label { display: block; margin-bottom: 5px; font-weight: 600; color: #333; }
    input[type="text"] { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; font-size: 16px; box-sizing: border-box; }
    input[type="text"]:focus { border-color: #4CAF50; outline: none; box-shadow: 0 0 0 2px rgba(76, 175, 80, 0.2); }
    button { width: 100%; padding: 14px; background: #4CAF50; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; font-weight: 600; }
    button:hover { background: #45a049; }
    button:disabled { background: #ccc; cursor: not-allowed; }
    .row { display: flex; gap: 12px; }
    .row .form-group { flex: 1; }
    fieldset { border: 1px solid #ddd; border-radius: 4px; padding: 15px; margin-bottom: 20px; }
    legend { font-weight: 600; padding: 0 10px; color: #333; }
    .info-box { background: #e8f5e9; padding: 15px; border-radius: 4px; margin-bottom: 20px; font-size: 14px; border-left: 4px solid #4CAF50; }
    .secure-badge { display: flex; align-items: center; gap: 8px; color: #666; font-size: 12px; margin-top: 15px; justify-content: center; }
    .card-icons { display: flex; gap: 8px; margin-top: 10px; }
    .card-icons img { height: 24px; opacity: 0.6; }
  </style>
</head>
<body>
  <div class="card-form">
    <h2>Save Payment Method</h2>

    <div class="info-box">
      <strong>Your card will be securely stored for future payments.</strong>
      <p style="margin: 8px 0 0 0;">No charge will be made today. You can remove this payment method at any time.</p>
    </div>

    <!-- This form submits directly to EPX -->
    <form id="card-storage-form" action="https://services.epxuap.com/browserpost/" method="POST">

      <!-- EPX Authentication (hidden - TAC from Key Exchange) -->
      <input type="hidden" name="TAC" id="tac" value="">

      <!-- Merchant Credentials (from form config response) -->
      <input type="hidden" name="CUST_NBR" id="cust_nbr" value="">
      <input type="hidden" name="MERCH_NBR" id="merch_nbr" value="">
      <input type="hidden" name="DBA_NBR" id="dba_nbr" value="">
      <input type="hidden" name="TERMINAL_NBR" id="terminal_nbr" value="">

      <!-- Transaction Details (hidden) -->
      <input type="hidden" name="TRAN_NBR" id="tran_nbr" value="">
      <input type="hidden" name="TRAN_CODE" value="STORAGE"> <!-- STORAGE for card tokenization -->
      <input type="hidden" name="AMOUNT" value="0.00"> <!-- $0 for storage-only -->
      <input type="hidden" name="INDUSTRY_TYPE" value="E">

      <!-- Custom Data (echoed back in callback) -->
      <input type="hidden" name="USER_DATA_1" id="user_data_1" value=""> <!-- transaction_id -->
      <input type="hidden" name="USER_DATA_2" id="user_data_2" value=""> <!-- customer_id -->
      <input type="hidden" name="USER_DATA_3" id="user_data_3" value=""> <!-- merchant_id -->

      <!-- Card Details -->
      <fieldset>
        <legend>Card Information</legend>

        <div class="form-group">
          <label for="card_nbr">Card Number</label>
          <input type="text" id="card_nbr" name="ACCOUNT_NBR"
                 placeholder="4111 1111 1111 1111" required maxlength="19"
                 autocomplete="cc-number" inputmode="numeric"
                 oninput="formatCardNumber(this)">
          <div class="card-icons">
            <span id="card-visa">Visa</span>
            <span id="card-mc">MC</span>
            <span id="card-amex">Amex</span>
            <span id="card-disc">Disc</span>
          </div>
        </div>

        <div class="row">
          <div class="form-group">
            <label for="exp_date">Expiration (MM/YY)</label>
            <input type="text" id="exp_date_display" placeholder="12/25" required maxlength="5"
                   autocomplete="cc-exp" inputmode="numeric"
                   oninput="formatExpDate(this)">
            <input type="hidden" name="EXP_DATE" id="exp_date" value="">
          </div>
          <div class="form-group">
            <label for="cvv">CVV</label>
            <input type="text" id="cvv" name="CVV2" placeholder="123" required maxlength="4"
                   autocomplete="cc-csc" inputmode="numeric">
          </div>
        </div>
      </fieldset>

      <!-- Cardholder Information -->
      <fieldset>
        <legend>Cardholder Information</legend>

        <div class="row">
          <div class="form-group">
            <label for="first_name">First Name</label>
            <input type="text" id="first_name" name="FIRST_NAME" placeholder="John" required
                   autocomplete="cc-given-name">
          </div>
          <div class="form-group">
            <label for="last_name">Last Name</label>
            <input type="text" id="last_name" name="LAST_NAME" placeholder="Doe" required
                   autocomplete="cc-family-name">
          </div>
        </div>
      </fieldset>

      <!-- Billing Address -->
      <fieldset>
        <legend>Billing Address</legend>

        <div class="form-group">
          <label for="address">Street Address</label>
          <input type="text" id="address" name="ADDRESS" placeholder="123 Main St" required
                 autocomplete="street-address">
        </div>

        <div class="row">
          <div class="form-group">
            <label for="city">City</label>
            <input type="text" id="city" name="CITY" placeholder="New York" required
                   autocomplete="address-level2">
          </div>
          <div class="form-group" style="flex: 0.5;">
            <label for="state">State</label>
            <input type="text" id="state" name="STATE" placeholder="NY" required maxlength="2"
                   autocomplete="address-level1">
          </div>
        </div>

        <div class="form-group">
          <label for="zip_code">ZIP Code</label>
          <input type="text" id="zip_code" name="ZIP_CODE" placeholder="10001" required maxlength="10"
                 autocomplete="postal-code">
        </div>
      </fieldset>

      <button type="submit" id="submit_btn">Save Card</button>

      <div class="secure-badge">
        🔒 Your card details are encrypted and sent directly to our payment processor
      </div>
    </form>
  </div>

  <script>
    // Format card number with spaces (display only)
    function formatCardNumber(input) {
      let value = input.value.replace(/\D/g, '');
      value = value.replace(/(\d{4})(?=\d)/g, '$1 ');
      input.value = value;
    }

    // Format expiration date as MM/YY and set hidden field
    function formatExpDate(input) {
      let value = input.value.replace(/\D/g, '');
      if (value.length >= 2) {
        value = value.substring(0, 2) + '/' + value.substring(2, 4);
      }
      input.value = value;

      // Set hidden field in MMYY format for EPX
      const rawValue = value.replace('/', '');
      document.getElementById('exp_date').value = rawValue;
    }

    // Luhn algorithm for card number validation
    function isValidCardNumber(number) {
      const digits = number.replace(/\D/g, '');
      if (digits.length < 13 || digits.length > 19) return false;

      let sum = 0;
      let isEven = false;
      for (let i = digits.length - 1; i >= 0; i--) {
        let digit = parseInt(digits[i], 10);
        if (isEven) {
          digit *= 2;
          if (digit > 9) digit -= 9;
        }
        sum += digit;
        isEven = !isEven;
      }
      return sum % 10 === 0;
    }

    // Validate expiration date
    function isValidExpDate(mmyy) {
      if (mmyy.length !== 4) return false;
      const month = parseInt(mmyy.substring(0, 2), 10);
      const year = parseInt('20' + mmyy.substring(2, 4), 10);
      if (month < 1 || month > 12) return false;

      const now = new Date();
      const expDate = new Date(year, month, 0); // Last day of exp month
      return expDate >= now;
    }

    // Form validation before submit
    document.getElementById('card-storage-form').addEventListener('submit', function(e) {
      const cardNbr = document.getElementById('card_nbr').value.replace(/\D/g, '');
      const expDate = document.getElementById('exp_date').value;
      const cvv = document.getElementById('cvv').value;

      // Validate card number (Luhn check)
      if (!isValidCardNumber(cardNbr)) {
        e.preventDefault();
        alert('Please enter a valid card number.');
        document.getElementById('card_nbr').focus();
        return false;
      }

      // Validate expiration date
      if (!isValidExpDate(expDate)) {
        e.preventDefault();
        alert('Please enter a valid expiration date.');
        document.getElementById('exp_date_display').focus();
        return false;
      }

      // Validate CVV
      if (cvv.length < 3) {
        e.preventDefault();
        alert('Please enter a valid CVV.');
        document.getElementById('cvv').focus();
        return false;
      }

      // Remove spaces from card number before submit
      document.getElementById('card_nbr').value = cardNbr;

      // Disable button to prevent double-submit
      document.getElementById('submit_btn').disabled = true;
      document.getElementById('submit_btn').textContent = 'Saving...';
    });

    // Initialize form with configuration from payment service
    async function initializeForm(merchantId, customerId, returnUrl) {
      const transactionId = generateUUID();

      // Get form configuration from payment service
      const formConfigUrl = `/api/v1/payments/browser-post/form?` +
        `transaction_id=${transactionId}&` +
        `merchant_id=${merchantId}&` +
        `amount=0.00&` +
        `transaction_type=STORAGE&` +
        `customer_id=${encodeURIComponent(customerId)}&` +
        `return_url=${encodeURIComponent(returnUrl)}`;

      try {
        const response = await fetch(formConfigUrl);
        if (!response.ok) throw new Error('Failed to get form configuration');

        const config = await response.json();

        // Populate hidden fields - including merchant credentials
        document.getElementById('tac').value = config.tac;
        document.getElementById('cust_nbr').value = config.custNbr;
        document.getElementById('merch_nbr').value = config.merchNbr;
        document.getElementById('dba_nbr').value = config.dbaName;
        document.getElementById('terminal_nbr').value = config.terminalNbr;
        document.getElementById('tran_nbr').value = config.epxTranNbr;
        document.getElementById('user_data_1').value = transactionId;
        document.getElementById('user_data_2').value = customerId;
        document.getElementById('user_data_3').value = merchantId;

        // Update form action
        document.getElementById('card-storage-form').action = config.postURL;

        console.log('Form initialized. TAC expires at:', new Date(config.expiresAt * 1000));

      } catch (error) {
        console.error('Failed to initialize form:', error);
        alert('Failed to load payment form. Please refresh and try again.');
      }
    }

    function generateUUID() {
      return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });
    }

    // Example: Initialize form on page load
    // initializeForm('merchant-uuid', 'customer-123', 'https://yourapp.com/account/payment-methods');
  </script>
</body>
</html>
```

**What Happens:**
1. Frontend calls `initializeForm(merchantId, customerId, returnUrl)` on page load
2. Payment service returns TAC token and form configuration
3. User enters card details (validated client-side with Luhn algorithm)
4. User clicks "Save Card"
5. Browser submits form directly to EPX (card data never touches your servers)
6. EPX tokenizes the card and generates a **BRIC** (Business Relationship Identification Code)
7. EPX redirects to your callback URL with results
8. Callback handler:
   - Updates the pending transaction with EPX response
   - Saves the BRIC to `customer_payment_methods` table
   - Payment method is immediately active (`is_active=true`)
9. User is redirected to the return URL

**Key Fields for Storage:**
- `TRAN_CODE=STORAGE` - Indicates storage transaction (no charge)
- `amount=0.00` - Zero amount since we're just storing
- `customer_id` - Required to associate the saved card with a customer

**Security Notes:**
- Card data is submitted directly to EPX (PCI DSS compliant)
- Luhn validation catches typos before submission
- TAC token expires in 15 minutes
- BRIC token is stored instead of actual card number

**Using the Stored Card Later:**
```javascript
// Server-side: Charge the stored card
const result = await paymentService.Sale({
  merchantId: 'merchant-uuid',
  paymentMethodId: 'saved-payment-method-uuid', // References the stored BRIC
  amount: '29.99',
  currency: 'USD'
});
```

---

### Bank Account Storage Form (ACH)

This example shows a complete form for storing a bank account for future ACH debits:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Add Bank Account</title>
  <style>
    .bank-form { max-width: 450px; margin: 50px auto; }
    .form-group { margin-bottom: 15px; }
    label { display: block; margin-bottom: 5px; font-weight: bold; }
    input[type="text"] { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
    button { width: 100%; padding: 12px; background: #2196F3; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; }
    button:hover { background: #1976D2; }
    .row { display: flex; gap: 10px; }
    .row .form-group { flex: 1; }
    .account-type { display: flex; gap: 20px; margin: 10px 0; }
    .account-type label { display: flex; align-items: center; gap: 8px; font-weight: normal; cursor: pointer; }
    .account-type input[type="radio"] { width: auto; }
    fieldset { border: 1px solid #ddd; border-radius: 4px; padding: 15px; margin-bottom: 15px; }
    legend { font-weight: bold; padding: 0 10px; }
    .info-box { background: #e3f2fd; padding: 15px; border-radius: 4px; margin-bottom: 20px; font-size: 14px; }
    .info-box ul { margin: 10px 0 0 20px; padding: 0; }
  </style>
</head>
<body>
  <div class="bank-form">
    <h2>Add Bank Account</h2>

    <div class="info-box">
      <strong>How it works:</strong>
      <ul>
        <li>Your bank account will be securely stored for future payments</li>
        <li>A $0.00 verification transaction (prenote) will be sent</li>
        <li>Once verified, your account can be used for recurring payments</li>
      </ul>
    </div>

    <!-- This form submits directly to EPX -->
    <form id="bank-form" action="https://services.epxuap.com/browserpost/" method="POST">

      <!-- EPX Authentication (hidden - TAC from Key Exchange) -->
      <input type="hidden" name="TAC" id="tac" value="">

      <!-- Merchant Credentials (from form config response) -->
      <input type="hidden" name="CUST_NBR" id="cust_nbr" value="">
      <input type="hidden" name="MERCH_NBR" id="merch_nbr" value="">
      <input type="hidden" name="DBA_NBR" id="dba_nbr" value="">
      <input type="hidden" name="TERMINAL_NBR" id="terminal_nbr" value="">

      <!-- Transaction Details (hidden) -->
      <input type="hidden" name="TRAN_NBR" id="tran_nbr" value="">
      <input type="hidden" name="TRAN_CODE" id="tran_code" value="ACH_STORAGE_C">
      <input type="hidden" name="AMOUNT" value="0.00"> <!-- $0 for storage-only -->
      <input type="hidden" name="INDUSTRY_TYPE" value="E">

      <!-- Custom Data (echoed back in callback) -->
      <input type="hidden" name="USER_DATA_1" id="user_data_1" value=""> <!-- transaction_id -->
      <input type="hidden" name="USER_DATA_2" id="user_data_2" value=""> <!-- customer_id -->
      <input type="hidden" name="USER_DATA_3" id="user_data_3" value=""> <!-- merchant_id -->

      <!-- Account Type Selection -->
      <fieldset>
        <legend>Account Type</legend>
        <div class="account-type">
          <label>
            <input type="radio" name="account_type_select" value="checking" checked
                   onchange="setAccountType('checking')">
            Checking
          </label>
          <label>
            <input type="radio" name="account_type_select" value="savings"
                   onchange="setAccountType('savings')">
            Savings
          </label>
        </div>
      </fieldset>
      <input type="hidden" name="ACCOUNT_TYPE" id="account_type" value="C">

      <!-- Bank Account Details -->
      <fieldset>
        <legend>Bank Account Information</legend>

        <div class="form-group">
          <label for="routing_nbr">Routing Number (9 digits)</label>
          <input type="text" id="routing_nbr" name="ROUTING_NBR"
                 placeholder="021000021" required maxlength="9" pattern="[0-9]{9}"
                 title="Please enter a 9-digit routing number">
        </div>

        <div class="form-group">
          <label for="account_nbr">Account Number</label>
          <input type="text" id="account_nbr" name="ACCOUNT_NBR"
                 placeholder="123456789" required maxlength="17"
                 title="Please enter your bank account number">
        </div>

        <div class="form-group">
          <label for="account_nbr_confirm">Confirm Account Number</label>
          <input type="text" id="account_nbr_confirm"
                 placeholder="123456789" required maxlength="17"
                 title="Please re-enter your bank account number">
        </div>
      </fieldset>

      <!-- Account Holder Information -->
      <fieldset>
        <legend>Account Holder</legend>

        <div class="row">
          <div class="form-group">
            <label for="first_name">First Name</label>
            <input type="text" id="first_name" name="FIRST_NAME" placeholder="John" required>
          </div>
          <div class="form-group">
            <label for="last_name">Last Name</label>
            <input type="text" id="last_name" name="LAST_NAME" placeholder="Doe" required>
          </div>
        </div>

        <div class="form-group">
          <label for="address">Street Address</label>
          <input type="text" id="address" name="ADDRESS" placeholder="123 Main St" required>
        </div>

        <div class="row">
          <div class="form-group">
            <label for="city">City</label>
            <input type="text" id="city" name="CITY" placeholder="New York" required>
          </div>
          <div class="form-group" style="flex: 0.5;">
            <label for="state">State</label>
            <input type="text" id="state" name="STATE" placeholder="NY" required maxlength="2">
          </div>
        </div>

        <div class="form-group">
          <label for="zip_code">ZIP Code</label>
          <input type="text" id="zip_code" name="ZIP_CODE" placeholder="10001" required maxlength="10">
        </div>
      </fieldset>

      <button type="submit" id="submit_btn">Save Bank Account</button>
    </form>
  </div>

  <script>
    // Set account type based on radio selection
    function setAccountType(type) {
      if (type === 'checking') {
        document.getElementById('tran_code').value = 'ACH_STORAGE_C';
        document.getElementById('account_type').value = 'C';
      } else {
        document.getElementById('tran_code').value = 'ACH_STORAGE_S';
        document.getElementById('account_type').value = 'S';
      }
    }

    // Validate account numbers match before submit
    document.getElementById('bank-form').addEventListener('submit', function(e) {
      const accountNbr = document.getElementById('account_nbr').value;
      const confirmNbr = document.getElementById('account_nbr_confirm').value;

      if (accountNbr !== confirmNbr) {
        e.preventDefault();
        alert('Account numbers do not match. Please re-enter.');
        document.getElementById('account_nbr_confirm').focus();
        return false;
      }

      // Validate routing number (basic check)
      const routingNbr = document.getElementById('routing_nbr').value;
      if (!/^[0-9]{9}$/.test(routingNbr)) {
        e.preventDefault();
        alert('Please enter a valid 9-digit routing number.');
        document.getElementById('routing_nbr').focus();
        return false;
      }
    });

    // Initialize form with configuration from payment service
    async function initializeForm(merchantId, customerId, returnUrl) {
      const transactionId = generateUUID();
      const accountType = document.querySelector('input[name="account_type_select"]:checked').value;
      const transactionType = accountType === 'checking' ? 'ACH_STORAGE_C' : 'ACH_STORAGE_S';

      // Get form configuration from payment service
      const formConfigUrl = `/api/v1/payments/browser-post/form?` +
        `transaction_id=${transactionId}&` +
        `merchant_id=${merchantId}&` +
        `amount=0.00&` +
        `transaction_type=${transactionType}&` +
        `customer_id=${encodeURIComponent(customerId)}&` +
        `return_url=${encodeURIComponent(returnUrl)}`;

      try {
        const response = await fetch(formConfigUrl);
        const config = await response.json();

        // Populate hidden fields - including merchant credentials
        document.getElementById('tac').value = config.tac;
        document.getElementById('cust_nbr').value = config.custNbr;
        document.getElementById('merch_nbr').value = config.merchNbr;
        document.getElementById('dba_nbr').value = config.dbaName;
        document.getElementById('terminal_nbr').value = config.terminalNbr;
        document.getElementById('tran_nbr').value = config.epxTranNbr;
        document.getElementById('user_data_1').value = transactionId;
        document.getElementById('user_data_2').value = customerId;
        document.getElementById('user_data_3').value = merchantId;

        // Update form action
        document.getElementById('bank-form').action = config.postURL;

      } catch (error) {
        console.error('Failed to initialize form:', error);
        alert('Failed to load form. Please refresh and try again.');
      }
    }

    function generateUUID() {
      return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });
    }

    // Example: Initialize form on page load
    // initializeForm('merchant-uuid', 'customer-123', 'https://yourapp.com/account/payment-methods');
  </script>
</body>
</html>
```

**What Happens:**
1. User selects Checking or Savings account type
2. User enters bank account details (routing number, account number)
3. User confirms account number (client-side validation)
4. User clicks "Save Bank Account"
5. Browser submits form directly to EPX
6. EPX stores the bank account and returns a storage BRIC
7. EPX redirects to your callback URL
8. Payment service:
   - Saves unverified payment method (`is_active=false`, `verification_status=pending`)
   - Sends prenote (CKC0 for checking, CKS0 for savings) using the BRIC
9. After SFTP return file processing confirms no returns, account is verified

**Security Notes:**
- Bank account numbers are submitted directly to EPX (never touch your servers)
- Account number confirmation prevents typos
- Routing number validation ensures valid format
- PCI DSS compliance maintained (sensitive data bypasses merchant)

---

## JavaScript Example

### Dynamic Form Population

Most applications generate forms dynamically using the configuration from the payment service:

```javascript
async function createPaymentForm(merchantId, amount, transactionType) {
  // Step 1: Get form configuration from payment service
  const transactionId = generateUUID();
  const returnUrl = `${window.location.origin}/payment/callback`;

  const formConfigUrl = `/api/v1/payments/browser-post/form?` +
    `transaction_id=${transactionId}&` +
    `merchant_id=${merchantId}&` +
    `amount=${amount}&` +
    `transaction_type=${transactionType}&` +
    `return_url=${encodeURIComponent(returnUrl)}`;

  const response = await fetch(formConfigUrl);
  const config = await response.json();

  // Step 2: Create form element
  const form = document.createElement('form');
  form.method = 'POST';
  form.action = config.postURL; // EPX endpoint

  // Step 3: Add hidden fields
  // TRAN_CODE: SALE, AUTH, STORAGE, ACH_STORAGE_C, ACH_STORAGE_S
  const tranCode = transactionType; // Use the transaction type directly as TRAN_CODE

  const hiddenFields = {
    'TAC': config.tac,
    'CUST_NBR': config.custNbr,           // Merchant credentials from form config
    'MERCH_NBR': config.merchNbr,
    'DBA_NBR': config.dbaName,
    'TERMINAL_NBR': config.terminalNbr,
    'TRAN_NBR': config.epxTranNbr,
    'TRAN_CODE': tranCode,
    'AMOUNT': amount,                      // Amount in dollars.cents format
    'INDUSTRY_TYPE': 'E',
    // Note: REDIRECT_URL is embedded in TAC, not sent separately
    'USER_DATA_1': transactionId,          // For tracking in callback
    'USER_DATA_2': merchantId,
  };

  Object.entries(hiddenFields).forEach(([name, value]) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    form.appendChild(input);
  });

  // Step 4: Add card input fields (or get from existing form)
  // EPX field names: ACCOUNT_NBR, EXP_DATE, CVV2, FIRST_NAME, LAST_NAME, ADDRESS, CITY, STATE, ZIP_CODE
  form.innerHTML += `
    <label>Card Number: <input type="text" name="ACCOUNT_NBR" required maxlength="16"></label>
    <label>Expiration (MMYY): <input type="text" name="EXP_DATE" required maxlength="4"></label>
    <label>CVV: <input type="text" name="CVV2" required maxlength="4"></label>
    <label>First Name: <input type="text" name="FIRST_NAME" required></label>
    <label>Last Name: <input type="text" name="LAST_NAME" required></label>
    <label>Address: <input type="text" name="ADDRESS" required></label>
    <label>City: <input type="text" name="CITY" required></label>
    <label>State: <input type="text" name="STATE" required maxlength="2"></label>
    <label>ZIP Code: <input type="text" name="ZIP_CODE" required maxlength="10"></label>
    <button type="submit">Pay $${amount}</button>
  `;

  // Step 5: Add to page and submit
  document.body.appendChild(form);
  return form;
}

// Usage
const form = await createPaymentForm('merchant-id-uuid', '99.99', 'SALE');

function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}
```

---

## Field Reference

### Required Hidden Fields

These fields must be included in every Browser Post form:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `TAC` | string | Transaction Authentication Code from Key Exchange | `"abc123xyz456"` |
| `CUST_NBR` | string | EPX customer number | `"9001"` |
| `MERCH_NBR` | string | EPX merchant number | `"900300"` |
| `DBA_NBR` | string | EPX DBA number | `"2"` |
| `TERMINAL_NBR` | string | EPX terminal number | `"77"` |
| `TRAN_NBR` | string | Transaction number (numeric, max 10 digits) | `"1234567890"` |
| `TRAN_CODE` | string | Transaction type: `SALE`, `AUTH`, `STORAGE`, `ACH_STORAGE_C`, `ACH_STORAGE_S` | `"SALE"` |
| `AMOUNT` | string | Transaction amount in dollars.cents format | `"99.99"` |
| `INDUSTRY_TYPE` | string | Industry type (E=E-commerce) | `"E"` |

**Note:** The `REDIRECT_URL` is embedded in the TAC token during Key Exchange and should NOT be sent separately in the Browser Post form. All merchant credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR) and AMOUNT MUST be included in the form.

### Required Card Fields (User Input)

These fields are filled by the user:

| Field | Type | Description | Example | Validation |
|-------|------|-------------|---------|------------|
| `ACCOUNT_NBR` | string | Credit card number (no spaces) | `"4111111111111111"` | 13-16 digits |
| `EXP_DATE` | string | Expiration date (MMYY format, single field) | `"1225"` | 4 digits |
| `CVV2` | string | Card verification value | `"123"` | 3-4 digits |
| `FIRST_NAME` | string | Cardholder first name | `"John"` | Required |
| `LAST_NAME` | string | Cardholder last name | `"Doe"` | Required |
| `ADDRESS` | string | Billing street address | `"123 Main St"` | Required |
| `CITY` | string | Billing city | `"New York"` | Required |
| `STATE` | string | Billing state (2-letter code) | `"NY"` | 2 chars |
| `ZIP_CODE` | string | Billing ZIP/postal code | `"10001"` | 5-10 chars |

### Optional Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `USER_DATA_1` | string | Custom data (echoed back in callback) | `"transaction-uuid"` |
| `USER_DATA_2` | string | Custom data (echoed back in callback) | `"customer-uuid"` |
| `USER_DATA_3` | string | Custom data (echoed back in callback) | `"merchant-uuid"` |
| `EMAIL` | string | Customer email address | `"john@example.com"` |
| `PHONE` | string | Customer phone number | `"555-123-4567"` |

### BRIC Storage Fields (Save Card for Future Use)

To store a card as a BRIC token for future payments, use `TRAN_CODE=STORAGE`:

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_CODE` | `"STORAGE"` | Storage transaction type |

**Note:** Storage transactions create a BRIC token without charging the card. The AMOUNT in Key Exchange should be `"0.00"` for storage-only transactions.

### ACH Form Fields (Bank Account Input)

For ACH storage transactions (ACH_STORAGE_C or ACH_STORAGE_S), use these bank account fields instead of card fields:

| Field | Type | Description | Example | Validation |
|-------|------|-------------|---------|------------|
| `ROUTING_NBR` | string | Bank routing number (ABA) | `"021000021"` | 9 digits |
| `ACCOUNT_NBR` | string | Bank account number | `"123456789"` | 4-17 digits |
| `ACCOUNT_TYPE` | string | Account type: `C` (Checking), `S` (Savings) | `"C"` | C or S |
| `FIRST_NAME` | string | Account holder first name | `"John"` | Required |
| `LAST_NAME` | string | Account holder last name | `"Doe"` | Required |
| `ADDRESS` | string | Billing street address | `"123 Main St"` | Required |
| `CITY` | string | Billing city | `"New York"` | Required |
| `STATE` | string | Billing state (2-letter code) | `"NY"` | 2 chars |
| `ZIP_CODE` | string | Billing ZIP/postal code | `"10001"` | 5-10 chars |

**ACH Browser Post Form Example:**

```html
<form id="ach-form" action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- EPX Authentication -->
  <input type="hidden" name="TAC" value="abc123xyz456">

  <!-- Merchant Credentials (from form config response) -->
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">

  <!-- Transaction Details -->
  <input type="hidden" name="TRAN_NBR" value="1234567890">
  <input type="hidden" name="TRAN_CODE" id="tran_code" value="ACH_STORAGE_C">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">

  <!-- Account Type Selection (user selects) -->
  <fieldset>
    <legend>Account Type</legend>
    <label>
      <input type="radio" name="account_type_selection" value="checking" checked
             onchange="document.getElementById('tran_code').value='ACH_STORAGE_C'; document.getElementById('account_type').value='C';">
      Checking
    </label>
    <label>
      <input type="radio" name="account_type_selection" value="savings"
             onchange="document.getElementById('tran_code').value='ACH_STORAGE_S'; document.getElementById('account_type').value='S';">
      Savings
    </label>
  </fieldset>
  <input type="hidden" name="ACCOUNT_TYPE" id="account_type" value="C">

  <!-- Bank Account Details (user enters these) -->
  <label>Routing Number: <input type="text" name="ROUTING_NBR" required maxlength="9"></label>
  <label>Account Number: <input type="text" name="ACCOUNT_NBR" required></label>

  <!-- Account Holder Info -->
  <label>First Name: <input type="text" name="FIRST_NAME" required></label>
  <label>Last Name: <input type="text" name="LAST_NAME" required></label>

  <!-- Billing Address -->
  <label>Address: <input type="text" name="ADDRESS" required></label>
  <label>City: <input type="text" name="CITY" required></label>
  <label>State: <input type="text" name="STATE" required maxlength="2"></label>
  <label>ZIP Code: <input type="text" name="ZIP_CODE" required></label>

  <button type="submit">Save Bank Account</button>
</form>
```

**Note:** After ACH storage callback, the payment service automatically:
1. Saves the bank account as an unverified payment method (`is_active=false`, `verification_status=pending`)
2. Sends a prenote (CKC0 for checking, CKS0 for savings) using the storage BRIC
3. The payment method remains inactive until verified via SFTP return file processing (to be configured)

---

## Transaction Types

### SALE (Auth + Capture) - TRAN_CODE=SALE

Authorizes and captures funds in a single step:

```javascript
// In Key Exchange request
{ "TRAN_GROUP": "SALE", "AMOUNT": "99.99" }

// In Browser Post form
{ "TRAN_CODE": "SALE" }
```

**Use Case:** Standard e-commerce checkout, immediate payment

### AUTH (Authorization Only) - TRAN_CODE=AUTH

Holds funds but doesn't capture (capture later via Server Post):

```javascript
// In Key Exchange request
{ "TRAN_GROUP": "AUTH", "AMOUNT": "99.99" }

// In Browser Post form
{ "TRAN_CODE": "AUTH" }
```

**Use Case:** Pre-authorization (hotels, car rentals), delayed capture

**Follow-up:** Use Server Post `Capture` RPC to capture funds later

### STORAGE (Save Card Only) - TRAN_CODE=STORAGE

Stores card as BRIC token without charging:

```javascript
// In Key Exchange request
{ "TRAN_GROUP": "STORAGE", "AMOUNT": "0.00" }

// In Browser Post form
{ "TRAN_CODE": "STORAGE" }
```

**Use Case:** Save payment method for future subscriptions/payments

**Follow-up:** Use BRIC token with Server Post APIs for future charges

### ACH_STORAGE_C (Save Checking Account) - TRAN_CODE=ACH_STORAGE_C

Stores checking account as BRIC token for ACH debits:

```javascript
// In Key Exchange request
{ "TRAN_GROUP": "STORAGE", "AMOUNT": "0.00" }

// In payment service endpoint
{ "transaction_type": "ACH_STORAGE_C" }
```

**Use Case:** Save checking account for recurring ACH debits

**Flow:**
1. Browser Post stores bank account and returns storage BRIC
2. Backend saves payment method as unverified (`is_active=false`, `verification_status="pending"`)
3. Backend automatically sends prenote (CKC0) using the storage BRIC
4. SFTP return files are processed to detect any ACH returns
5. If no returns, payment method is verified and becomes active for recurring debits

### ACH_STORAGE_S (Save Savings Account) - TRAN_CODE=ACH_STORAGE_S

Stores savings account as BRIC token for ACH debits:

```javascript
// In Key Exchange request
{ "TRAN_GROUP": "STORAGE", "AMOUNT": "0.00" }

// In payment service endpoint
{ "transaction_type": "ACH_STORAGE_S" }
```

**Use Case:** Save savings account for recurring ACH debits

**Flow:** Same as ACH_STORAGE_C but uses prenote type CKS0 for savings accounts

**Note:** SFTP return file processing is required to verify accounts. Until configured, payment methods remain in `pending` status.

---

## Test Cards

### EPX Sandbox Test Cards

**Approval Cards:**

| Card Number | Brand | CVV | Exp | Result |
|-------------|-------|-----|-----|--------|
| `4111111111111111` | Visa | 123 | 12/25 | ✅ Approved |
| `4788250000028291` | Visa | 123 | 12/25 | ✅ Approved |
| `5454545454545454` | Mastercard | 123 | 12/25 | ✅ Approved |

**Decline/Error Cards:**

Use approved card numbers with specific amounts to trigger error codes:

| Amount | Response Code | Meaning |
|--------|--------------|---------|
| `$1.05` | 05 | Do Not Honor |
| `$1.20` | 51 | Insufficient Funds |
| `$1.54` | 54 | Expired Card |
| `$1.91` | 91 | Issuer Unavailable |

**Example:**
```javascript
// To test "Insufficient Funds" decline:
{
  "ACCOUNT_NBR": "4111111111111111",
  "AMOUNT": "1.20",  // Triggers code 51
  "CVV2": "123",
  "EXP_DATE": "1225"
}
```

---

## Common Issues

### Issue: "TAC validation failed" (EPX Code 58)

**Cause:** Invalid or expired TAC token

**Solutions:**
- TAC expires in 15 minutes - generate new one if expired
- Verify MAC_SECRET matches EPX account
- Check merchant credentials (CUST_NBR, MERCH_NBR, etc.) are correct
- Ensure amount, tran_nbr match what was sent in Key Exchange request

### Issue: Callback not received

**Cause:** EPX cannot reach your callback URL

**Solutions:**
- Verify `REDIRECT_URL` is publicly accessible (use ngrok for local dev)
- Check HTTPS/TLS certificate is valid
- Verify firewall allows EPX IPs
- Check server logs for incoming POST requests

### Issue: Form submits but nothing happens

**Cause:** EPX endpoint URL incorrect

**Solutions:**
- Verify form `action` is correct EPX endpoint:
  - Sandbox: `https://services.epxuap.com/browserpost/`
  - Production: `https://secure.epxuap.com/browserpost/`
- Check browser console for CORS errors
- Ensure form method is `POST`

### Issue: Card declined unexpectedly

**Cause:** AVS (Address Verification System) mismatch

**Solutions:**
- For test cards, use ZIP: `12345`
- Ensure AVS_ZIP matches card billing address
- Check EPX merchant settings for AVS requirements

### Issue: "Amount mismatch" error

**Cause:** Amount in form doesn't match TAC request

**Solutions:**
- Ensure `AMOUNT` field matches the amount used in Key Exchange request
- TAC is tied to specific amount - can't change after generation
- Generate new TAC if amount changes

---

## Best Practices

1. **Always use HTTPS** - Even in development (use ngrok)
2. **Validate card data client-side** - Before submitting to EPX (Luhn check, expiration date)
3. **Show loading state** - Disable submit button while processing
4. **Handle popup blockers** - If using `target="_blank"`, ensure user interaction triggered it
5. **Implement timeouts** - TAC expires in 15 minutes
6. **Never log card data** - PCI compliance violation
7. **Use USER_DATA fields** - To track transactions in your system
8. **Test all flows** - Approval, decline, timeout, callback failure

---

## Next Steps

- **[Getting Started](GETTING_STARTED.md)** - Quick start integration guide
- **[API Specs](API_SPECS.md)** - Complete API reference
- **[Payment Dataflows](../development/DATAFLOW.md)** - Detailed payment flow diagrams
- **[FAQ](../wiki-templates/FAQ.md)** - Common questions answered

---

**Questions?** Open an issue on [GitHub](https://github.com/kevin07696/payment-service/issues) or check the [FAQ](wiki-templates/FAQ.md).
