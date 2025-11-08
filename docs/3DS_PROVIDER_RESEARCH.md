# 3D Secure Provider Research for North/EPX Payment Gateway

**Research Date:** 2025-01-05
**Payment Gateway:** Electronic Payment Exchange (EPX) / North American Bancard

## Executive Summary

EPX supports 3D Secure Version 2 authentication but **does not perform the authentication itself**. EPX only receives and processes 3DS authentication data from external providers. This means you need to integrate with a separate 3DS authentication provider (MPI - Merchant Plug-In) to perform the authentication before sending transactions to EPX.

## Key Finding: North + Cybersource Partnership

**Most Relevant Integration:** North American Bancard has a direct integration with **Cybersource**, which includes payer authentication (3D Secure) capabilities. This is likely the most seamless path for EPX merchants to add 3DS support.

## Recommended 3DS Provider Options

### Option 1: Cybersource (Recommended for EPX)

**Why Recommended:**
- Direct partnership with North American Bancard (parent company of EPX)
- Uses Cardinal Commerce (Visa-owned) 3DS infrastructure
- Integrated solution for fraud management and payer authentication
- EMVCo certified

**Key Features:**
- Front-end authentication with Decision Manager integration
- PSD2 Strong Customer Authentication (SCA) compliant
- Mobile-optimized authentication flows
- Supports frictionless and challenge flows
- EMV 3DS 2.x protocol

**Integration Approach:**
- When using CyberSource for 3DS, you receive Cardinal Cruise API credentials
- Authenticate payment through CyberSource/Cardinal
- Receive 3DS authentication data
- Pass 3DS fields to EPX in Server Post transaction

**Pricing:**
- Contact-based pricing (not publicly disclosed)
- Likely includes per-authentication fee + scheme fees

**Pros:**
- Aligned with North American Bancard ecosystem
- Single vendor for fraud management + 3DS
- Enterprise-grade support

**Cons:**
- Pricing not transparent
- May require enterprise-level contract
- Full payment gateway features (may be more than needed)

---

### Option 2: Cardinal Commerce / Visa Acceptance Platform

**Overview:**
- Owned by Visa (subsidiary)
- Industry standard 3DS authentication provider
- Compatible with many payment processors including EPX

**Key Features:**
- Consumer Authentication (CCA) service
- Supports all major card brands (Visa, MC, Amex, Discover)
- 3DS 2.x with EMVCo certification
- Supports frictionless and challenge flows

**Integration Approach:**
- Integrate Cardinal SDK on frontend
- Use Cardinal API for authentication
- Receive 3DS results
- Pass to EPX Server Post with required fields

**Migration Timeline:**
- Cardinal Cruise Standard will not be supported after December 31, 2025
- Migrating to Visa Acceptance Platform by June 30, 2025

**Market Position:**
- 3.90% market share in payments processing
- 20,718+ customers
- Compatible with CyberSource, Authorize.net, Chase Paymentech, First Data, etc.

**Pricing:**
- Per-authentication fee model
- Contact Cardinal for pricing

**Pros:**
- Industry standard, widely adopted
- Visa-backed reliability
- Extensive documentation
- Works with multiple processors

**Cons:**
- Platform migration happening in 2025
- Requires separate Cardinal contract
- Pricing not transparent

---

### Option 3: Stripe Standalone 3DS

**Overview:**
- Standalone 3DS decouples authentication from payment processing
- Designed for enterprises using independent processors

**Key Features:**
- API-level control over 3DS requests
- Returns 3DS cryptogram for use with any processor
- Supports Visa, Mastercard, Amex, Discover, Cartes Bancaires
- Available in all Stripe countries (except India, Malaysia, Thailand)

**Integration Approach:**
1. Use Stripe Standalone 3DS API to authenticate
2. Receive 3DS cryptogram and authentication data
3. Submit to EPX via Server Post with 3DS fields

**Pricing:**
- Per-authentication pricing (contact Stripe)
- Separate from payment processing fees since using standalone mode

