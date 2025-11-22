package payment

import (
	"sync"

	"github.com/kevin07696/payment-service/internal/domain"
)

var (
	// TransactionSlicePool pools transaction slices for query results
	// Used when fetching lists of transactions (merchant dashboard, customer history, etc.)
	TransactionSlicePool = sync.Pool{
		New: func() interface{} {
			// Pre-allocate for typical page size (100 transactions)
			slice := make([]*domain.Transaction, 0, 100)
			return &slice
		},
	}

	// TransactionPool pools individual Transaction objects
	// Used for single transaction operations (create, update, fetch by ID)
	TransactionPool = sync.Pool{
		New: func() interface{} {
			return &domain.Transaction{
				// Pre-allocate metadata map with typical capacity
				Metadata: make(map[string]interface{}, 8),
			}
		},
	}

	// MetadataMapPool pools metadata maps
	// Used across the service for transaction, payment method, and subscription metadata
	MetadataMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 8)
		},
	}
)

// GetTransactionSlice retrieves a transaction slice from the pool
func GetTransactionSlice() *[]*domain.Transaction {
	return TransactionSlicePool.Get().(*[]*domain.Transaction)
}

// PutTransactionSlice returns a transaction slice to the pool after clearing
func PutTransactionSlice(slice *[]*domain.Transaction) {
	// Clear the slice but keep capacity
	*slice = (*slice)[:0]
	TransactionSlicePool.Put(slice)
}

// GetTransaction retrieves a Transaction from the pool
func GetTransaction() *domain.Transaction {
	return TransactionPool.Get().(*domain.Transaction)
}

// PutTransaction returns a Transaction to the pool after clearing
func PutTransaction(tx *domain.Transaction) {
	// Clear all fields
	tx.ID = ""
	tx.MerchantID = ""
	tx.CustomerID = nil
	tx.PaymentMethodID = nil
	tx.AmountCents = 0
	tx.Currency = ""
	tx.Status = ""
	tx.Type = ""
	tx.PaymentMethodType = ""
	tx.AuthGUID = ""
	tx.ParentTransactionID = nil
	tx.SubscriptionID = nil
	tx.IdempotencyKey = nil
	tx.AuthResp = nil
	tx.AuthCode = nil
	tx.AuthRespText = nil
	tx.AuthAVS = nil
	tx.AuthCVV2 = nil
	tx.AuthCardType = nil

	// Clear metadata map
	for k := range tx.Metadata {
		delete(tx.Metadata, k)
	}

	TransactionPool.Put(tx)
}

// GetMetadataMap retrieves a metadata map from the pool
func GetMetadataMap() map[string]interface{} {
	m := MetadataMapPool.Get().(map[string]interface{})
	// Clear existing data
	for k := range m {
		delete(m, k)
	}
	return m
}

// PutMetadataMap returns a metadata map to the pool
func PutMetadataMap(m map[string]interface{}) {
	// Clear all data
	for k := range m {
		delete(m, k)
	}
	MetadataMapPool.Put(m)
}
