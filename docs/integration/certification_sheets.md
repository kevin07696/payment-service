# EPX Payment Gateway Certification Sheet

## Quick Reference

### Merchant Credentials
```
CUST_NBR=9001  MERCH_NBR=900300  DBA_NBR=2  TERMINAL_NBR=77
```

### EPX Endpoints

**Staging (defaults)**:
| Service | Endpoint |
|---------|----------|
| Key Exchange | `https://keyexch.epxuap.com` |
| Browser POST | `https://services.epxuap.com/browserpost/` |
| Server POST | `https://secure.epxuap.com` |
| Server POST Socket | `secure.epxuap.com:8087` |

**Production Environment Variables**:
| Variable | Description |
|----------|-------------|
| `EPX_KEY_EXCHANGE_ENDPOINT` | Key Exchange endpoint |
| `EPX_BROWSER_POST_ENDPOINT` | Browser POST endpoint |
| `EPX_SERVER_POST_ENDPOINT` | Server POST HTTPS endpoint |
| `EPX_SERVER_POST_SOCKET_ENDPOINT` | Server POST XML Socket (host:port) |

> **Note**: Production endpoints are configured via environment variables. The service will **fail to start** if any required endpoint is missing in production. Contact North for production endpoints during certification.

---

## Table of Contents