**Pros:**
- Modern API with excellent documentation
- Flexible - works with any payment processor
- Strong developer experience
- Clear separation of concerns

**Cons:**
- Requires Stripe account (even though not processing through Stripe)
- Additional vendor relationship
- May have geographic limitations

---

### Option 4: Adyen 3DS Authentication Service

**Overview:**
- Full-featured authentication platform
- Can use Adyen 3DS with external payment processors

**Key Features:**
- Native and Redirect authentication flows
- Frictionless and Challenge authentication
- Delegated authentication capability
- Advanced risk-based authentication
- Supports third-party MPI data

**Integration Approach:**
- Use Adyen Authentication Engine
- Receive `mpiData` object
- Pass authentication results to EPX

**Pricing:**
- Adyen Authentication Service fee per request
- Plus card scheme fees (passed through):
  - Mastercard: per 3DS authentication request
  - Visa: 0.02 EUR per authentication
- Contact Adyen for specific pricing

**Pros:**
- Advanced authentication optimization
- Strong authorization rate improvements
- Can use their MPI with external processors
- Global coverage

**Cons:**
- Premium pricing tier
- Enterprise-focused (may be overkill)
- Requires Adyen relationship

---

### Option 5: GPayments ActiveMerchant MPI

**Overview:**
- Dedicated MPI provider with 20+ years experience
- Platform-agnostic solution

**Key Features:**
- Supports Visa, MC, JCB, Amex, Diners Club
- ActiveMerchant MPI product
- EMVCo certified
- Proven track record (used by Sony Payment Services)

**Integration Approach:**
- Integrate ActiveMerchant MPI
- Perform 3DS authentication
- Receive authentication data
- Submit to EPX

**Pricing:**
- Contact GPayments for pricing

**Pros:**
- Dedicated MPI focus (not a full gateway)
- Platform-agnostic
- Proven enterprise clients
- EMVCo certified

**Cons:**
- Less well-known than Cardinal/Cybersource
- Documentation may be less accessible
- Smaller developer community

---

## EPX 3DS Integration Requirements

### Required Fields for EPX

Based on EPX Transaction Specs - 3D Secure documentation:

**Standard 3DS Fields:**
- `TDS_VER`: Must be "2" (3DS Version 2)
- `CAVV_RESP`: ECI (Electronic Commerce Indicator)
- `CAVV_UCAF`: Cryptogram (Base64 28 chars or HEX 40 chars)
- `TOKEN_TRAN_IDENT`: Token Transaction Identifier
- `DIRECTORY_SERVER_TRAN_ID`: UUID format, up to 36 chars (required for 3DS V2)
- `CARD_ENT_METH`: "E" for account number, "Z" for BRIC token
- `INDUSTRY_TYPE`: "E" for Ecommerce

**3rd Party Token (Digital Wallets):**
- Use `TAVV` instead of `CAVV_UCAF`
- Use `TAVV_ECI` instead of `CAVV_RESP`
- Different `CARD_ENT_METH` values by brand

**Transaction Types:**
- CCE1: Sale (Auth + Capture)
- CCE2: Authorization Only

**Configuration:**
- Merchant profile must be configured as "Ecommerce"

### Authentication Flow

```
1. Frontend: Collect card details
2. Backend → 3DS Provider: Initiate authentication
3. 3DS Provider → Frontend: Display challenge (if required)
4. User: Complete authentication
5. Frontend → Backend: Send 3DS results
6. Backend → EPX: Send transaction with 3DS fields
```

## Comparison Matrix

