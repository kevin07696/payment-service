package ports

import (
	"context"
	"time"
)

// DisputeSearchRequest contains parameters for searching disputes
type DisputeSearchRequest struct {
	MerchantID string
	FromDate   *time.Time // Optional: filter disputes from this date
	ToDate     *time.Time // Optional: filter disputes to this date
}

// Dispute represents a chargeback dispute from North gateway
type Dispute struct {
	CaseNumber         string
	DisputeDate        string
	ChargebackDate     string
	DisputeType        string
	Status             string
	CardBrand          string
	CardNumberLastFour string
	TransactionNumber  string
	ReasonCode         string
	ReasonDescription  string
	TransactionAmount  float64
	TransactionDate    string
	ChargebackAmount   float64
}

// DisputeSearchResponse contains dispute search results
type DisputeSearchResponse struct {
	Disputes           []*Dispute
	TotalDisputes      int
	CurrentResultCount int
}

// MerchantReportingAdapter defines the port for merchant reporting operations
type MerchantReportingAdapter interface {
	// SearchDisputes retrieves dispute/chargeback data for a merchant
	SearchDisputes(ctx context.Context, req *DisputeSearchRequest) (*DisputeSearchResponse, error)
}