1. [Browser POST Credit Card](#1-browser-post-credit-card)
   - 1.1 [AUTH (CCE2)](#11-credit-card-auth-cce2)
   - 1.2 [STORAGE (CCE8)](#12-credit-card-storage-cce8)
   - 1.3 [SALE (CCE1)](#13-credit-card-sale-cce1)
2. [Browser POST ACH](#2-browser-post-ach)
   - 2.1 [ACH Checking Storage (CKC8)](#21-ach-checking-storage-ckc8)
   - 2.2 [ACH Savings Storage (CKS8)](#22-ach-savings-storage-cks8)
3. [Server POST Credit Card](#3-server-post-credit-card)
   - 3.1 [AUTH (CCE2)](#31-credit-card-auth-cce2)
   - 3.2 [CAPTURE (CCE4)](#32-credit-card-capture-cce4)
   - 3.3 [VOID (CCEX)](#33-credit-card-void-ccex)
   - 3.4 [SALE (CCE1)](#34-credit-card-sale-cce1)
   - 3.5 [REFUND (CCE9)](#35-credit-card-refund-cce9)
   - 3.6 [REVERSAL (CCE7)](#36-credit-card-reversal-cce7)
   - 3.7 [Recurring Billing (RB)](#37-recurring-billing-cce1--aci_extrb)
   - 3.8 [Recurring Retry (RS)](#38-recurring-retry-cce1--aci_extrs)
4. [Server POST ACH](#4-server-post-ach)
   - 4.1 [ACH Sale (CKC2)](#41-ach-sale-ckc2)
   - 4.2 [ACH Refund (CKC3)](#42-ach-refund-ckc3)
   - 4.3 [ACH Pre-note (CKC0)](#43-ach-pre-note-ckc0)
   - 4.4 [ACH Subscription (PPD)](#44-ach-subscription-ckc2--ppd)
5. [Payment Service REST API](#5-payment-service-rest-api)
   - 5.1 [Get Form Config](#51-get-form-config)
   - 5.2 [Browser POST HTML Forms](#52-browser-post-html-forms)
     - 5.2.1 [Credit Card Payment Form (SALE/AUTH)](#521-credit-card-payment-form-saleauth)
     - 5.2.2 [Credit Card Storage Form (STORAGE)](#522-credit-card-storage-form-storage)
     - 5.2.3 [ACH Bank Account Storage Form](#523-ach-bank-account-storage-form-ach_storage_cach_storage_s)
   - 5.3 [Callback HTML Responses](#53-callback-html-responses)
     - 5.3.1 [Payment Receipt (SALE/AUTH)](#531-payment-receipt-saleauth)
     - 5.3.2 [Credit Card Stored (STORAGE)](#532-credit-card-stored-storage)
     - 5.3.3 [Bank Account Stored (ACH_STORAGE)](#533-bank-account-stored-ach_storage)
     - 5.3.4 [Error](#534-error)
6. [Card on File (COF) Compliance](#6-card-on-file-cof-compliance)
7. [Configuration Reference](#7-configuration-reference)

---

## 1. Browser POST Credit Card

Browser POST follows a three-step flow:
1. **Key Exchange** (backend to EPX): Get TAC token
2. **Form POST** (browser to EPX): Submit card data
3. **Callback** (EPX to backend): Receive result

---

### 1.1 Credit Card AUTH (CCE2)

**Purpose**: Authorize only (hold funds for later capture)

#### Step 1: Key Exchange (Get TAC)

```bash
# Capture TAC in variable for use in Browser POST
TAC=$(curl -s -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000011" \
  -d "AMOUNT=100.00" \
  -d "TRAN_GROUP=AUTH" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" | grep -oP '(?<=KEY="TAC">)[^<]+')

echo "TAC: $TAC"
```

#### Key Exchange Response

```xml
<RESPONSE><FIELDS><FIELD KEY="TAC">svM11dEbyhguvn5w8CuTPQ==|xPX24MwXfbMy262rkTF4lPe0qbzO84WBLHBAvJDvJU8Vpz0YrQ5YxWcGY137a/trxtcXJDTDiM0sl1PU4st6b4X3M5tbAWFnL/a6K7NkIKjn2BSuN50Mxiyf6sgDc8rUvGelklLWc0OPd4u2+QfGq7iAdgUupiG6m5ohNihFMWfq4lDlmVQvs2qwniZLyGANgSWNq+XGwgex8wMK8dbLXQ==</FIELD></FIELDS></RESPONSE>
```

#### Step 2: Browser POST (Submit Card Data)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -c cookies.txt \
  --data-urlencode "TAC=$TAC" \
  -d "TRAN_CODE=AUTH" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "AMOUNT=100.00" \
  -d "INDUSTRY_TYPE=E" \
  -d "ACCOUNT_NBR=************0002" \
  -d "EXP_DATE=2712" \
  -d "CVV2=123" \
  -d "FIRST_NAME=John" \
  -d "LAST_NAME=Doe" \
  -d "ADDRESS=123 Main St" \
  -d "CITY=New York" \
  -d "STATE=NY" \
  -d "ZIP_CODE=10001"
```

#### Step 3: Get Callback Response

```bash
curl -b cookies.txt "https://services.epxuap.com/browserpost/response.php"
```

#### Callback Response (HTML)

```html
<html>
<head>
  <meta http-equiv='Content-type' content='text/html;charset=utf-8'>
</head>
<body onload='document.frm.submit()'>
  <form action='http://localhost:8081/api/v1/payments/browser-post/callback' method='post' name='frm' accept-charset='utf-8'>
    <input type='hidden' name='MSG_VERSION' value='003'>
    <input type='hidden' name='CUST_NBR' value='9001'>
    <input type='hidden' name='MERCH_NBR' value='900300'>
    <input type='hidden' name='DBA_NBR' value='2'>
    <input type='hidden' name='TERMINAL_NBR' value='77'>
    <input type='hidden' name='TRAN_TYPE' value='CCE2'>
    <input type='hidden' name='TRAN_NBR' value='2000000011'>
    <input type='hidden' name='LOCAL_DATE' value='120325'>
    <input type='hidden' name='LOCAL_TIME' value='191136'>
    <input type='hidden' name='AUTH_GUID' value='0A1MZFGY26XX341TFRN'>
    <input type='hidden' name='AUTH_RESP' value='00'>
    <input type='hidden' name='AUTH_CODE' value='045023'>
    <input type='hidden' name='AUTH_AVS' value='A'>
    <input type='hidden' name='AUTH_CVV2' value='M'>
    <input type='hidden' name='AUTH_RESP_TEXT' value='ADDRESS MATCH'>
    <input type='hidden' name='AUTH_CARD_TYPE' value='V'>
    <input type='hidden' name='AUTH_TRAN_DATE_GMT' value='12/04/2025 12:11:36 AM'>
    <input type='hidden' name='AUTH_AMOUNT_REQUESTED' value='100.00'>
    <input type='hidden' name='AUTH_AMOUNT' value='100.00'>
    <input type='hidden' name='AUTH_CURRENCY_CODE' value='840'>
    <input type='hidden' name='NETWORK_RESPONSE' value='00'>
    <input type='hidden' name='AUTH_CARD_COUNTRY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_CURRENCY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_B' value='D'>
    <input type='hidden' name='AUTH_CARD_C' value='F'>
    <input type='hidden' name='AUTH_CARD_E' value='N'>
    <input type='hidden' name='AUTH_CARD_F' value='Y'>
    <input type='hidden' name='AUTH_CARD_G' value='N'>
    <input type='hidden' name='AUTH_CARD_I' value='Y'>
    <input type='hidden' name='AUTH_MASKED_ACCOUNT_NBR' value='************0002'>
    <input type='hidden' name='AUTH_CARD_L' value='P'>
    <input type='hidden' name='ORIG_TRAN_TYPE' value='CCE2'>
    <input type='hidden' name='AUTH_TRAN_IDENT' value='355338006962896'>
    <input type='hidden' name='AUTH_PAR' value='V40000000028FAB8191EEC1C39808'>
  </form>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CCE2 | Transaction type: CCE1=Sale, CCE2=Auth, CCE4=Capture, CCE9=Refund, CCEX=Void, CCE8=Storage |
| TRAN_NBR | 2000000011 | Your transaction reference number (echo back) |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 191136 | Local time (HHMMSS format) |
| AUTH_GUID | 0A1MZFGY26XX341TFRN | BRIC token - use as ORIG_AUTH_GUID for CAPTURE/VOID/REFUND |
| AUTH_RESP | 00 | Response code: 00=Approved, 05=Declined, 51=Insufficient Funds, 54=Expired Card, 61=Exceeds Limit, RR=Referral |
| AUTH_CODE | 045023 | 6-digit authorization code from card issuer (only on approval) |
| AUTH_AVS | A | AVS result: Y=Full Match, A=Address Match Only, Z=ZIP Match Only, N=No Match, U=Unavailable, R=Retry, S=Not Supported |
| AUTH_CVV2 | M | CVV result: M=Match, N=No Match, P=Not Processed, S=Should be present, U=Issuer Unable, X=No Response |
| AUTH_RESP_TEXT | ADDRESS MATCH | Human-readable response message |
| AUTH_CARD_TYPE | V | Card brand: V=Visa, M=Mastercard, A=Amex, D=Discover, J=JCB, C=Diners |
| AUTH_TRAN_DATE_GMT | 12/04/2025 12:11:36 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 100.00 | Original amount requested |
| AUTH_AMOUNT | 100.00 | Actual amount authorized (may differ for partial auth) |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD, 124=CAD, 484=MXN |
| NETWORK_RESPONSE | 00 | Raw response from card network |
| AUTH_CARD_COUNTRY_CODE | 840 | Card issuing country (ISO 3166): 840=USA, 124=Canada |
| AUTH_CARD_CURRENCY_CODE | 840 | Card's billing currency |
| AUTH_CARD_B | D | Product type: C=Credit, D=Debit, P=Prepaid, H=Charge |
| AUTH_CARD_C | F | Card level: F=Classic, G=Gold, P=Platinum, S=Signature, I=Infinite |
| AUTH_CARD_E | N | Commercial card: Y=Yes (corporate/business), N=No (consumer) |
| AUTH_CARD_F | Y | Signature debit capable: Y=Yes, N=No |
| AUTH_CARD_G | N | PIN-less debit capable: Y=Yes, N=No |
| AUTH_CARD_I | Y | Issuer participates in auth service: Y=Yes, N=No |
| AUTH_MASKED_ACCOUNT_NBR | ************0002 | Masked PAN showing last 4 digits only |
| AUTH_CARD_L | P | Visa product level: P=Premium, T=Traditional, empty=N/A |
| ORIG_TRAN_TYPE | CCE2 | Original transaction type (echoed back) |
| AUTH_TRAN_IDENT | 355338006962896 | Network transaction ID (use for disputes/chargebacks) |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference - persistent token ID across card networks |

</details>

---

### 1.2 Credit Card STORAGE (CCE8)

**Purpose**: Tokenize card for future use (no charge)

#### Step 1: Key Exchange (Get TAC)

```bash
# Capture TAC in variable for use in Browser POST
TAC=$(curl -s -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000012" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=STORAGE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" | grep -oP '(?<=KEY="TAC">)[^<]+')

echo "TAC: $TAC"
```

#### Key Exchange Response

```xml
<RESPONSE><FIELDS><FIELD KEY="TAC">s1b1iWsNp4oq2tb3nRzSbQ==|xnbFANnu/kZ3H/uk8pWGX3Xtpt73WzpUpoZpUi81AkPldMiWP6rHaTSKC2VBhB5DDRIcQQwoFAiR3tVY7mvopkKEbL19zXECep/nXiSbIDUXJ6tQZL0Q/Po5Vq7M0xRye0upCtK2n94HxdY9BH0omsJuzYdJfSJJ/oJ7PjhELVv/0GdCoFFkLiG/O6BpQ3AEhvw08adrKzz9TwTYFRO+ZQ==</FIELD></FIELDS></RESPONSE>
```

#### Step 2: Browser POST (Submit Card Data)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -c cookies.txt \
  --data-urlencode "TAC=$TAC" \
  -d "TRAN_CODE=STORAGE" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "AMOUNT=0.00" \
  -d "INDUSTRY_TYPE=E" \
  -d "ACCOUNT_NBR=************0002" \
  -d "EXP_DATE=2712" \
  -d "CVV2=123" \
  -d "FIRST_NAME=John" \
  -d "LAST_NAME=Doe" \
  -d "ADDRESS=123 Main St" \
  -d "CITY=New York" \
  -d "STATE=NY" \
  -d "ZIP_CODE=10001"
```

#### Step 3: Get Callback Response

```bash
curl -b cookies.txt "https://services.epxuap.com/browserpost/response.php"
```

#### Callback Response (HTML)

```html
<html>
<head>
  <meta http-equiv='Content-type' content='text/html;charset=utf-8'>
</head>
<body onload='document.frm.submit()'>
  <form action='http://localhost:8081/api/v1/payments/browser-post/callback' method='post' name='frm' accept-charset='utf-8'>
    <input type='hidden' name='MSG_VERSION' value='003'>
    <input type='hidden' name='CUST_NBR' value='9001'>
    <input type='hidden' name='MERCH_NBR' value='900300'>
    <input type='hidden' name='DBA_NBR' value='2'>
    <input type='hidden' name='TERMINAL_NBR' value='77'>
    <input type='hidden' name='TRAN_TYPE' value='CCE8'>
    <input type='hidden' name='TRAN_NBR' value='2000000012'>
    <input type='hidden' name='LOCAL_DATE' value='120325'>
    <input type='hidden' name='LOCAL_TIME' value='200734'>
    <input type='hidden' name='AUTH_GUID' value='0A1MZFJUHWT9REHXFRR'>
    <input type='hidden' name='AUTH_RESP' value='00'>
    <input type='hidden' name='AUTH_CODE' value='045464'>
    <input type='hidden' name='AUTH_AVS' value='A'>
    <input type='hidden' name='AUTH_CVV2' value='M'>
    <input type='hidden' name='AUTH_RESP_TEXT' value='APPROVAL 045464'>
    <input type='hidden' name='AUTH_CARD_TYPE' value='V'>
    <input type='hidden' name='AUTH_TRAN_DATE_GMT' value='12/04/2025 01:07:34 AM'>
    <input type='hidden' name='AUTH_AMOUNT_REQUESTED' value='0.00'>
    <input type='hidden' name='AUTH_AMOUNT' value='0.00'>
    <input type='hidden' name='AUTH_CURRENCY_CODE' value='840'>
    <input type='hidden' name='NETWORK_RESPONSE' value='00'>
    <input type='hidden' name='AUTH_CARD_COUNTRY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_CURRENCY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_B' value='D'>
    <input type='hidden' name='AUTH_CARD_C' value='F'>
    <input type='hidden' name='AUTH_CARD_E' value='N'>
    <input type='hidden' name='AUTH_CARD_F' value='Y'>
    <input type='hidden' name='AUTH_CARD_G' value='N'>
    <input type='hidden' name='AUTH_CARD_I' value='Y'>
    <input type='hidden' name='AUTH_MASKED_ACCOUNT_NBR' value='************0002'>
    <input type='hidden' name='AUTH_CARD_L' value='P'>
    <input type='hidden' name='ORIG_TRAN_TYPE' value='CCE8'>
    <input type='hidden' name='AUTH_TRAN_IDENT' value='355338040541796'>
    <input type='hidden' name='AUTH_PAR' value='V40000000028FAB8191EEC1C39808'>
  </form>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CCE8 | Transaction type: CCE8=Storage (tokenization only) |
| TRAN_NBR | 2000000012 | Your transaction reference number (echo back) |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 200734 | Local time (HHMMSS format) |
| AUTH_GUID | 0A1MZFJUHWT9REHXFRR | BRIC token - store for future Card-on-File transactions |
| AUTH_RESP | 00 | Response code: 00=Approved, 05=Declined, 51=Insufficient Funds, 54=Expired Card |
| AUTH_CODE | 045464 | 6-digit authorization code (may be present even for $0 storage) |
| AUTH_AVS | A | AVS result: Y=Full Match, A=Address Match Only, Z=ZIP Match Only, N=No Match |
| AUTH_CVV2 | M | CVV result: M=Match, N=No Match, P=Not Processed, U=Unavailable |
| AUTH_RESP_TEXT | APPROVAL 045464 | Human-readable response message |
| AUTH_CARD_TYPE | V | Card brand: V=Visa, M=Mastercard, A=Amex, D=Discover |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:07:34 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 0.00 | Original amount requested ($0 for storage) |
| AUTH_AMOUNT | 0.00 | Actual amount (always $0 for storage) |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from card network |
| AUTH_CARD_COUNTRY_CODE | 840 | Card issuing country (ISO 3166): 840=USA |
| AUTH_CARD_CURRENCY_CODE | 840 | Card's billing currency |
| AUTH_CARD_B | D | Product type: C=Credit, D=Debit, P=Prepaid, H=Charge |
| AUTH_CARD_C | F | Card level: F=Classic, G=Gold, P=Platinum, S=Signature |
| AUTH_CARD_E | N | Commercial card: Y=Yes (corporate), N=No (consumer) |
| AUTH_CARD_F | Y | Signature debit capable: Y=Yes, N=No |
| AUTH_CARD_G | N | PIN-less debit capable: Y=Yes, N=No |
| AUTH_CARD_I | Y | Issuer participates in auth service: Y=Yes, N=No |
| AUTH_MASKED_ACCOUNT_NBR | ************0002 | Masked PAN showing last 4 digits only |
| AUTH_CARD_L | P | Visa product level: P=Premium, T=Traditional |
| ORIG_TRAN_TYPE | CCE8 | Original transaction type (echoed back) |
| AUTH_TRAN_IDENT | 355338040541796 | Network transaction ID |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference - persistent token ID |

</details>

**Note**: Store `AUTH_GUID` as BRIC token for future Card-on-File transactions.

---

### 1.3 Credit Card SALE (CCE1)

**Purpose**: Authorize + capture in one step

#### Step 1: Key Exchange (Get TAC)

```bash
# Capture TAC in variable for use in Browser POST
TAC=$(curl -s -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000013" \
  -d "AMOUNT=50.00" \
  -d "TRAN_GROUP=SALE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" | grep -oP '(?<=KEY="TAC">)[^<]+')

echo "TAC: $TAC"
```

#### Key Exchange Response

```xml
<RESPONSE><FIELDS><FIELD KEY="TAC">tLSEWp3D+YqpkBkRj9+mew==|PS6LpNK8ov91p1PnENJkw831t+WOm3VM6zFMRBH2CTTcrE6zlirCv7+91gdEmV8TYB2cI8JTu8EqbFpWFCvZ4xJv9oz0qXX7kO9AjVa63fb4vCWc0Y2tvPdxKGXDyE6f1SbnJM3m83+vDS6Ea3d3OY+xXcTuT7BSm/TtAWc7Uq7sQi+ma+M2vbSmr5wkjfhs</FIELD></FIELDS></RESPONSE>
```

#### Step 2: Browser POST (Submit Card Data)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -c cookies.txt \
  --data-urlencode "TAC=$TAC" \
  -d "TRAN_CODE=SALE" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "AMOUNT=50.00" \
  -d "INDUSTRY_TYPE=E" \
  -d "ACCOUNT_NBR=************0002" \
  -d "EXP_DATE=2712" \
  -d "CVV2=123" \
  -d "FIRST_NAME=John" \
  -d "LAST_NAME=Doe" \
  -d "ADDRESS=123 Main St" \
  -d "CITY=New York" \
  -d "STATE=NY" \
  -d "ZIP_CODE=10001"
```

#### Step 3: Get Callback Response

```bash
curl -b cookies.txt "https://services.epxuap.com/browserpost/response.php"
```

#### Callback Response (HTML)

```html
<html>
<head>
  <meta http-equiv='Content-type' content='text/html;charset=utf-8'>
</head>
<body onload='document.frm.submit()'>
  <form action='http://localhost:8081/api/v1/payments/browser-post/callback' method='post' name='frm' accept-charset='utf-8'>
    <input type='hidden' name='MSG_VERSION' value='003'>
    <input type='hidden' name='CUST_NBR' value='9001'>
    <input type='hidden' name='MERCH_NBR' value='900300'>
    <input type='hidden' name='DBA_NBR' value='2'>
    <input type='hidden' name='TERMINAL_NBR' value='77'>
    <input type='hidden' name='TRAN_TYPE' value='CCE1'>
    <input type='hidden' name='TRAN_NBR' value='2000000013'>
    <input type='hidden' name='LOCAL_DATE' value='120325'>
    <input type='hidden' name='LOCAL_TIME' value='201019'>
    <input type='hidden' name='AUTH_GUID' value='0A1MZFK3HM2WWMYHFRZ'>
    <input type='hidden' name='AUTH_RESP' value='00'>
    <input type='hidden' name='AUTH_CODE' value='045561'>
    <input type='hidden' name='AUTH_AVS' value='A'>
    <input type='hidden' name='AUTH_CVV2' value='M'>
    <input type='hidden' name='AUTH_RESP_TEXT' value='ADDRESS MATCH'>
    <input type='hidden' name='AUTH_CARD_TYPE' value='V'>
    <input type='hidden' name='AUTH_TRAN_DATE_GMT' value='12/04/2025 01:10:19 AM'>
    <input type='hidden' name='AUTH_AMOUNT_REQUESTED' value='50.00'>
    <input type='hidden' name='AUTH_AMOUNT' value='50.00'>
    <input type='hidden' name='AUTH_CURRENCY_CODE' value='840'>
    <input type='hidden' name='NETWORK_RESPONSE' value='00'>
    <input type='hidden' name='AUTH_CARD_COUNTRY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_CURRENCY_CODE' value='840'>
    <input type='hidden' name='AUTH_CARD_B' value='D'>
    <input type='hidden' name='AUTH_CARD_C' value='F'>
    <input type='hidden' name='AUTH_CARD_E' value='N'>
    <input type='hidden' name='AUTH_CARD_F' value='Y'>
    <input type='hidden' name='AUTH_CARD_G' value='N'>
    <input type='hidden' name='AUTH_CARD_I' value='Y'>
    <input type='hidden' name='AUTH_MASKED_ACCOUNT_NBR' value='************0002'>
    <input type='hidden' name='AUTH_CARD_L' value='P'>
    <input type='hidden' name='ORIG_TRAN_TYPE' value='CCE1'>
    <input type='hidden' name='AUTH_TRAN_IDENT' value='355338042195319'>
    <input type='hidden' name='AUTH_PAR' value='V40000000028FAB8191EEC1C39808'>
  </form>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CCE1 | Transaction type: CCE1=Sale (authorize + capture) |
| TRAN_NBR | 2000000013 | Your transaction reference number (echo back) |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 201019 | Local time (HHMMSS format) |
| AUTH_GUID | 0A1MZFK3HM2WWMYHFRZ | BRIC token - use for VOID or REFUND |
| AUTH_RESP | 00 | Response code: 00=Approved, 05=Declined, 51=Insufficient Funds, 54=Expired Card |
| AUTH_CODE | 045561 | 6-digit authorization code from card issuer |
| AUTH_AVS | A | AVS result: Y=Full Match, A=Address Match Only, Z=ZIP Match Only, N=No Match |
| AUTH_CVV2 | M | CVV result: M=Match, N=No Match, P=Not Processed, U=Unavailable |
| AUTH_RESP_TEXT | ADDRESS MATCH | Human-readable response message |
| AUTH_CARD_TYPE | V | Card brand: V=Visa, M=Mastercard, A=Amex, D=Discover |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:10:19 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 50.00 | Original amount requested |
| AUTH_AMOUNT | 50.00 | Actual amount captured |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from card network |
| AUTH_CARD_COUNTRY_CODE | 840 | Card issuing country (ISO 3166): 840=USA |
| AUTH_CARD_CURRENCY_CODE | 840 | Card's billing currency |
| AUTH_CARD_B | D | Product type: C=Credit, D=Debit, P=Prepaid, H=Charge |
| AUTH_CARD_C | F | Card level: F=Classic, G=Gold, P=Platinum, S=Signature |
| AUTH_CARD_E | N | Commercial card: Y=Yes (corporate), N=No (consumer) |
| AUTH_CARD_F | Y | Signature debit capable: Y=Yes, N=No |
| AUTH_CARD_G | N | PIN-less debit capable: Y=Yes, N=No |
| AUTH_CARD_I | Y | Issuer participates in auth service: Y=Yes, N=No |
| AUTH_MASKED_ACCOUNT_NBR | ************0002 | Masked PAN showing last 4 digits only |
| AUTH_CARD_L | P | Visa product level: P=Premium, T=Traditional |
| ORIG_TRAN_TYPE | CCE1 | Original transaction type (echoed back) |
| AUTH_TRAN_IDENT | 355338042195319 | Network transaction ID |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference - persistent token ID

</details>

---

## 2. Browser POST ACH

> **Note**: ACH Browser POST may require merchant-specific configuration. The sandbox returned `BP_129 TAC validation - Unknown TRAN_GROUP` for ACH transaction types. Contact EPX support to enable ACH Browser POST for your merchant account.

### 2.1 ACH Checking Storage (CKC8)

**Purpose**: Tokenize checking account + automatic prenote

#### Step 1: Key Exchange (Get TAC)

```bash
# Capture TAC in variable for use in Browser POST
TAC=$(curl -s -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000014" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=STORAGE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" | grep -oP '(?<=KEY="TAC">)[^<]+')

echo "TAC: $TAC"
```

#### Step 2: Browser POST (Submit Bank Data)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -c cookies.txt \
  --data-urlencode "TAC=$TAC" \
  -d "TRAN_CODE=ACHSTORAGE_C" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "AMOUNT=0.00" \
  -d "INDUSTRY_TYPE=E" \
  -d "STD_ENTRY_CLASS=WEB" \
  -d "ACCOUNT_NBR=*****6789" \
  -d "ROUTING_NBR=011000015" \
  -d "FIRST_NAME=John" \
  -d "LAST_NAME=Doe"
```

#### Step 3: Get Callback Response

```bash
curl -b cookies.txt "https://services.epxuap.com/browserpost/response.php"
```

#### Callback Response (HTML)

```html
<html>
<head>
  <meta http-equiv='Content-type' content='text/html;charset=utf-8'>
</head>
<body onload='document.frm.submit()'>
  <form action='http://localhost:8081/api/v1/payments/browser-post/callback' method='post' name='frm' accept-charset='utf-8'>
    <input type='hidden' name='MSG_VERSION' value='003'>
    <input type='hidden' name='CUST_NBR' value='9001'>
    <input type='hidden' name='MERCH_NBR' value='900300'>
    <input type='hidden' name='DBA_NBR' value='2'>
    <input type='hidden' name='TERMINAL_NBR' value='77'>
    <input type='hidden' name='TRAN_TYPE' value='CKC8'>
    <input type='hidden' name='TRAN_NBR' value='2000000014'>
    <input type='hidden' name='LOCAL_DATE' value='120325'>
    <input type='hidden' name='LOCAL_TIME' value='202314'>
    <input type='hidden' name='AUTH_GUID' value='0A1MZFKR6U2DPP5BFRT'>
    <input type='hidden' name='AUTH_RESP' value='00'>
    <input type='hidden' name='AUTH_CODE' value='461105'>
    <input type='hidden' name='AUTH_RESP_TEXT' value='ACCEPTED 461105'>
    <input type='hidden' name='AUTH_TRAN_DATE_GMT' value='12/04/2025 01:23:14 AM'>
    <input type='hidden' name='AUTH_AMOUNT_REQUESTED' value='0.00'>
    <input type='hidden' name='AUTH_AMOUNT' value='0.00'>
    <input type='hidden' name='AUTH_CURRENCY_CODE' value='840'>
    <input type='hidden' name='NETWORK_RESPONSE' value='00'>
    <input type='hidden' name='AUTH_MASKED_ACCOUNT_NBR' value='*****6789'>
    <input type='hidden' name='ORIG_TRAN_TYPE' value='CKC8'>
  </form>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CKC8 | Transaction type: CKC8=ACH Storage |
| TRAN_NBR | 2000000014 | Your transaction reference number |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 202314 | Local time (HHMMSS format) |
| AUTH_GUID | 0A1MZFKR6U2DPP5BFRT | BRIC token - store for future ACH transactions |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 461105 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 461105 | Human-readable response message |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:23:14 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 0.00 | Original amount requested ($0 for storage) |
| AUTH_AMOUNT | 0.00 | Actual amount (always $0 for storage) |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from network |
| AUTH_MASKED_ACCOUNT_NBR | *****6789 | Masked bank account showing last 4 digits |
| ORIG_TRAN_TYPE | CKC8 | Original transaction type (echoed back) |

</details>

**Note**: Store `AUTH_GUID` as BRIC token for future ACH transactions. Prenote (CKC0) is sent automatically by EPX.

---

### 2.2 ACH Savings Storage (CKS8)

**Purpose**: Tokenize savings account + automatic prenote

#### Step 1: Key Exchange (Get TAC)

```bash
# Capture TAC in variable for use in Browser POST
TAC=$(curl -s -X POST "https://keyexch.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_NBR=2000000015" \
  -d "AMOUNT=0.00" \
  -d "TRAN_GROUP=STORAGE" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "INDUSTRY_TYPE=E" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" | grep -oP '(?<=KEY="TAC">)[^<]+')

echo "TAC: $TAC"
```

#### Step 2: Browser POST (Submit Bank Data)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -c cookies.txt \
  --data-urlencode "TAC=$TAC" \
  -d "TRAN_CODE=ACHSTORAGE_S" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "AMOUNT=0.00" \
  -d "INDUSTRY_TYPE=E" \
  -d "STD_ENTRY_CLASS=WEB" \
  -d "ACCOUNT_NBR=*****4321" \
  -d "ROUTING_NBR=011000015" \
  -d "FIRST_NAME=John" \
  -d "LAST_NAME=Doe"
```

#### Step 3: Get Callback Response

```bash
curl -b cookies.txt "https://services.epxuap.com/browserpost/response.php"
```

#### Callback Response (HTML)

```html
<html>
<head>
  <meta http-equiv='Content-type' content='text/html;charset=utf-8'>
</head>
<body onload='document.frm.submit()'>
  <form action='http://localhost:8081/api/v1/payments/browser-post/callback' method='post' name='frm' accept-charset='utf-8'>
    <input type='hidden' name='MSG_VERSION' value='003'>
    <input type='hidden' name='CUST_NBR' value='9001'>
    <input type='hidden' name='MERCH_NBR' value='900300'>
    <input type='hidden' name='DBA_NBR' value='2'>
    <input type='hidden' name='TERMINAL_NBR' value='77'>
    <input type='hidden' name='TRAN_TYPE' value='CKS8'>
    <input type='hidden' name='TRAN_NBR' value='2000000015'>
    <input type='hidden' name='LOCAL_DATE' value='120325'>
    <input type='hidden' name='LOCAL_TIME' value='202726'>
    <input type='hidden' name='AUTH_GUID' value='0A1MZFL2T2YTRFBWFRU'>
    <input type='hidden' name='AUTH_RESP' value='00'>
    <input type='hidden' name='AUTH_CODE' value='028709'>
    <input type='hidden' name='AUTH_RESP_TEXT' value='ACCEPTED 028709'>
    <input type='hidden' name='AUTH_TRAN_DATE_GMT' value='12/04/2025 01:27:26 AM'>
    <input type='hidden' name='AUTH_AMOUNT_REQUESTED' value='0.00'>
    <input type='hidden' name='AUTH_AMOUNT' value='0.00'>
    <input type='hidden' name='AUTH_CURRENCY_CODE' value='840'>
    <input type='hidden' name='NETWORK_RESPONSE' value='00'>
    <input type='hidden' name='AUTH_MASKED_ACCOUNT_NBR' value='*****4321'>
    <input type='hidden' name='ORIG_TRAN_TYPE' value='CKS8'>
  </form>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CKS8 | Transaction type: CKS8=ACH Savings Storage |
| TRAN_NBR | 2000000015 | Your transaction reference number |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 202726 | Local time (HHMMSS format) |
| AUTH_GUID | 0A1MZFL2T2YTRFBWFRU | BRIC token - store for future ACH transactions |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 028709 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 028709 | Human-readable response message |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:27:26 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 0.00 | Original amount requested ($0 for storage) |
| AUTH_AMOUNT | 0.00 | Actual amount (always $0 for storage) |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from network |
| AUTH_MASKED_ACCOUNT_NBR | *****4321 | Masked bank account showing last 4 digits |
| ORIG_TRAN_TYPE | CKS8 | Original transaction type (echoed back) |

</details>

**Note**: Store `AUTH_GUID` as BRIC token for future ACH transactions. Prenote (CKC0) is sent automatically by EPX.

---

## 3. Server POST Credit Card

Server POST is used for backend-to-backend transactions using stored BRIC tokens.

### Credit Card Transaction Types

| EPX Code | Name | Purpose |
|----------|------|---------|
| CCE2 | Auth | Authorize only (hold funds) |
| CCE4 | Capture | Capture prior auth |
| CCEX | Void | Cancel transaction (funds held 3-10 days) |
| CCE7 | Reversal | Cancel + release auth hold (immediate fund release) |
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
  -d "TRAN_NBR=1000000020" \
  -d "ORIG_AUTH_GUID=0A1MZFJUHWT9REHXFRR" \
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
    <FIELD KEY="TRAN_TYPE">CCE2</FIELD>
    <FIELD KEY="TRAN_NBR">1000000020</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">205419</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFMK42BDVJGMWW0</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">046075</FIELD>
    <FIELD KEY="AUTH_AVS">A</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ADDRESS MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 01:54:19 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">100.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE2</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355338068594036</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CCE2 | Transaction type: CCE2=Auth |
| TRAN_NBR | 1000000020 | Your transaction reference number |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 205419 | Local time (HHMMSS format) |
| AUTH_GUID | 09LMZFMK42BDVJGMWW0 | **Use this for CAPTURE (section 3.2)** |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 046075 | 6-digit authorization code from issuer |
| AUTH_AVS | A | AVS result: A=Address Match Only |
| AUTH_RESP_TEXT | ADDRESS MATCH | Human-readable response |
| AUTH_CARD_TYPE | V | Card brand: V=Visa |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:54:19 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 100.00 | Original amount requested |
| AUTH_AMOUNT | 100.00 | Amount authorized |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from card network |
| AUTH_MASKED_ACCOUNT_NBR | ************0002 | Masked PAN showing last 4 digits |
| AUTH_TRAN_IDENT | 355338068594036 | Network transaction ID |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference |

</details>

---

### 3.2 Credit Card CAPTURE (CCE4)

**Purpose**: Capture a previously authorized transaction

**Important**:
- Use the `AUTH_GUID` from the AUTH response as `ORIG_AUTH_GUID`
- CAPTURE must be performed within the authorization window (typically 7-30 days)
- Consider using **SALE (CCE1)** for Auth+Capture in one step

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE4" \
  -d "AMOUNT=100.00" \
  -d "TRAN_NBR=1000000021" \
  -d "ORIG_AUTH_GUID=09LMZFMK42BDVJGMWW0" \
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
    <FIELD KEY="TRAN_TYPE">CCE4</FIELD>
    <FIELD KEY="TRAN_NBR">1000000021</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">205539</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFMMXBZR9Z91WF8</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVAL</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 01:55:39 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">100.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE4</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| MSG_VERSION | 003 | EPX message format version |
| CUST_NBR | 9001 | Customer number (merchant identifier) |
| MERCH_NBR | 900300 | Merchant number |
| DBA_NBR | 2 | DBA (Doing Business As) number |
| TERMINAL_NBR | 77 | Terminal number |
| TRAN_TYPE | CCE4 | Transaction type: CCE4=Capture |
| TRAN_NBR | 1000000021 | Your transaction reference number |
| LOCAL_DATE | 120325 | Local date (MMDDYY format) |
| LOCAL_TIME | 205539 | Local time (HHMMSS format) |
| AUTH_GUID | 09LMZFMMXBZR9Z91WF8 | Capture BRIC - use for VOID/REFUND |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_RESP_TEXT | APPROVAL | Human-readable response |
| AUTH_CARD_TYPE | V | Card brand: V=Visa |
| AUTH_TRAN_DATE_GMT | 12/04/2025 01:55:39 AM | Transaction timestamp in GMT |
| AUTH_AMOUNT_REQUESTED | 100.00 | Original amount requested |
| AUTH_AMOUNT | 100.00 | Amount captured |
| AUTH_CURRENCY_CODE | 840 | ISO 4217 currency: 840=USD |
| NETWORK_RESPONSE | 00 | Raw response from card network |
| ORIG_TRAN_TYPE | CCE4 | Original transaction type (echoed back) |

</details>

---

### 3.3 Credit Card VOID (CCEX)

**Purpose**: Cancel an authorized or captured transaction

> **VOID vs REFUND - Settlement Timing**
>
> | Timing | Action | Result |
> |--------|--------|--------|
> | **Before settlement** | VOID (CCEX) | Cancels transaction, no funds move |
> | **After settlement** | REFUND (CCE9) | Returns funds to customer |
>
> - **Settlement cutoff**: Typically 11 PM ET daily
> - **Sandbox**: Usually lenient - voids may always succeed
> - **Production**: VOID fails after settlement → must use REFUND
> - **Best practice**: Try VOID first, fallback to REFUND if it fails

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCEX" \
  -d "AMOUNT=100.00" \
  -d "TRAN_NBR=1000000022" \
  -d "ORIG_AUTH_GUID=09LMZFMMXBZR9Z91WF8" \
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
    <FIELD KEY="TRAN_TYPE">CCEX</FIELD>
    <FIELD KEY="TRAN_NBR">1000000022</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">205827</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFMRLNKUV5TBWJ6</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVAL</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 01:58:27 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">100.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCEX</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCEX | Transaction type: CCEX=Void |
| TRAN_NBR | 1000000022 | Your transaction reference number |
| AUTH_GUID | 09LMZFMRLNKUV5TBWJ6 | Void reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_RESP_TEXT | APPROVAL | Transaction voided successfully |
| AUTH_AMOUNT | 100.00 | Amount voided |

</details>

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
  -d "TRAN_NBR=1000000023" \
  -d "ORIG_AUTH_GUID=0A1MZFJUHWT9REHXFRR" \
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
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="TRAN_NBR">1000000023</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">205859</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFMZL1UY5667WJY</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">046183</FIELD>
    <FIELD KEY="AUTH_AVS">A</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ADDRESS MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 01:58:59 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">200.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">200.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355338071391122</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCE1 | Transaction type: CCE1=Sale (Auth+Capture) |
| TRAN_NBR | 1000000023 | Your transaction reference number |
| AUTH_GUID | 09LMZFMZL1UY5667WJY | **Use for VOID or REFUND** |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 046183 | 6-digit authorization code from issuer |
| AUTH_AVS | A | AVS result: A=Address Match Only |
| AUTH_RESP_TEXT | ADDRESS MATCH | Human-readable response |
| AUTH_CARD_TYPE | V | Card brand: V=Visa |
| AUTH_AMOUNT | 200.00 | Amount charged |
| AUTH_TRAN_IDENT | 355338071391122 | Network transaction ID |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference |

</details>

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
  -d "TRAN_NBR=1000000024" \
  -d "ORIG_AUTH_GUID=09LMZFMZL1UY5667WJY" \
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
    <FIELD KEY="TRAN_TYPE">CCE9</FIELD>
    <FIELD KEY="TRAN_NBR">1000000024</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210318</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFN4EZD8QEL1WYX</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">046265</FIELD>
    <FIELD KEY="AUTH_AVS">A</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ADDRESS MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:03:18 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">50.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">50.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE9</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355338073981370</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCE9 | Transaction type: CCE9=Refund |
| TRAN_NBR | 1000000024 | Your transaction reference number |
| AUTH_GUID | 09LMZFN4EZD8QEL1WYX | Refund reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 046265 | Authorization code |
| AUTH_RESP_TEXT | ADDRESS MATCH | Human-readable response |
| AUTH_AMOUNT | 50.00 | Amount refunded (partial refund of $200 SALE) |
| AUTH_TRAN_IDENT | 355338073981370 | Network transaction ID |

</details>

---

### 3.6 Credit Card REVERSAL (CCE7)

**Purpose**: Cancel transaction AND release authorization hold immediately

> **REVERSAL (CCE7) vs VOID (CCEX) - Key Difference**
>
> | Method | What it does | Fund Release |
> |--------|--------------|--------------|
> | **VOID (CCEX)** | Cancels transaction only | 3-10 business days |
> | **REVERSAL (CCE7)** | Cancels transaction AND releases auth hold | **Immediate** |
>
> - **Use CCE7** when customer needs immediate fund release (e.g., canceled order)
> - **Use CCEX** when timing doesn't matter (e.g., duplicate transaction cleanup)
> - **Credit cards only** - ACH transactions use CKCX (Void) instead

#### Request

```bash
# Reversal requires the AUTH_GUID from a previous AUTH (CCE2) or SALE (CCE1) transaction
# In this example, 0A1MT3PF0WL4PHUMJU9 was obtained from a $100.00 AUTH (CCE2) via Browser POST
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CCE7" \
  -d "TRAN_NBR=1733958209" \
  -d "TRAN_GROUP=CCE7_1733958209" \
  -d "ORIG_AUTH_GUID=0A1MT3PF0WL4PHUMJU9" \
  -d "CARD_ENT_METH=Z" \
  -d "INDUSTRY_TYPE=E"
```

> **Note**: Unlike VOID (CCEX), REVERSAL does not require an AMOUNT field - it always reverses the full authorization.

#### Response

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="MSG_VERSION">003</FIELD>
    <FIELD KEY="CUST_NBR">9001</FIELD>
    <FIELD KEY="MERCH_NBR">900300</FIELD>
    <FIELD KEY="DBA_NBR">2</FIELD>
    <FIELD KEY="TERMINAL_NBR">77</FIELD>
    <FIELD KEY="TRAN_TYPE">CCE7</FIELD>
    <FIELD KEY="TRAN_NBR">1733958209</FIELD>
    <FIELD KEY="LOCAL_DATE">121125</FIELD>
    <FIELD KEY="LOCAL_TIME">160927</FIELD>
    <FIELD KEY="AUTH_GUID">09LMT3PG8FJ1XERAG52</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">039477</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">APPROVAL 039477</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/11/2025 09:09:27 PM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">0.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">0.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE7</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355345761267562</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

> **Verified**: Actual EPX sandbox response from CCE7 reversal test (December 11, 2025)

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCE7 | Transaction type: CCE7=Reversal |
| TRAN_NBR | 1733958209 | Your transaction reference number |
| ORIG_AUTH_GUID | 0A1MT3PF0WL4PHUMJU9 | BRIC from original AUTH ($100.00) that was reversed |
| AUTH_GUID | 09LMT3PG8FJ1XERAG52 | New BRIC for the reversal transaction |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 039477 | Authorization code from card network |
| AUTH_RESP_TEXT | APPROVAL 039477 | Transaction reversed successfully |
| AUTH_AMOUNT | 0.00 | Reversal releases full authorization (no partial reversal) |
| AUTH_TRAN_IDENT | 355345761267562 | Network transaction identifier |
| AUTH_CARD_TYPE | V | Visa |
| AUTH_MASKED_ACCOUNT_NBR | ************0002 | Masked card number from original |
| AUTH_PAR | V40000000028FAB8191EEC1C39808 | Payment Account Reference (for network token tracking) |

</details>

---

### 3.7 Recurring Billing (CCE1 + ACI_EXT=RB)

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
  -d "TRAN_NBR=1000000025" \
  -d "ORIG_AUTH_GUID=0A1MZFJUHWT9REHXFRR" \
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
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="TRAN_NBR">1000000025</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210406</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFN61K68JM1QWPD</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">046280</FIELD>
    <FIELD KEY="AUTH_AVS">A</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ADDRESS MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:04:06 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">29.99</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355338074469817</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCE1 | Transaction type: CCE1=Sale |
| ACI_EXT | RB | **Recurring Billing** - first attempt each billing cycle |
| AUTH_GUID | 09LMZFN61K68JM1QWPD | Transaction reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 046280 | Authorization code |
| AUTH_AMOUNT | 29.99 | Subscription amount charged |

</details>

---

### 3.8 Recurring Retry (CCE1 + ACI_EXT=RS)

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
  -d "TRAN_NBR=1000000026" \
  -d "ORIG_AUTH_GUID=0A1MZFJUHWT9REHXFRR" \
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
    <FIELD KEY="TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="TRAN_NBR">1000000026</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210455</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFN7HHP133J8WQA</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">046297</FIELD>
    <FIELD KEY="AUTH_AVS">A</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ADDRESS MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:04:55 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">29.99</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_CARD_COUNTRY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="AUTH_CARD_B">D</FIELD>
    <FIELD KEY="AUTH_CARD_C">F</FIELD>
    <FIELD KEY="AUTH_CARD_E">N</FIELD>
    <FIELD KEY="AUTH_CARD_F">Y</FIELD>
    <FIELD KEY="AUTH_CARD_G">N</FIELD>
    <FIELD KEY="AUTH_CARD_I">Y</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">************0002</FIELD>
    <FIELD KEY="AUTH_CARD_L">P</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CCE1</FIELD>
    <FIELD KEY="AUTH_TRAN_IDENT">355338074950953</FIELD>
    <FIELD KEY="AUTH_PAR">V40000000028FAB8191EEC1C39808</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CCE1 | Transaction type: CCE1=Sale |
| ACI_EXT | RS | **Resubmission** - retry within 30 days of decline |
| AUTH_GUID | 09LMZFN7HHP133J8WQA | Transaction reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 046297 | Authorization code |
| AUTH_AMOUNT | 29.99 | Retry amount charged |

</details>

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
  -d "TRAN_NBR=2000000030" \
  -d "ORIG_AUTH_GUID=0A1MZFKR6U2DPP5BFRT" \
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
    <FIELD KEY="TRAN_TYPE">CKC2</FIELD>
    <FIELD KEY="TRAN_NBR">2000000030</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210638</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFNAL64EHR4XWRZ</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">197189</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ACCEPTED 197189</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">L</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:06:38 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">25.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">25.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CKC2</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CKC2 | Transaction type: CKC2=ACH Debit (Sale) |
| TRAN_NBR | 2000000030 | Your transaction reference number |
| AUTH_GUID | 09LMZFNAL64EHR4XWRZ | **Use for REFUND** |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 197189 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 197189 | Human-readable response |
| AUTH_CARD_TYPE | L | L=ACH/eCheck |
| AUTH_AMOUNT | 25.00 | Amount debited |
| AUTH_MASKED_ACCOUNT_NBR | *****6789 | Masked bank account |

</details>

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
  -d "TRAN_NBR=2000000031" \
  -d "ORIG_AUTH_GUID=09LMZFNAL64EHR4XWRZ" \
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
    <FIELD KEY="TRAN_TYPE">CKC3</FIELD>
    <FIELD KEY="TRAN_NBR">2000000031</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210750</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFNWRL70JBGJWZF</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">912095</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ACCEPTED 912095</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">L</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:07:50 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">10.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">10.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CKC3</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CKC3 | Transaction type: CKC3=ACH Credit (Refund) |
| TRAN_NBR | 2000000031 | Your transaction reference number |
| AUTH_GUID | 09LMZFNWRL70JBGJWZF | Refund reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 912095 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 912095 | Human-readable response |
| AUTH_CARD_TYPE | L | L=ACH/eCheck |
| AUTH_AMOUNT | 10.00 | Amount refunded (partial refund of $25 sale) |

</details>

---

### 4.3 ACH Pre-note (CKC0)

**Purpose**: Verify bank account validity without charging ($0.00 transaction)

**Note**: Prenote verifies the bank account is valid before charging. Use with stored ACH BRIC token.

#### Request

```bash
curl -X POST "https://secure.epxuap.com" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=CKC0" \
  -d "AMOUNT=0.00" \
  -d "TRAN_NBR=2000000032" \
  -d "ORIG_AUTH_GUID=0A1MZFKR6U2DPP5BFRT" \
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
    <FIELD KEY="TRAN_TYPE">CKC0</FIELD>
    <FIELD KEY="TRAN_NBR">2000000032</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">210901</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFNF1225ZB2QWT3</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">195144</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ACCEPTED 195144</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">L</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:09:01 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">0.00</FIELD>
    <FIELD KEY="AUTH_AMOUNT">0.00</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CKC0</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CKC0 | Transaction type: CKC0=ACH Pre-note (verification) |
| TRAN_NBR | 2000000032 | Your transaction reference number |
| AUTH_GUID | 09LMZFNF1225ZB2QWT3 | Pre-note reference |
| AUTH_RESP | 00 | Response code: 00=Account verified |
| AUTH_CODE | 195144 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 195144 | Human-readable response |
| AUTH_CARD_TYPE | L | L=ACH/eCheck |
| AUTH_AMOUNT | 0.00 | $0 verification (no charge) |

</details>

**Note**: ACH transactions do not use ACI_EXT for MIT classification. The STD_ENTRY_CLASS (WEB/PPD) indicates the authorization method.

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
  -d "TRAN_NBR=2000000033" \
  -d "ORIG_AUTH_GUID=0A1MZFKR6U2DPP5BFRT" \
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
    <FIELD KEY="TRAN_TYPE">CKC2</FIELD>
    <FIELD KEY="TRAN_NBR">2000000033</FIELD>
    <FIELD KEY="LOCAL_DATE">120325</FIELD>
    <FIELD KEY="LOCAL_TIME">211052</FIELD>
    <FIELD KEY="AUTH_GUID">09LMZFNXDZH5QV70WUT</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">115014</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ACCEPTED 115014</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">L</FIELD>
    <FIELD KEY="AUTH_TRAN_DATE_GMT">12/04/2025 02:10:52 AM</FIELD>
    <FIELD KEY="AUTH_AMOUNT_REQUESTED">29.99</FIELD>
    <FIELD KEY="AUTH_AMOUNT">29.99</FIELD>
    <FIELD KEY="AUTH_CURRENCY_CODE">840</FIELD>
    <FIELD KEY="NETWORK_RESPONSE">00</FIELD>
    <FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">*****6789</FIELD>
    <FIELD KEY="ORIG_TRAN_TYPE">CKC2</FIELD>
  </FIELDS>
</RESPONSE>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Value | Description |
|-------|-------|-------------|
| TRAN_TYPE | CKC2 | Transaction type: CKC2=ACH Debit |
| STD_ENTRY_CLASS | PPD | **Prearranged Payment** - for recurring/subscription |
| TRAN_NBR | 2000000033 | Your transaction reference number |
| AUTH_GUID | 09LMZFNXDZH5QV70WUT | Transaction reference |
| AUTH_RESP | 00 | Response code: 00=Approved |
| AUTH_CODE | 115014 | Authorization code |
| AUTH_RESP_TEXT | ACCEPTED 115014 | Human-readable response |
| AUTH_CARD_TYPE | L | L=ACH/eCheck |
| AUTH_AMOUNT | 29.99 | Subscription amount charged |

</details>

**Note**: ACH subscription retries use the same `STD_ENTRY_CLASS=PPD`. Unlike credit cards, ACH does not use ACI_EXT for retry tracking.

---

## 5. Payment Service REST API

These endpoints are provided by the Payment Service to facilitate Browser POST integration.

---

### 5.1 Get Form Config

**Endpoint**: `GET /api/v1/payments/browser-post/form`

**Purpose**: Returns configuration needed to build the Browser POST form

#### JSON Response

```json
{
  "transactionId": "txn_abc123",
  "epxTranNbr": "2000000050",
  "tac": "svM11dEbyhguvn5w8CuTPQ==|xPX24MwXfbMy262rkTF4lPe0qbzO84WB...",
  "expiresAt": 1733356800,
  "postURL": "https://services.epxuap.com/browserpost/",
  "custNbr": "9001",
  "merchNbr": "900300",
  "dbaName": "2",
  "terminalNbr": "77",
  "industryType": "E",
  "tranCode": "SALE",
  "redirectURL": "http://localhost:8081/api/v1/payments/browser-post/callback",
  "returnUrl": "https://merchant.example.com/checkout/complete",
  "merchantId": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
  "merchantName": "Acme Corp"
}
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Type | Description |
|-------|------|-------------|
| transactionId | string | Payment Service transaction ID |
| epxTranNbr | string | EPX transaction number (TRAN_NBR) |
| tac | string | Transaction Authentication Code from Key Exchange |
| expiresAt | integer | Unix timestamp when TAC expires |
| postURL | string | EPX Browser POST endpoint URL |
| custNbr | string | EPX customer number |
| merchNbr | string | EPX merchant number |
| dbaName | string | EPX DBA number |
| terminalNbr | string | EPX terminal number |
| industryType | string | Industry type (E=E-commerce) |
| tranCode | string | Transaction code: SALE, AUTH, STORAGE, ACHSTORAGE_C, ACHSTORAGE_S |
| redirectURL | string | Callback URL where EPX sends response |
| returnUrl | string | Merchant URL to redirect user after completion |
| merchantId | string | Payment Service merchant UUID |
| merchantName | string | Merchant display name |

</details>

---

### 5.2 Browser POST HTML Forms

These HTML forms submit directly to EPX. Card/bank data never touches your servers (PCI compliant).

#### 5.2.1 Credit Card Payment Form (SALE/AUTH)

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- EPX Authentication (from form config) -->
  <input type="hidden" name="TAC" value="${formConfig.tac}">

  <!-- Merchant Credentials (from form config) -->
  <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}">
  <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}">
  <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}">
  <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}">
  <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}">
  <input type="hidden" name="AMOUNT" value="50.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}">

  <!-- Card Details (user input) -->
  <input type="hidden" name="ACCOUNT_NBR" value="4761739001010010">
  <input type="hidden" name="EXP_DATE" value="1227">
  <input type="hidden" name="CVV2" value="999">

  <!-- Cardholder Info (optional but recommended for AVS) -->
  <input type="hidden" name="FIRST_NAME" value="John">
  <input type="hidden" name="LAST_NAME" value="Doe">
  <input type="hidden" name="ADDRESS" value="123 Main St">
  <input type="hidden" name="CITY" value="New York">
  <input type="hidden" name="STATE" value="NY">
  <input type="hidden" name="ZIP_CODE" value="10001">

  <button type="submit">Pay $50.00</button>
</form>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| TAC | hidden | Yes | Transaction Authentication Code from Key Exchange |
| TRAN_CODE | hidden | Yes | `SALE` (auth+capture) or `AUTH` (auth only) |
| CUST_NBR | hidden | Yes | EPX customer number |
| MERCH_NBR | hidden | Yes | EPX merchant number |
| DBA_NBR | hidden | Yes | EPX DBA number |
| TERMINAL_NBR | hidden | Yes | EPX terminal number |
| AMOUNT | hidden | Yes | Amount in dollars.cents format |
| INDUSTRY_TYPE | hidden | Yes | `E` for E-commerce |
| ACCOUNT_NBR | text | Yes | Credit card number (no spaces, 13-16 digits) |
| EXP_DATE | text | Yes | Expiration date in MMYY format |
| CVV2 | text | Yes | Card verification value (3-4 digits) |
| FIRST_NAME | text | No | Cardholder first name (optional, for AVS) |
| LAST_NAME | text | No | Cardholder last name (optional, for AVS) |
| ADDRESS | text | No | Billing street address (optional, for AVS) |
| CITY | text | No | Billing city (optional, for AVS) |
| STATE | text | No | Billing state (optional, for AVS) |
| ZIP_CODE | text | No | Billing ZIP code (recommended for AVS) |

</details>

---

#### 5.2.2 Credit Card Storage Form (STORAGE)

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- EPX Authentication (from form config) -->
  <input type="hidden" name="TAC" value="${formConfig.tac}">

  <!-- Merchant Credentials (from form config) -->
  <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}">
  <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}">
  <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}">
  <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}">
  <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}">

  <!-- Card Details (user input) -->
  <input type="hidden" name="ACCOUNT_NBR" value="4761739001010010">
  <input type="hidden" name="EXP_DATE" value="1227">
  <input type="hidden" name="CVV2" value="999">

  <!-- Cardholder Info (optional but recommended for AVS) -->
  <input type="hidden" name="FIRST_NAME" value="Jane">
  <input type="hidden" name="LAST_NAME" value="Smith">
  <input type="hidden" name="ADDRESS" value="456 Oak Ave">
  <input type="hidden" name="CITY" value="Los Angeles">
  <input type="hidden" name="STATE" value="CA">
  <input type="hidden" name="ZIP_CODE" value="90001">

  <button type="submit">Save Card</button>
</form>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| TAC | hidden | Yes | Transaction Authentication Code from Key Exchange |
| TRAN_CODE | hidden | Yes | `STORAGE` for card tokenization |
| CUST_NBR | hidden | Yes | EPX customer number |
| MERCH_NBR | hidden | Yes | EPX merchant number |
| DBA_NBR | hidden | Yes | EPX DBA number |
| TERMINAL_NBR | hidden | Yes | EPX terminal number |
| AMOUNT | hidden | Yes | `0.00` (no charge for storage) |
| INDUSTRY_TYPE | hidden | Yes | `E` for E-commerce |
| ACCOUNT_NBR | text | Yes | Credit card number (no spaces) |
| EXP_DATE | text | Yes | Expiration date in MMYY format |
| CVV2 | text | Yes | Card verification value |
| FIRST_NAME | text | No | Cardholder first name (optional, for AVS) |
| LAST_NAME | text | No | Cardholder last name (optional, for AVS) |
| ADDRESS | text | No | Billing street address (optional, for AVS) |
| CITY | text | No | Billing city (optional, for AVS) |
| STATE | text | No | Billing state (optional, for AVS) |
| ZIP_CODE | text | No | Billing ZIP code (recommended for AVS) |

</details>

---

#### 5.2.3 ACH Bank Account Storage Form (ACH_STORAGE_C/ACH_STORAGE_S)

**Note:** AVS (Address Verification System) does NOT work for ACH transactions - it only applies to credit cards. ACH verification uses prenote ($0 transaction), micro-deposits, or third-party services like Plaid. Address fields are not recommended for ACH forms.

```html
<form action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- EPX Authentication (from form config) -->
  <input type="hidden" name="TAC" value="${formConfig.tac}">

  <!-- Merchant Credentials (from form config) -->
  <input type="hidden" name="TRAN_CODE" value="${formConfig.tranCode}">
  <input type="hidden" name="CUST_NBR" value="${formConfig.custNbr}">
  <input type="hidden" name="MERCH_NBR" value="${formConfig.merchNbr}">
  <input type="hidden" name="DBA_NBR" value="${formConfig.dbaName}">
  <input type="hidden" name="TERMINAL_NBR" value="${formConfig.terminalNbr}">
  <input type="hidden" name="AMOUNT" value="0.00">
  <input type="hidden" name="INDUSTRY_TYPE" value="${formConfig.industryType}">
  <input type="hidden" name="STD_ENTRY_CLASS" value="WEB">

  <!-- Bank Account Details (user input) -->
  <input type="hidden" name="ACCOUNT_NBR" value="*****6789">
  <input type="hidden" name="ROUTING_NBR" value="011000015">

  <!-- Account Holder Name (required for ACH) -->
  <input type="hidden" name="FIRST_NAME" value="John">
  <input type="hidden" name="LAST_NAME" value="Doe">

  <button type="submit">Save Bank Account</button>
</form>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| TAC | hidden | Yes | Transaction Authentication Code from Key Exchange |
| TRAN_CODE | hidden | Yes | `ACHSTORAGE_C` (checking) or `ACHSTORAGE_S` (savings) |
| CUST_NBR | hidden | Yes | EPX customer number |
| MERCH_NBR | hidden | Yes | EPX merchant number |
| DBA_NBR | hidden | Yes | EPX DBA number |
| TERMINAL_NBR | hidden | Yes | EPX terminal number |
| AMOUNT | hidden | Yes | `0.00` (no charge for storage) |
| INDUSTRY_TYPE | hidden | Yes | `E` for E-commerce |
| STD_ENTRY_CLASS | hidden | Yes | `WEB` for internet-initiated |
| ACCOUNT_NBR | text | Yes | Bank account number |
| ROUTING_NBR | text | Yes | Bank routing number (9 digits) |
| FIRST_NAME | text | Yes | Account holder first name |
| LAST_NAME | text | Yes | Account holder last name |

</details>

---

### 5.3 Callback HTML Responses

The callback endpoint returns HTML pages to the user's browser after EPX processes the transaction.

#### 5.3.1 Payment Receipt (SALE/AUTH)

**Endpoint**: `GET/POST /api/v1/payments/browser-post/callback`

**Purpose**: Processes EPX callback for SALE/AUTH transactions and displays receipt

#### HTML Response

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Successful</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .card {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header { text-align: center; margin-bottom: 30px; }
        .icon { font-size: 64px; margin-bottom: 20px; }
        .icon.success { color: #10b981; }
        .status { font-size: 24px; font-weight: bold; margin: 20px 0; }
        .status.success { color: #10b981; }
        .amount {
            font-size: 32px;
            font-weight: bold;
            text-align: center;
            margin: 20px 0;
            color: #111827;
        }
        .details {
            margin: 30px 0;
            border-top: 2px solid #e5e7eb;
            padding-top: 20px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            margin: 15px 0;
            padding: 10px 0;
            border-bottom: 1px solid #f3f4f6;
        }
        .detail-label { font-weight: 600; color: #6b7280; }
        .detail-value { color: #111827; font-weight: 500; }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            color: #6b7280;
            font-size: 14px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <div class="icon success">✓</div>
            <div class="status success">Payment Successful</div>
        </div>
        <div class="amount">$100.00 USD</div>
        <div class="details">
            <div class="detail-row">
                <span class="detail-label">Card Type</span>
                <span class="detail-value">Visa</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Card Number</span>
                <span class="detail-value">**** **** **** 0002</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Authorization Code</span>
                <span class="detail-value">045023</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Transaction ID</span>
                <span class="detail-value">txn_abc123</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Reference Number</span>
                <span class="detail-value">2000000011</span>
            </div>
        </div>
        <div class="footer">
            <p>Thank you for your payment!</p>
            <a href="https://merchant.example.com/checkout/complete" class="button">Return to Application</a>
        </div>
    </div>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Description |
|-------|-------------|
| Amount | Transaction amount with currency |
| Card Type | Card brand (Visa, Mastercard, Amex, Discover) |
| Card Number | Masked PAN showing last 4 digits |
| Authorization Code | 6-digit auth code from issuer |
| Transaction ID | Payment Service transaction ID |
| Reference Number | EPX TRAN_NBR |

</details>

---

#### 5.3.2 Credit Card Stored (STORAGE)

**Endpoint**: `GET/POST /api/v1/payments/browser-post/callback`

**Purpose**: Processes EPX callback for STORAGE transactions and displays confirmation

#### HTML Response

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Method Saved</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .card {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header { text-align: center; margin-bottom: 30px; }
        .icon { font-size: 64px; margin-bottom: 20px; }
        .icon.success { color: #10b981; }
        .status { font-size: 24px; font-weight: bold; margin: 20px 0; }
        .status.success { color: #10b981; }
        .details {
            margin: 30px 0;
            border-top: 2px solid #e5e7eb;
            padding-top: 20px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            margin: 15px 0;
            padding: 10px 0;
            border-bottom: 1px solid #f3f4f6;
        }
        .detail-label { font-weight: 600; color: #6b7280; }
        .detail-value { color: #111827; font-weight: 500; }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 9999px;
            font-size: 14px;
            font-weight: 500;
        }
        .badge.verified { background-color: #d1fae5; color: #065f46; }
        .info-box {
            background-color: #eff6ff;
            border: 1px solid #bfdbfe;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
            color: #1e40af;
            font-size: 14px;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            color: #6b7280;
            font-size: 14px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <div class="icon success">✓</div>
            <div class="status success">Payment Method Saved</div>
        </div>
        <div class="details">
            <div class="detail-row">
                <span class="detail-label">Card Type</span>
                <span class="detail-value">Visa</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Card Number</span>
                <span class="detail-value">**** **** **** 0002</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Expiration</span>
                <span class="detail-value">12/27</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Status</span>
                <span class="detail-value"><span class="badge verified">Verified</span></span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Payment Method ID</span>
                <span class="detail-value">pm_xyz789</span>
            </div>
        </div>
        <div class="info-box">
            Your card has been securely saved and is ready for use.
        </div>
        <div class="footer">
            <p>Your payment method has been saved successfully.</p>
            <a href="https://merchant.example.com/account/payment-methods" class="button">Return to Application</a>
        </div>
    </div>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Description |
|-------|-------------|
| Card Type | Card brand (Visa, Mastercard, Amex, Discover) |
| Card Number | Masked PAN showing last 4 digits |
| Expiration | Card expiration date (MM/YY) |
| Status | Verification status (Verified for credit cards) |
| Payment Method ID | Payment Service payment method UUID |

</details>

---

#### 5.3.3 Bank Account Stored (ACH_STORAGE)

**Endpoint**: `GET/POST /api/v1/payments/browser-post/callback`

**Purpose**: Processes EPX callback for ACH STORAGE transactions and displays confirmation

#### HTML Response

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Bank Account Saved</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .card {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header { text-align: center; margin-bottom: 30px; }
        .icon { font-size: 64px; margin-bottom: 20px; }
        .icon.warning { color: #f59e0b; }
        .status { font-size: 24px; font-weight: bold; margin: 20px 0; }
        .status.warning { color: #f59e0b; }
        .details {
            margin: 30px 0;
            border-top: 2px solid #e5e7eb;
            padding-top: 20px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            margin: 15px 0;
            padding: 10px 0;
            border-bottom: 1px solid #f3f4f6;
        }
        .detail-label { font-weight: 600; color: #6b7280; }
        .detail-value { color: #111827; font-weight: 500; }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 9999px;
            font-size: 14px;
            font-weight: 500;
        }
        .badge.pending { background-color: #fef3c7; color: #92400e; }
        .warning-box {
            background-color: #fefce8;
            border: 1px solid #fde047;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
            color: #854d0e;
            font-size: 14px;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            color: #6b7280;
            font-size: 14px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <div class="icon warning">⚠</div>
            <div class="status warning">Verification Pending</div>
        </div>
        <div class="details">
            <div class="detail-row">
                <span class="detail-label">Account Type</span>
                <span class="detail-value">Checking</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Account Number</span>
                <span class="detail-value">***** 6789</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Status</span>
                <span class="detail-value"><span class="badge pending">Pending Verification</span></span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Payment Method ID</span>
                <span class="detail-value">pm_ach456</span>
            </div>
        </div>
        <div class="warning-box">
            <strong>Verification in Progress</strong><br>
            A prenote has been sent to verify your bank account. This typically takes 2-3 business days.
            You will be able to use this payment method once verification is complete.
        </div>
        <div class="footer">
            <p>Your bank account has been saved and is pending verification.</p>
            <a href="https://merchant.example.com/account/payment-methods" class="button">Return to Application</a>
        </div>
    </div>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Description |
|-------|-------------|
| Account Type | Checking or Savings |
| Account Number | Masked account showing last 4 digits |
| Status | Pending Verification (ACH requires prenote verification) |
| Payment Method ID | Payment Service payment method UUID |

</details>

---

#### 5.3.4 Error

**Endpoint**: `GET/POST /api/v1/payments/browser-post/callback`

**Purpose**: Displays error when payment processing fails

#### HTML Response

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Error</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .card {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header { text-align: center; margin-bottom: 30px; }
        .icon { font-size: 64px; margin-bottom: 20px; }
        .icon.error { color: #ef4444; }
        .status { font-size: 24px; font-weight: bold; margin: 20px 0; }
        .status.error { color: #ef4444; }
        .message {
            text-align: center;
            color: #6b7280;
            margin: 20px 0;
        }
        .error-box {
            background-color: #fef2f2;
            border: 1px solid #fecaca;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
            color: #991b1b;
            font-size: 14px;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            color: #6b7280;
            font-size: 14px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <div class="icon error">⚠</div>
            <div class="status error">Payment Error</div>
        </div>
        <p class="message">Your payment could not be processed.</p>
        <div class="error-box">
            Card declined: Insufficient funds (51)
        </div>
        <div class="footer">
            <a href="https://merchant.example.com/checkout" class="button">Return to Application</a>
        </div>
    </div>
</body>
</html>
```

<details>
<summary>Field Reference (click to expand)</summary>

| Field | Description |
|-------|-------------|
| Message | General error description |
| Details | Specific error reason (e.g., decline code) |

</details>

---

## 6. Card on File (COF) Compliance

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
| CAPTURE (CCE4) | AUTH's AUTH_GUID | Links capture to specific auth |
| REFUND (CCE9) | Sale/Capture AUTH_GUID | Links refund to original charge |
| VOID (CCEX) | Transaction's AUTH_GUID | Voids specific transaction |

---

## 7. Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_ACH_CLASS` | `WEB` | Default STD_ENTRY_CLASS for ACH transactions |
| `RS_WINDOW_DAYS` | `30` | Days within which ACI_EXT=RS can be used |

### CARD_ENT_METH Values

| Value | Meaning | Use Case |
|-------|---------|----------|
| `E` | Key-entered | Browser POST with raw card/bank data |
| `Z` | BRIC token | Server POST with stored credential (Credit Card or ACH) |

### INDUSTRY_TYPE Values

| Value | Meaning |
|-------|---------|
| `E` | E-commerce (online transactions) |

---

**Document Version**: 1.1 | **Last Updated**: 2025-12-10 | **Status**: UNCERTIFIED (Real Sandbox Data)