| Provider | EPX Integration | Cost Transparency | Developer Experience | Enterprise Support | Recommendation |
|----------|----------------|-------------------|---------------------|-------------------|----------------|
| **Cybersource** | ⭐⭐⭐⭐⭐ (Direct partnership) | ⭐⭐ (Contact sales) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | **Best for EPX** |
| **Cardinal Commerce** | ⭐⭐⭐⭐ (Industry standard) | ⭐⭐ (Contact sales) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Strong option |
| **Stripe Standalone** | ⭐⭐⭐ (Generic support) | ⭐⭐⭐ (Clear docs) | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Best DX |
| **Adyen** | ⭐⭐⭐ (Generic MPI) | ⭐⭐⭐ (Documented) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Premium tier |
| **GPayments** | ⭐⭐⭐⭐ (MPI specialist) | ⭐⭐ (Contact sales) | ⭐⭐⭐ | ⭐⭐⭐⭐ | Specialized |

## Final Recommendation

### For Production: Cybersource + Cardinal

**Recommended Approach:**
1. Contact North American Bancard about Cybersource integration
2. Cybersource provides Cardinal Commerce 3DS capabilities
3. Leverages existing North partnership
4. Single vendor for fraud + 3DS
5. Enterprise support included

**Why This Path:**
- North already has Cybersource partnership announced
- Reduces vendor complexity
- Better support from aligned vendors
- Likely better pricing due to existing relationship

### For Modern Developer Experience: Stripe Standalone 3DS

**Alternative Approach:**
1. Use Stripe Standalone 3DS API for authentication only
2. Process payments through EPX as current
3. Clear separation of concerns
4. Excellent documentation and developer experience

**Why This Path:**
- Best API documentation
- Modern developer tools
- Flexible and processor-agnostic
- Easy to test and implement

## Implementation Considerations

### Technical Requirements

1. **Frontend Integration:**
   - 3DS provider SDK (JavaScript)
   - Challenge flow UI handling
   - Device fingerprinting support

2. **Backend Integration:**
   - 3DS provider API client
   - EPX Server Post adapter modifications
   - Add 3DS field mapping

3. **Security:**
   - PCI DSS compliance maintained
   - Secure transmission of authentication data
   - No storage of sensitive 3DS cryptograms

### Timeline Estimate

- **Research & Vendor Selection:** 1-2 weeks
- **Contract & Credentials:** 2-4 weeks
- **Frontend Integration:** 1-2 weeks
- **Backend Integration:** 1 week
- **Testing & Certification:** 2-3 weeks
- **Total:** 7-12 weeks

### Cost Considerations

**Typical 3DS Pricing Components:**
1. Setup/Integration fee: $0-$5,000 (varies by provider)
2. Monthly platform fee: $0-$500/month
3. Per-authentication fee: $0.05-$0.15 per request
4. Card scheme fees: $0.02-$0.04 per authentication (passed through)

**Estimated Monthly Cost for 10,000 transactions:**
- Provider fee: $500-$1,500
- Scheme fees: $200-$400
- Total: $700-$1,900/month

## Next Steps

1. **Contact North American Bancard:**
   - Ask about Cybersource 3DS integration
   - Request pricing and integration guide
   - Confirm EPX compatibility

2. **Evaluate Alternatives:**
   - Request Stripe Standalone 3DS demo
   - Get Cardinal Commerce pricing quote
   - Compare features vs. cost

3. **Plan Implementation:**
   - Review EPX 3DS field requirements
   - Design authentication flow
   - Plan frontend/backend changes
   - Estimate development timeline

4. **Proof of Concept:**
   - Start with test environment
   - Implement basic 3DS flow
   - Validate EPX integration
   - Measure performance impact

## Resources

- **EPX Documentation:** `/supplemental-resources/Trans Specs/EPX Transaction Specs - 3D Secure _ 3rd Party Token Support.pdf`
- **Cybersource 3DS:** https://www.cybersource.com/en-us/solutions/fraud-and-risk-management/payer-authentication-for-3d-secure.html
- **Stripe Standalone 3DS:** https://docs.stripe.com/payments/payment-intents/standalone-three-d-secure
- **Cardinal Commerce:** https://win.cardinalcommerce.com/
- **Adyen 3DS:** https://docs.adyen.com/online-payments/3d-secure
