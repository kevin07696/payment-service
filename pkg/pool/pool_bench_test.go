package pool_test

import (
	"testing"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/pkg/pool"
)

// Benchmark_TransactionPool_Get measures allocation overhead of pool vs direct allocation
func Benchmark_TransactionPool_Get(b *testing.B) {
	b.Run("with_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tx := pool.GetTransaction()
			// Simulate usage
			tx.ID = "test-id"
			tx.AmountCents = 1000
			pool.PutTransaction(tx)
		}
	})

	b.Run("without_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tx := &domain.Transaction{
				Metadata: make(map[string]interface{}, 8),
			}
			// Simulate usage
			tx.ID = "test-id"
			tx.AmountCents = 1000
			// No pool return - simulates current behavior
			_ = tx
		}
	})
}

// Benchmark_EPXPool_ServerPost measures EPX request/response pooling
func Benchmark_EPXPool_ServerPost(b *testing.B) {
	b.Run("request_with_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := pool.GetServerPostRequest()
			// Simulate usage
			req.CustNbr = "12345"
			req.Amount = "10.00"
			pool.PutServerPostRequest(req)
		}
	})

	b.Run("request_without_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := &ports.ServerPostRequest{}
			// Simulate usage
			req.CustNbr = "12345"
			req.Amount = "10.00"
			_ = req
		}
	})

	b.Run("response_with_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			resp := pool.GetServerPostResponse()
			// Simulate usage
			resp.AuthGUID = "test-bric"
			resp.AuthResp = "00"
			pool.PutServerPostResponse(resp)
		}
	})

	b.Run("response_without_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			resp := &ports.ServerPostResponse{}
			// Simulate usage
			resp.AuthGUID = "test-bric"
			resp.AuthResp = "00"
			_ = resp
		}
	})
}

// Benchmark_BRICPool measures BRIC storage request/response pooling
func Benchmark_BRICPool(b *testing.B) {
	b.Run("request_with_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := pool.GetBRICStorageRequest()
			// Simulate usage
			req.CustNbr = "12345"
			req.CardNumber = "4111111111111111"
			pool.PutBRICStorageRequest(req)
		}
	})

	b.Run("request_without_pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := &ports.BRICStorageRequest{}
			// Simulate usage
			req.CustNbr = "12345"
			req.CardNumber = "4111111111111111"
			_ = req
		}
	})
}
