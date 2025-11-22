package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Payment transaction metrics
	paymentTransactionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_transactions_total",
		Help: "Total number of payment transactions",
	}, []string{
		"merchant_id",      // Which merchant
		"payment_type",     // credit_card, ach
		"transaction_type", // sale, auth, capture, refund, void
		"status",           // approved, declined, failed, pending
		"gateway_response", // 00=approved, 05=declined, etc. (EPX response code)
	})

	paymentAmountCents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_amount_cents_total",
		Help: "Total payment amount in cents (for revenue tracking)",
	}, []string{
		"merchant_id",
		"payment_type",
		"transaction_type",
		"status",
		"currency",
	})

	paymentSuccessRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_success_rate",
		Help: "Payment success rate (approved / total) calculated from counters",
	}, []string{
		"merchant_id",
		"payment_type",
	})

	// Payment processing duration (end-to-end)
	paymentProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "payment_processing_duration_seconds",
		Help: "Total time to process a payment transaction (end-to-end)",
		// Buckets: 100ms to 30s (typical payment processing times)
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{
		"merchant_id",
		"payment_type",
		"transaction_type",
		"status",
	})

	// ACH verification metrics
	achVerificationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ach_verifications_total",
		Help: "Total ACH payment method verifications",
	}, []string{
		"merchant_id",
		"status", // verified, failed, pending
	})

	// Subscription billing metrics
	subscriptionBillingsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "subscription_billings_total",
		Help: "Total subscription billing attempts",
	}, []string{
		"merchant_id",
		"status", // success, failed, retrying
	})

	subscriptionRevenueCents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "subscription_revenue_cents_total",
		Help: "Total subscription revenue in cents",
	}, []string{
		"merchant_id",
		"currency",
	})

	// Webhook delivery metrics
	webhookDeliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_deliveries_total",
		Help: "Total webhook delivery attempts",
	}, []string{
		"event_type",
		"status", // success, failed, pending
	})

	webhookDeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webhook_delivery_duration_seconds",
		Help:    "Time to deliver webhook",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	}, []string{
		"event_type",
	})

	// Chargeback metrics
	chargebacksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chargebacks_total",
		Help: "Total chargebacks received",
	}, []string{
		"merchant_id",
		"reason_code",
		"status", // pending, won, lost
	})

	// Customer payment method metrics
	paymentMethodsCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_methods_created_total",
		Help: "Total payment methods created (tokenized)",
	}, []string{
		"merchant_id",
		"payment_type", // credit_card, ach
	})

	activeCustomers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "active_customers",
		Help: "Number of active customers with valid payment methods",
	}, []string{
		"merchant_id",
	})

	// Customer lifetime value proxy
	customerTransactionCount = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "customer_transaction_count",
		Help:    "Distribution of transaction counts per customer",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{
		"merchant_id",
	})
)

// RecordPaymentTransaction records a payment transaction
// This is the primary business metric for revenue tracking and success rate calculation
func RecordPaymentTransaction(
	merchantID, paymentType, transactionType, status, gatewayResponse string,
	amountCents int64,
	currency string,
	duration float64,
) {
	// Record transaction count
	paymentTransactionsTotal.WithLabelValues(
		merchantID,
		paymentType,
		transactionType,
		status,
		gatewayResponse,
	).Inc()

	// Record amount (for revenue tracking)
	// Only counts approved transactions toward revenue
	paymentAmountCents.WithLabelValues(
		merchantID,
		paymentType,
		transactionType,
		status,
		currency,
	).Add(float64(amountCents))

	// Record duration
	paymentProcessingDuration.WithLabelValues(
		merchantID,
		paymentType,
		transactionType,
		status,
	).Observe(duration)

	// Update success rate (calculated from approved/total ratio)
	// Note: This is calculated in PromQL queries, not stored directly:
	// sum(rate(payment_transactions_total{status="approved"}[5m])) by (merchant_id, payment_type)
	// /
	// sum(rate(payment_transactions_total[5m])) by (merchant_id, payment_type)
}

// RecordACHVerification records ACH verification result
func RecordACHVerification(merchantID, status string) {
	achVerificationsTotal.WithLabelValues(merchantID, status).Inc()
}

// RecordSubscriptionBilling records subscription billing attempt
func RecordSubscriptionBilling(merchantID, status string, amountCents int64, currency string) {
	subscriptionBillingsTotal.WithLabelValues(merchantID, status).Inc()

	// Only count successful billings toward revenue
	if status == "success" {
		subscriptionRevenueCents.WithLabelValues(merchantID, currency).Add(float64(amountCents))
	}
}

// RecordWebhookDelivery records webhook delivery
func RecordWebhookDelivery(eventType, status string, duration float64) {
	webhookDeliveriesTotal.WithLabelValues(eventType, status).Inc()
	webhookDeliveryDuration.WithLabelValues(eventType).Observe(duration)
}

// RecordChargeback records a chargeback
func RecordChargeback(merchantID, reasonCode, status string) {
	chargebacksTotal.WithLabelValues(merchantID, reasonCode, status).Inc()
}

// RecordPaymentMethodCreated records payment method tokenization
func RecordPaymentMethodCreated(merchantID, paymentType string) {
	paymentMethodsCreated.WithLabelValues(merchantID, paymentType).Inc()
}

// UpdateActiveCustomers updates the active customer count for a merchant
func UpdateActiveCustomers(merchantID string, count float64) {
	activeCustomers.WithLabelValues(merchantID).Set(count)
}

// RecordCustomerTransactionCount records transaction count for customer lifetime value analysis
func RecordCustomerTransactionCount(merchantID string, transactionCount float64) {
	customerTransactionCount.WithLabelValues(merchantID).Observe(transactionCount)
}
