# Future: End-to-End Testing Architecture

## When to Create E2E Tests Repository

**Create a separate `e2e-tests` repository when:**
- ✅ You have 2+ microservices that interact
- ✅ You need to test complete user workflows spanning multiple services
- ✅ You want to test service orchestration and inter-service communication

**Current Status:** Not needed yet (only have payment-service)

---

## Future Microservices Architecture

```
kevin07696/
├── payment-service/              # Payment processing
├── subscription-service/         # Recurring billing
├── notification-service/         # Email/SMS notifications
├── user-service/                # User authentication
├── e2e-tests/                   # ← Cross-service E2E tests
└── deployment-workflows/         # Shared infrastructure
```

---

## E2E Tests Repository Structure

### When You Have Multiple Services:

```
e2e-tests/
├── .github/workflows/
│   └── e2e-tests.yml            # Triggered after all services deployed
│
├── tests/
│   ├── user_journey/
│   │   ├── signup_subscribe_pay_test.go
│   │   │   # 1. User signs up (user-service)
│   │   │   # 2. Creates subscription (subscription-service)
│   │   │   # 3. Processes payment (payment-service)
│   │   │   # 4. Sends confirmation (notification-service)
│   │   │
│   │   └── cancel_refund_test.go
│   │       # 1. Cancel subscription (subscription-service)
│   │       # 2. Process refund (payment-service)
│   │       # 3. Send notification (notification-service)
│   │
│   ├── integration_flows/
│   │   ├── payment_notification_test.go
│   │   │   # Test payment-service → notification-service
│   │   │
│   │   └── subscription_payment_test.go
│   │       # Test subscription-service → payment-service
│   │
│   └── testutil/
│       ├── service_clients/     # HTTP clients for each service
│       │   ├── payment_client.go
│       │   ├── subscription_client.go
│       │   ├── notification_client.go
│       │   └── user_client.go
│       │
│       ├── setup.go             # Multi-service test setup
│       └── fixtures.go          # Cross-service test data
│
├── config/
│   └── services.yaml            # Service URLs and configuration
│
├── go.mod
└── README.md
```

---

## Deployment Pipeline with E2E Tests

### Current (Single Service):

```
┌────────────────────────────────────────────┐
│ payment-service                             │
├────────────────────────────────────────────┤
│ Unit Tests → Build → Deploy → Integration │
│                                  ↑          │
│                          Tests in same repo │
└────────────────────────────────────────────┘
```

### Future (Multiple Services):

```
┌──────────────────────────────────────────────────────────────┐
│ Deploy All Services to Staging                               │
│ ├─ payment-service → staging                                 │
│ ├─ subscription-service → staging                            │
│ ├─ notification-service → staging                            │
│ └─ user-service → staging                                    │
├──────────────────────────────────────────────────────────────┤
│ Run Per-Service Integration Tests (in parallel)              │
│ ├─ payment-service/tests/integration/                        │
│ ├─ subscription-service/tests/integration/                   │
│ └─ ... (test each service independently)                     │
├──────────────────────────────────────────────────────────────┤
│ Run Cross-Service E2E Tests (DEPLOYMENT GATE)                │
│ ├─ e2e-tests/tests/user_journey/                            │
│ ├─ e2e-tests/tests/integration_flows/                       │
│ └─ Validates all services work together                      │
├──────────────────────────────────────────────────────────────┤
│ Deploy to Production (ONLY if E2E tests pass)                │
│ └─ Deploy all services to production                         │
└──────────────────────────────────────────────────────────────┘
```

---

## Example E2E Test

```go
// e2e-tests/tests/user_journey/signup_subscribe_pay_test.go

package user_journey_test

import (
	"testing"

	"github.com/kevin07696/e2e-tests/testutil/service_clients"
	"github.com/stretchr/testify/require"
)

func TestCompleteUserJourney_SignupSubscribePay(t *testing.T) {
	// Setup clients for all services
	userClient := service_clients.NewUserClient()
	subClient := service_clients.NewSubscriptionClient()
	payClient := service_clients.NewPaymentClient()
	notifClient := service_clients.NewNotificationClient()

	// 1. User signs up
	user, err := userClient.SignUp(&SignUpRequest{
		Email: "test@example.com",
		Name:  "Test User",
	})
	require.NoError(t, err)
	t.Logf("User created: %s", user.ID)

	// 2. Create subscription
	subscription, err := subClient.CreateSubscription(&SubscriptionRequest{
		UserID: user.ID,
		Plan:   "premium",
	})
	require.NoError(t, err)
	t.Logf("Subscription created: %s", subscription.ID)

	// 3. Verify payment was processed (subscription-service calls payment-service)
	payment, err := payClient.GetPaymentBySubscription(subscription.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", payment.Status)
	t.Logf("Payment processed: %s", payment.ID)

	// 4. Verify notification was sent (payment-service triggers notification-service)
	notifications, err := notifClient.GetNotificationsByUser(user.ID)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	require.Contains(t, notifications[0].Message, "subscription confirmed")
	t.Logf("Notification sent: %s", notifications[0].ID)

	// Cleanup
	defer userClient.DeleteUser(user.ID)
	defer subClient.DeleteSubscription(subscription.ID)
}
```

---

## Differences: Integration Tests vs E2E Tests

### Integration Tests (payment-service/tests/integration/)

**Purpose**: Test payment-service's integration with external systems
**Scope**: Single service + its dependencies (DB, EPX)
**Location**: Same repo as service code
**Examples:**
- Does merchant registration API work?
- Does EPX adapter process payments correctly?
- Does database store data correctly?

**Characteristics:**
- Tests ONE service in isolation
- Uses seed data
- Fast execution
- Part of service CI/CD

---

### E2E Tests (future: e2e-tests repo)

**Purpose**: Test complete user workflows across ALL services
**Scope**: Multiple services working together
**Location**: Separate repository
**Examples:**
- Can a user sign up, subscribe, and get charged?
- Does subscription cancellation trigger refund?
- Are notifications sent when payments complete?

**Characteristics:**
- Tests MULTIPLE services together
- Simulates real user journeys
- Slower execution (orchestrates multiple services)
- Separate from individual service CI/CD

---

## When to Implement E2E Tests

### Triggers:
1. **Second service deployed**: When subscription-service or notification-service exists
2. **Service dependencies**: When services call each other's APIs
3. **Complex workflows**: When user journeys span multiple services

### Prerequisites:
- All services deployed to staging environment
- Service discovery or known service URLs
- Consistent authentication across services
- Shared test credentials/data

---

## Benefits of Separate E2E Repo

✅ **Independent versioning**: E2E tests evolve separately from services
✅ **Cross-team ownership**: Different teams can contribute
✅ **Prevent circular dependencies**: Services don't depend on each other's test code
✅ **Comprehensive coverage**: Test real system behavior
✅ **Deployment confidence**: Catch integration issues before production

---

## Implementation Checklist (Future)

When you're ready to create E2E tests:

- [ ] Create `e2e-tests` repository
- [ ] Set up service clients for each microservice
- [ ] Define common test fixtures and utilities
- [ ] Write user journey tests
- [ ] Integrate E2E tests into deployment pipeline
- [ ] Configure service URLs in GitHub Secrets
- [ ] Set up test data management across services
- [ ] Add E2E test results to deployment gates

---

## Current Recommendation

**For now:** Keep using integration tests in `payment-service/tests/integration/`

**When you have 2+ services:** Create `e2e-tests` repository

**Pipeline remains Amazon-style:** Integration tests → E2E tests → Production deployment
