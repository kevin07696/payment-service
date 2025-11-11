# Server POST Payment Flow - Technical Reference

**Date**: 2025-11-11
**Status**: ✅ CURRENT
**Payment Methods**: Credit Cards & ACH
**Integration Type**: Server-to-Server (gRPC)

> **For Browser POST (Credit Cards Only)**: See [BROWSER_POST_DATAFLOW.md](./BROWSER_POST_DATAFLOW.md)

---

## Executive Summary

This document analyzes the complete dataflow for **Server POST** payment transactions - direct server-to-server integration between calling services (POS/e-commerce) and the Payment Service via gRPC. Server POST supports both credit cards and ACH payments using saved payment methods or BRIC tokens.

**Key Characteristics**:
- Direct gRPC API calls (no browser involvement)
- Supports credit cards AND ACH payments
- Uses saved payment methods or BRIC tokens (no raw card data)
- Immediate synchronous response with complete transaction data
- PCI-compliant (card data never touches calling service)

---

## Architecture Overview

**Server POST Flow (POS → Payment Service → EPX)**:
1. POS sends payment request to Payment Service gRPC API
2. Payment Service translates request to EPX Server Post API
3. EPX processes transaction with card networks
4. Payment Service returns complete transaction data to POS
5. POS renders receipt with transaction details

**Use Cases**:
- Recurring payments with saved payment methods
- ACH transactions (checking/savings account debits)
- POS terminal payments with stored BRIC tokens
- Subscription billing
- Refunds, voids, captures

---

## Server POST Dataflow

### 1.1 POS Request → Payment Service (gRPC)

**Entry Point**: `PaymentService.Sale()` or `PaymentService.Authorize()`

**Proto Definition** (payment/v1/payment.proto):
```
message SaleRequest {
  string agent_id = 1;              // Merchant ID
  string customer_id = 2;            // Optional customer ID
  string amount = 3;                 // Decimal as string
  string currency = 4;               // ISO 4217 code
  oneof payment_method {
    string payment_method_id = 5;    // Saved payment method UUID
    string payment_token = 6;        // EPX token (AUTH_GUID/BRIC)
  }
  string idempotency_key = 7;
  map<string, string> metadata = 8;
}
```

**Handler Processing** (payment_handler.go):
- Validates request
- Converts proto to internal service request
- Calls service layer
- Converts transaction domain model to proto response

### 1.2 Payment Service Processing

**Service Layer**: Internal payment service processes the request
1. Validates payment method (saved or token)
2. Prepares Server Post request for EPX
3. Calls EPX Server Post adapter

### 1.3 EPX Server Post Adapter

**Adapter** (server_post_adapter.go):

**Request Construction**:
```
ServerPostRequest {
  CustNbr        string  // EPX customer number
  MerchNbr       string  // EPX merchant number
  DBAnbr         string  // EPX DBA number
  TerminalNbr    string  // EPX terminal number
  TransactionType        // "CCE1" (Sale), "CCE2" (Auth), etc.
  Amount         string  // Transaction amount
  AuthGUID       string  // BRIC token for repeat payments
  TranNbr        string  // Unique transaction number
  TranGroup      string  // Group ID
  CardEntryMethod *string // "Z" for BRIC, "E" for ecommerce
  IndustryType   *string  // "E" for ecommerce
  BillingInfo    // First/Last name, Address, City, State, Zip
}
```

**EPX Gateway Response**:
```
ServerPostResponse {
  AuthGUID     string  // BRIC token for future transactions
  AuthResp     string  // "00" = approved, "05" = declined, etc.
  AuthCode     string  // Bank auth code
  AuthRespText string  // Human-readable message
  AuthCardType string  // "V"/"M"/"A"/"D"
  AuthAVS      string  // Address verification result
  AuthCVV2     string  // CVV verification result
  TranNbr      string  // Echo-back transaction number
  TranGroup    string  // Echo-back group ID
  Amount       string  // Echo-back amount
}
```

### 1.4 Payment Service Response to POS

**PaymentResponse** (payment/v1/payment.proto):
```
message PaymentResponse {
  string transaction_id = 1;        // Our transaction UUID
  string group_id = 2;              // Groups related transactions
  string agent_id = 3;              // Merchant ID
  string customer_id = 4;           // Customer ID (if provided)
  string amount = 5;                // Decimal as string
  string currency = 6;              // ISO 4217 code
  TransactionStatus status = 7;     // COMPLETED, FAILED
  TransactionType type = 8;         // CHARGE, AUTH, etc.
  PaymentMethodType payment_method_type = 9;
  
  // EPX Gateway response fields
  string auth_guid = 10;            // BRIC for future use
  string auth_resp = 11;            // "00" = approved
  string auth_code = 12;            // Bank auth code
  string auth_resp_text = 13;       // Human-readable message
  string auth_card_type = 14;       // Card brand
  string auth_avs = 15;             // AVS result
  string auth_cvv2 = 16;            // CVV result
  
  bool is_approved = 17;
  google.protobuf.Timestamp created_at = 18;
  map<string, string> metadata = 19;
}
```

