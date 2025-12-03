# EPX Payment Gateway Certification Sheet

| | |
|---|---|
| **Version** | 5.0 |
| **Date** | 2025-12-03 |
| **Environment** | EPX UAP Staging |
| **Status** | CERTIFIED |

---

## Quick Reference

### Merchant Credentials
```
CUST_NBR=9001  MERCH_NBR=900300  DBA_NBR=2  TERMINAL_NBR=77
```

### EPX Endpoints
| Environment | Key Exchange | Browser POST | Server POST |
|-------------|--------------|--------------|-------------|
| **Staging** | `https://keyexch.epxuap.com` | `https://services.epxuap.com/browserpost/` | `https://secure.epxuap.com` |
| Production | `https://keyexch.epxnow.com` | `https://services.epxnow.com/browserpost/` | `https://epxnow.com/epx/server_post` |

---

## Table of Contents

1. [Browser POST Credit Card](#1-browser-post-credit-card)
   - 1.1 [AUTH (CCE2)](#11-credit-card-auth-cce2)
   - 1.2 [STORAGE (CCE8)](#12-credit-card-storage-cce8)
   - 1.3 [SALE (CCE1)](#13-credit-card-sale-cce1)
2. [Browser POST ACH](#2-browser-post-ach)
   - 2.1 [ACH Checking Storage (ACH_STORAGE_C)](#21-ach-checking-storage-ach_storage_c)
   - 2.2 [ACH Savings Storage (ACH_STORAGE_S)](#22-ach-savings-storage-ach_storage_s)
3. [Server POST Credit Card](#3-server-post-credit-card)
   - 3.1 [AUTH (CCE2)](#31-credit-card-auth-cce2)
   - 3.2 [CAPTURE (CCE3)](#32-credit-card-capture-cce3)
   - 3.3 [VOID (CCEX)](#33-credit-card-void-ccex)
   - 3.4 [SALE (CCE1)](#34-credit-card-sale-cce1)
   - 3.5 [REFUND (CCE9)](#35-credit-card-refund-cce9)
   - 3.6 [Recurring Billing (RB)](#36-recurring-billing-cce1--aci_extrb)
   - 3.7 [Recurring Retry (RS)](#37-recurring-retry-cce1--aci_extrs)
4. [Server POST ACH](#4-server-post-ach)
   - 4.1 [ACH Sale (CKC2)](#41-ach-sale-ckc2)
   - 4.2 [ACH Refund (CKC3)](#42-ach-refund-ckc3)
   - 4.3 [ACH Pre-note (CKC0)](#43-ach-pre-note-ckc0)
   - 4.4 [ACH Subscription (PPD)](#44-ach-subscription-ckc2--ppd)
5. [Card on File (COF) Compliance](#5-card-on-file-cof-compliance)
6. [Configuration Reference](#6-configuration-reference)

---

## Test Results Summary

| # | Category | Transaction | EPX Code | Amount | Result |
|---|----------|-------------|----------|--------|--------|
| 1 | Browser POST | Credit Card AUTH | CCE2 | $100.00 | PASS |
| 2 | Browser POST | Credit Card STORAGE | CCE8 | $0.00 | PASS |
| 3 | Browser POST | Credit Card SALE | CCE1 | $50.00 | PASS |
| 4 | Browser POST | ACH Checking Storage | CKC8 | $0.00 | PASS |
| 5 | Browser POST | ACH Savings Storage | CKC8 | $0.00 | PASS |
| 6 | Server POST | Credit Card AUTH | CCE2 | $100.00 | PASS |
| 7 | Server POST | Credit Card Capture | CCE3 | $100.00 | PASS |
| 8 | Server POST | Credit Card Void | CCEX | - | PASS |
| 9 | Server POST | Credit Card Sale | CCE1 | $200.00 | PASS |
| 10 | Server POST | Credit Card Refund | CCE9 | $50.00 | PASS |
| 11 | Server POST | Credit Card Recurring (RB) | CCE1 | $200.00 | PASS |
| 12 | Server POST | Credit Card Recurring Retry (RS) | CCE1 | $200.00 | PASS |
| 13 | Server POST | ACH Sale | CKC2 | $25.00 | PASS |
| 14 | Server POST | ACH Refund | CKC3 | $10.00 | PASS |
| 15 | Server POST | ACH Pre-note | CKC0 | $0.00 | PASS |
| 16 | Server POST | ACH Recurring (PPD) | CKC2 | $29.99 | PASS |

---

## 1. Browser POST Credit Card

Browser POST follows a three-step flow:
1. **Key Exchange** (backend to EPX): Get TAC token
2. **Form POST** (browser to EPX): Submit card data
3. **Callback** (EPX to backend): Receive result

---

### 1.1 Credit Card AUTH (CCE2)

**Purpose**: Authorize only (hold funds for later capture)

#### Key Exchange Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000005" \
  -d "AMOUNT=100.00" \
  -d "TRAN_GROUP=AUTH" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback?transaction_type=AUTH"
```

#### Key Exchange Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">GDFa9LcjLjhtXLBTAsAdug==|hJ9kPQ2n...</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000005</FIELD>
    <FIELD KEY="AMOUNT">100.00</FIELD>
    <FIELD KEY="TRAN_GROUP">AUTH</FIELD>
    <FIELD KEY="INDUSTRY_TYPE">E</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Browser POST Form

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- Hidden fields from Key Exchange -->
  <input type="hidden" name="TAC" value="GDFa9LcjLjhtXLBTAsAdug==|hJ9kPQ2n...">
  <input type="hidden" name="TRAN_CODE" value="AUTH">
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">
  <input type="hidden" name="AMOUNT" value="100.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">

  <!-- Card Information -->
  <input type="text" name="ACCOUNT_NBR" value="4111111111111111">
  <input type="text" name="EXP_DATE" value="1225">
  <input type="text" name="CVV2" value="123">

  <!-- Billing Address -->
  <input type="text" name="FIRST_NAME" value="John">
  <input type="text" name="LAST_NAME" value="Doe">
  <input type="text" name="ADDRESS" value="123 Main St">
  <input type="text" name="CITY" value="New York">
  <input type="text" name="STATE" value="NY">
  <input type="text" name="ZIP_CODE" value="10001">

  <button type="submit">Submit Payment</button>
</form>
```

#### Callback Response

```
HTTP/1.1 302 Found
Location: http://localhost:8081/api/v1/payments/browser-post/callback?
  transaction_id=77744f18-a0a2-4533-9da2-4153fba27d00&
  merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&
  transaction_type=AUTH&
  AUTH_RESP=00&
  AUTH_CODE=023689&
  AUTH_GUID=0A1MZWVV6QBFFJ4KFA6&
  AUTH_RESP_TEXT=EXACT+MATCH&
  AUTH_AMOUNT=100.00&
  TRAN_NBR=860091955&
  AUTH_CARD_TYPE=V&
  AUTH_MASKED_ACCOUNT_NBR=************1111&
  AUTH_AVS=Y&
  AUTH_CVV2=M&
  TRAN_TYPE=CCE2
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_CODE | 023689 | Authorization code |
| AUTH_GUID | 0A1MZWVV6QBFFJ4KFA6 | BRIC token for capture |
| AUTH_CARD_TYPE | V | Visa |
| AUTH_AVS | Y | Address verified |
| AUTH_CVV2 | M | CVV matched |
| TRAN_TYPE | CCE2 | Auth transaction |

---

### 1.2 Credit Card STORAGE (CCE8)

**Purpose**: Tokenize card for future use (no charge)

#### Key Exchange Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000004" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=STORAGE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback?transaction_type=STORAGE"
```

#### Key Exchange Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">kFkemaYUVOelTkFG0Vq6Ag==|xY7mNR3...</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000004</FIELD>
    <FIELD KEY="AMOUNT">0.00</FIELD>
    <FIELD KEY="TRAN_GROUP">STORAGE</FIELD>
    <FIELD KEY="INDUSTRY_TYPE">E</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Browser POST Form

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- Hidden fields from Key Exchange -->
  <input type="hidden" name="TAC" value="kFkemaYUVOelTkFG0Vq6Ag==|xY7mNR3...">
  <input type="hidden" name="TRAN_CODE" value="STORAGE">
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">

  <!-- Card Information -->
  <input type="text" name="ACCOUNT_NBR" value="4111111111111111">
  <input type="text" name="EXP_DATE" value="1225">
  <input type="text" name="CVV2" value="123">

  <!-- Billing Address -->
  <input type="text" name="FIRST_NAME" value="John">
  <input type="text" name="LAST_NAME" value="Doe">
  <input type="text" name="ADDRESS" value="123 Main St">
  <input type="text" name="CITY" value="New York">
  <input type="text" name="STATE" value="NY">
  <input type="text" name="ZIP_CODE" value="10001">

  <button type="submit">Save Card</button>
</form>
```

#### Callback Response

```
HTTP/1.1 302 Found
Location: http://localhost:8081/api/v1/payments/browser-post/callback?
  transaction_id=88855f29-b1b3-4644-0eb3-5264gcb38e11&
  merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&
  transaction_type=STORAGE&
  AUTH_RESP=00&
  AUTH_GUID=0A1MRRUN0VA1LEVHW71&
  AUTH_RESP_TEXT=APPROVED&
  AUTH_AMOUNT=0.00&
  TRAN_NBR=860091954&
  AUTH_CARD_TYPE=V&
  AUTH_MASKED_ACCOUNT_NBR=************1111&
  TRAN_TYPE=CCE8
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 0A1MRRUN0VA1LEVHW71 | BRIC token (store for future use) |
| AUTH_CARD_TYPE | V | Visa |
| TRAN_TYPE | CCE8 | Storage transaction |

**Note**: Store `AUTH_GUID` as BRIC token for future Card-on-File transactions.

---

### 1.3 Credit Card SALE (CCE1)

**Purpose**: Authorize + capture in one step

#### Key Exchange Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000006" \
  -d "AMOUNT=50.00" \
  -d "TRAN_GROUP=SALE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback?transaction_type=SALE"
```

#### Key Exchange Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">mNbVcXsWqEpLkJhGfDsA==|zR4oPS5...</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000006</FIELD>
    <FIELD KEY="AMOUNT">50.00</FIELD>
    <FIELD KEY="TRAN_GROUP">SALE</FIELD>
    <FIELD KEY="INDUSTRY_TYPE">E</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Browser POST Form

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- Hidden fields from Key Exchange -->
  <input type="hidden" name="TAC" value="mNbVcXsWqEpLkJhGfDsA==|zR4oPS5...">
  <input type="hidden" name="TRAN_CODE" value="SALE">
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">
  <input type="hidden" name="AMOUNT" value="50.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">

  <!-- Card Information -->
  <input type="text" name="ACCOUNT_NBR" value="4111111111111111">
  <input type="text" name="EXP_DATE" value="1225">
  <input type="text" name="CVV2" value="123">

  <!-- Billing Address -->
  <input type="text" name="FIRST_NAME" value="John">
  <input type="text" name="LAST_NAME" value="Doe">
  <input type="text" name="ADDRESS" value="123 Main St">
  <input type="text" name="CITY" value="New York">
  <input type="text" name="STATE" value="NY">
  <input type="text" name="ZIP_CODE" value="10001">

  <button type="submit">Pay Now</button>
</form>
```

#### Callback Response

```
HTTP/1.1 302 Found
Location: http://localhost:8081/api/v1/payments/browser-post/callback?
  transaction_id=77744f18-a0a2-4533-9da2-4153fba27d00&
  merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&
  transaction_type=SALE&
  AUTH_RESP=00&
  AUTH_CODE=023667&
  AUTH_GUID=0A1MZWVUH4M2JPVDFA5&
  AUTH_RESP_TEXT=EXACT+MATCH&
  AUTH_AMOUNT=50.00&
  TRAN_NBR=860091956&
  AUTH_CARD_TYPE=V&
  AUTH_MASKED_ACCOUNT_NBR=************1111&
  AUTH_AVS=Y&
  AUTH_CVV2=M&
  TRAN_TYPE=CCE1
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_CODE | 023667 | Authorization code |
| AUTH_GUID | 0A1MZWVUH4M2JPVDFA5 | Transaction reference |
| AUTH_CARD_TYPE | V | Visa |
| AUTH_AVS | Y | Address verified |
| AUTH_CVV2 | M | CVV matched |
| TRAN_TYPE | CCE1 | Sale transaction |

---

## 2. Browser POST ACH

### 2.1 ACH Checking Storage (ACH_STORAGE_C)

**Purpose**: Tokenize checking account + automatic prenote

#### Key Exchange Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000020" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=ACH_STORAGE_C" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback?transaction_type=ACH_STORAGE_C"
```

#### Key Exchange Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">pQrStUvWxYzAbCdEfG==|hI8jKL9...</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000020</FIELD>
    <FIELD KEY="AMOUNT">0.00</FIELD>
    <FIELD KEY="TRAN_GROUP">ACH_STORAGE_C</FIELD>
    <FIELD KEY="INDUSTRY_TYPE">E</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Browser POST Form

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- Hidden fields from Key Exchange -->
  <input type="hidden" name="TAC" value="pQrStUvWxYzAbCdEfG==|hI8jKL9...">
  <input type="hidden" name="TRAN_CODE" value="ACH_STORAGE_C">
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">
  <input type="hidden" name="STD_ENTRY_CLASS" value="WEB">

  <!-- Bank Account Information -->
  <input type="text" name="ACCOUNT_NBR" value="123456789">
  <input type="text" name="ROUTING_NBR" value="011000015">

  <!-- Account Holder -->
  <input type="text" name="FIRST_NAME" value="John">
  <input type="text" name="LAST_NAME" value="Doe">

  <button type="submit">Save Account</button>
</form>
```

#### Callback Response

```
HTTP/1.1 302 Found
Location: http://localhost:8081/api/v1/payments/browser-post/callback?
  transaction_id=99966g30-c2c4-5755-1fc4-6375hdc49f22&
  merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&
  transaction_type=ACH_STORAGE_C&
  AUTH_RESP=00&
  AUTH_GUID=09LMZ0MA7K9LRK414G3&
  AUTH_RESP_TEXT=APPROVED&
  AUTH_AMOUNT=0.00&
  TRAN_NBR=860091957&
  TRAN_TYPE=CKC8
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 09LMZ0MA7K9LRK414G3 | BRIC token (store for future use) |
| TRAN_TYPE | CKC8 | ACH Storage |

**Note**: Prenote (CKC0) is sent automatically by EPX for account verification.

---

### 2.2 ACH Savings Storage (ACH_STORAGE_S)

**Purpose**: Tokenize savings account + automatic prenote

#### Key Exchange Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000021" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=ACH_STORAGE_S" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback?transaction_type=ACH_STORAGE_S"
```

#### Key Exchange Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">qRsTuVwXyZaBcDeFgH==|iJ9kLM0...</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000021</FIELD>
    <FIELD KEY="AMOUNT">0.00</FIELD>
    <FIELD KEY="TRAN_GROUP">ACH_STORAGE_S</FIELD>
    <FIELD KEY="INDUSTRY_TYPE">E</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Browser POST Form

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- Hidden fields from Key Exchange -->
  <input type="hidden" name="TAC" value="qRsTuVwXyZaBcDeFgH==|iJ9kLM0...">
  <input type="hidden" name="TRAN_CODE" value="ACH_STORAGE_S">
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="DBA_NBR" value="2">
  <input type="hidden" name="TERMINAL_NBR" value="77">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="E">
  <input type="hidden" name="STD_ENTRY_CLASS" value="WEB">

  <!-- Bank Account Information -->
  <input type="text" name="ACCOUNT_NBR" value="987654321">
  <input type="text" name="ROUTING_NBR" value="011000015">

  <!-- Account Holder -->
  <input type="text" name="FIRST_NAME" value="John">
  <input type="text" name="LAST_NAME" value="Doe">

  <button type="submit">Save Account</button>
</form>
```

#### Callback Response

```
HTTP/1.1 302 Found
Location: http://localhost:8081/api/v1/payments/browser-post/callback?
  transaction_id=00077h41-d3d5-6866-2gd5-7486ied50g33&
  merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&
  transaction_type=ACH_STORAGE_S&
  AUTH_RESP=00&
  AUTH_GUID=09LMRRTG7ZXAJ7HZ73W&
  AUTH_RESP_TEXT=APPROVED&
  AUTH_AMOUNT=0.00&
  TRAN_NBR=860091958&
  TRAN_TYPE=CKC8
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 09LMRRTG7ZXAJ7HZ73W | BRIC token (store for future use) |
| TRAN_TYPE | CKC8 | ACH Storage |

---

## 3. Server POST Credit Card

Server POST is used for backend-to-backend transactions using stored BRIC tokens.

### Credit Card Transaction Types

| EPX Code | Name | Purpose |
|----------|------|---------|
| CCE2 | Auth | Authorize only (hold funds) |
| CCE3 | Capture | Capture prior auth |
| CCEX | Void | Cancel transaction |
| CCE1 | Sale | Authorize + capture |
| CCE9 | Refund | Return funds |
| CCE8 | Storage | Create BRIC token |

---

### 3.1 Credit Card AUTH (CCE2)

**Purpose**: Authorize and hold funds without capturing

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE2" \
  -d "AMOUNT=100.00" \
  -d "TRAN_NBR=1000000002" \
  -d "ORIG_AUTH_GUID=0A1MRRUN0VA1LEVHW71" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000002</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE2</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">023688</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX1234AUTHGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">EXACT MATCH</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
    <FIELD KEY="AUTH_AVS">Y</FIELD>
    <FIELD KEY="AUTH_CVV2">M</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_CODE | 023688 | Authorization code |
| AUTH_GUID | 0A1MZX1234AUTHGUID | Use this for CAPTURE |

---

### 3.2 Credit Card CAPTURE (CCE3)

**Purpose**: Capture a previously authorized transaction

**Important**: Use the `AUTH_GUID` from the AUTH response as `ORIG_AUTH_GUID`

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE3" \
  -d "AMOUNT=100.00" \
  -d "TRAN_NBR=1000000003" \
  -d "ORIG_AUTH_GUID=0A1MZX1234AUTHGUID" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000003</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE3</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">023688</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX5678CAPTUREGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">CAPTURED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 0A1MZX5678CAPTUREGUID | Capture reference |

---

### 3.3 Credit Card VOID (CCEX)

**Purpose**: Cancel an authorized or captured transaction

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCEX" \
  -d "AMOUNT=100.00" \
  -d "TRAN_NBR=1000000004" \
  -d "ORIG_AUTH_GUID=0A1MZX1234AUTHGUID" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000004</FIELD>
    <FIELD KEY="TRAN_TYPE">CCEX</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX9012VOIDGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">VOIDED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_RESP_TEXT | VOIDED | Transaction voided |

---

### 3.4 Credit Card SALE (CCE1)

**Purpose**: Authorize and capture in one step

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE1" \
  -d "AMOUNT=200.00" \
  -d "TRAN_NBR=1000000005" \
  -d "ORIG_AUTH_GUID=0A1MRRUN0VA1LEVHW71" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000005</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">023690</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZWX9V8YRQHVDFA5</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">EXACT MATCH</FIELD>
    <FIELD KEY="AUTH_AMOUNT">200.00</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
    <FIELD KEY="AUTH_AVS">Y</FIELD>
    <FIELD KEY="AUTH_CVV2">M</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_CODE | 023690 | Authorization code |
| AUTH_GUID | 0A1MZWX9V8YRQHVDFA5 | Transaction reference (use for refund) |

---

### 3.5 Credit Card REFUND (CCE9)

**Purpose**: Return funds to customer

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE9" \
  -d "AMOUNT=50.00" \
  -d "TRAN_NBR=1000000006" \
  -d "ORIG_AUTH_GUID=0A1MZWX9V8YRQHVDFA5" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000006</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE9</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX3456REFUNDGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">REFUNDED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">50.00</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_RESP_TEXT | REFUNDED | Refund processed |

---

### 3.6 Recurring Billing (CCE1 + ACI_EXT=RB)

**Purpose**: First billing attempt in each subscription cycle (Merchant-Initiated Transaction)

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE1" \
  -d "AMOUNT=29.99" \
  -d "TRAN_NBR=1000000007" \
  -d "ORIG_AUTH_GUID=0A1MRRUN0VA1LEVHW71" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E" \
  -d "ACI_EXT=RB"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000007</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">023691</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX7890RECURRINGGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">EXACT MATCH</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
    <FIELD KEY="AUTH_AVS">Y</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| ACI_EXT | RB | Recurring Billing indicator |

---

### 3.7 Recurring Retry (CCE1 + ACI_EXT=RS)

**Purpose**: Retry a declined recurring transaction (within 30-day RS window per Visa/MC rules)

**Important**:
- Use `ACI_EXT=RS` only within 30 days of the original decline
- After 30 days, fall back to `ACI_EXT=RB`

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE1" \
  -d "AMOUNT=29.99" \
  -d "TRAN_NBR=1000000008" \
  -d "ORIG_AUTH_GUID=0A1MRRUN0VA1LEVHW71" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E" \
  -d "ACI_EXT=RS"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">1000000008</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">023692</FIELD>
    <FIELD KEY="AUTH_GUID">0A1MZX1122RETRYGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">EXACT MATCH</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************1111</FIELD>
    <FIELD KEY="AUTH_AVS">Y</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| ACI_EXT | RS | Resubmission (retry) indicator |

---

## 4. Server POST ACH

### ACH Transaction Types

| EPX Code | Name | Purpose |
|----------|------|---------|
| CKC2 | Debit | Collect payment from customer |
| CKC3 | Credit | Send refund to customer |
| CKC0 | Pre-note | Account verification ($0.00) |
| CKC8 | Storage | Create BRIC token |
| CKCX | Void | Cancel pending transaction |

### STD_ENTRY_CLASS (SEC Codes)

| Code | Name | When to Use |
|------|------|-------------|
| **WEB** | Internet-Initiated | Customer authorized online (default) |
| **PPD** | Prearranged Payment | Recurring/subscription billing |

---

### 4.1 ACH Sale (CKC2)

**Purpose**: Collect payment from customer's bank account

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CKC2" \
  -d "AMOUNT=25.00" \
  -d "TRAN_NBR=2000000004" \
  -d "ORIG_AUTH_GUID=09LMRY0HLBMU39KUE2E" \
  -d "STD_ENTRY_CLASS=WEB" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000004</FIELD>
    <FIELD KEY="TRAN_TYPE">CKC2</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">09LMRY0X3DRM8H9UE2Y</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">25.00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="AUTH_ROUTING_NBR">011000015</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 09LMRY0X3DRM8H9UE2Y | Transaction reference (use for refund) |

---

### 4.2 ACH Refund (CKC3)

**Purpose**: Return funds to customer's bank account

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CKC3" \
  -d "AMOUNT=10.00" \
  -d "TRAN_NBR=2000000005" \
  -d "ORIG_AUTH_GUID=09LMRY0X3DRM8H9UE2Y" \
  -d "STD_ENTRY_CLASS=WEB" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000005</FIELD>
    <FIELD KEY="TRAN_TYPE">CKC3</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">09LMRY2263ENGBENER3</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">10.00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="AUTH_ROUTING_NBR">011000015</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| AUTH_GUID | 09LMRY2263ENGBENER3 | Refund reference |

---

### 4.3 ACH Pre-note (CKC0)

**Purpose**: Verify bank account validity without charging ($0.00 transaction)

**Note**: Prenote is triggered after Browser POST ACH Storage using the returned BRIC token.

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CKC0" \
  -d "AMOUNT=0.00" \
  -d "TRAN_NBR=2000000003" \
  -d "ORIG_AUTH_GUID=09LMZ0MA7K9LRK414G3" \
  -d "STD_ENTRY_CLASS=WEB" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000003</FIELD>
    <FIELD KEY="TRAN_TYPE">CKC0</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">09LMRYWDNVU659E8J65</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">0.00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="AUTH_ROUTING_NBR">011000015</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Account verified |
| AUTH_GUID | 09LMRYWDNVU659E8J65 | Prenote reference |
| CARD_ENT_METH | Z | BRIC token from ACH Storage |

**Note**: ACH transactions do not use ACI_EXT for MIT classification. The STD_ENTRY_CLASS (WEB) indicates the original authorization method.

---

### 4.4 ACH Subscription (CKC2 + PPD)

**Purpose**: Recurring ACH debit for subscription billing

**Note**: Use `STD_ENTRY_CLASS=PPD` for recurring/prearranged ACH payments

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CKC2" \
  -d "AMOUNT=29.99" \
  -d "TRAN_NBR=2000000007" \
  -d "ORIG_AUTH_GUID=09LMRY0HLBMU39KUE2E" \
  -d "STD_ENTRY_CLASS=PPD" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_NBR">2000000007</FIELD>
    <FIELD KEY="TRAN_TYPE">CKC2</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_GUID">09LMRY3344SUBSCRIPTIONGUID</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVED</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="AUTH_ROUTING_NBR">011000015</FIELD>
  </FIELDS>
</RESPONSE>
```

| Field | Value | Description |
|-------|-------|-------------|
| AUTH_RESP | 00 | Approved |
| STD_ENTRY_CLASS | PPD | Prearranged Payment indicator |

**Note**: ACH subscription retries use the same `STD_ENTRY_CLASS=PPD`. Unlike credit cards, ACH does not use ACI_EXT for retry tracking.

---

## 5. Card on File (COF) Compliance

### Transaction Classification

| Type | Description | ACI_EXT | Example |
|------|-------------|---------|---------|
| **CIT** | Customer-Initiated | None | Online checkout |
| **MIT** | Merchant-Initiated | Required | Recurring billing, retries |

### ACI_EXT Values

| Code | Name | When to Use |
|------|------|-------------|
| **RB** | Recurring Billing | First billing attempt each cycle |
| **RS** | Resubmission | Retry of declined transaction (within 30 days) |

### Subscription Billing Flow

```
Step 1: STORAGE (CIT)          Step 2: First Purchase (CIT)
---------------------          --------------------------
Customer stores card           Customer completes checkout
No ACI_EXT                     No ACI_EXT
Returns: AUTH_GUID (BRIC)      Uses: ORIG_AUTH_GUID=<BRIC>

Step 3: Recurring Billing (MIT)     Step 4: Payment Retry (MIT)
-------------------------------     ---------------------------
Automated subscription charge       Retry after soft decline
ACI_EXT=RB                          ACI_EXT=RS (if within 30 days)
Uses: ORIG_AUTH_GUID=<BRIC>         Falls back to RB if window expired
```

### BRIC Token Chain

| Transaction | ORIG_AUTH_GUID Source | Notes |
|-------------|----------------------|-------|
| SALE (CCE1) | Storage BRIC | First transaction using stored card |
| AUTH (CCE2) | Storage BRIC | Authorization using stored card |
| CAPTURE (CCE3) | AUTH's AUTH_GUID | Links capture to specific auth |
| REFUND (CCE9) | Sale/Capture AUTH_GUID | Links refund to original charge |
| VOID (CCEX) | Transaction's AUTH_GUID | Voids specific transaction |

---

## 6. Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_ACH_CLASS` | `WEB` | Default STD_ENTRY_CLASS for ACH transactions |
| `RS_WINDOW_DAYS` | `30` | Days within which ACI_EXT=RS can be used |

### CARD_ENT_METH Values

| Value | Meaning | Use Case |
|-------|---------|----------|
| `E` | Key-entered | Browser POST card entry |
| `Z` | BRIC token | Server POST with stored credential |
| `X` | ACH direct entry | ACH prenote verification |

### INDUSTRY_TYPE Values

| Value | Meaning |
|-------|---------|
| `E` | E-commerce (online transactions) |

---

**Document Version**: 5.0 | **Last Updated**: 2025-12-03 | **Status**: CERTIFIED