### 1.5 Server POST Dataflow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ POS SYSTEM                                                   │
│                                                              │
│  1. Send SaleRequest:                                       │
│     - agent_id, amount, currency                           │
│     - payment_token (BRIC from previous txn)              │
│     - idempotency_key                                      │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (gRPC Handler)                              │
│                                                              │
│  - Validates request                                        │
│  - Converts proto to service request                        │
│  - Calls service.Sale()                                    │
└──────────────────────────┬──────────────────────────────────┘
                           │ Internal Service Layer
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (Service Layer)                             │
│                                                              │
│  - Validates payment method                                 │
│  - Prepares ServerPostRequest with:                        │
│    * EPX credentials (cust_nbr, merch_nbr, etc.)          │
│    * Transaction details (tran_nbr, amount, auth_guid)    │
│    * Billing info (if saved payment)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │ Call EPX Adapter
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ EPX SERVER POST ADAPTER                                      │
│                                                              │
│  - Builds form data (URL-encoded or XML)                    │
│  - POSTs to: https://epxnow.com/epx/server_post           │
│  - Method: HTTPS POST (application/x-www-form-urlencoded) │
│  - Retries up to 3x on transient failures                 │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ EPX GATEWAY (epxnow.com)                                    │
│                                                              │
│  - Processes transaction                                    │
│  - Communicates with card network                          │
│  - Returns key-value response:                             │
│    * AUTH_GUID (BRIC token)                               │
│    * AUTH_RESP (00 = approved)                            │
│    * AUTH_CODE (bank approval code)                        │
│    * AUTH_CARD_TYPE (V/M/A/D)                             │
│    * AUTH_AVS, AUTH_CVV2 (verification results)           │
└──────────────────────────┬──────────────────────────────────┘
                           │ Response
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ EPX ADAPTER (Parse Response)                                │
│                                                              │
│  - Parses key-value response                                │
│  - Creates ServerPostResponse                              │
│  - Returns to service layer                                │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (Service Layer)                             │
│                                                              │
│  - Creates domain.Transaction with:                         │
│    * ID, GroupID (UUIDs)                                    │
│    * AgentID, Amount, Currency                              │
│    * Status (COMPLETED/FAILED)                              │
│    * AuthGUID, AuthResp, AuthCode, etc.                    │
│  - Stores to database                                       │
│  - Returns domain.Transaction                              │
└──────────────────────────┬──────────────────────────────────┘
                           │ Convert to Proto
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (gRPC Handler)                              │
│                                                              │
│  - Converts domain.Transaction to PaymentResponse          │
│  - Returns to POS                                           │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ POS SYSTEM                                                   │
│                                                              │
│  2. Receive PaymentResponse:                               │
│     - transaction_id, group_id                             │
│     - agent_id, customer_id (if provided)                 │
│     - amount, currency, status                             │
│     - auth_code, auth_resp, auth_guid                      │
│     - card info: auth_card_type, auth_avs, auth_cvv2      │
│     - created_at timestamp                                 │
│                                                             │
│  3. POS can now render receipt with this data              │
└─────────────────────────────────────────────────────────────┘
```

### 1.6 Server POST Dataflow Summary

| Step | From | To | Data | Key Fields |
|------|------|-----|------|-----------|
| 1 | POS | Handler | SaleRequest | agent_id, amount, currency, payment_token, idempotency_key |
| 2 | Handler | Service | Internal SaleRequest | AgentID, Amount, Currency, PaymentToken |
| 3 | Service | EPX Adapter | ServerPostRequest | CustNbr, MerchNbr, Amount, AuthGUID, TranNbr, TranGroup |
| 4 | EPX Adapter | EPX | Form data (POST) | CUST_NBR, MERCH_NBR, AMOUNT, AUTH_GUID, TRAN_NBR, TRAN_GROUP |
| 5 | EPX | EPX Adapter | Response | AUTH_GUID, AUTH_RESP, AUTH_CODE, AUTH_CARD_TYPE, AUTH_AVS, AUTH_CVV2 |
| 6 | EPX Adapter | Service | ServerPostResponse | AuthGUID, AuthResp, AuthCode, AuthCardType, AuthAVS, AuthCVV2 |
| 7 | Service | DB | Transaction record | ID, GroupID, AgentID, Amount, AuthGUID, AuthResp, AuthCode, etc. |
| 8 | Service | Handler | domain.Transaction | All transaction fields |
| 9 | Handler | POS | PaymentResponse | transaction_id, group_id, agent_id, amount, auth_code, auth_guid, auth_card_type |

