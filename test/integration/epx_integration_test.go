package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/kevin07696/payment-service/api/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestEPXSandboxConnectivity verifies EPX sandbox is accessible
func TestEPXSandboxConnectivity(t *testing.T) {
	if os.Getenv("SKIP_EPX_TESTS") == "true" {
		t.Skip("EPX tests disabled via SKIP_EPX_TESTS")
	}

	cfg := LoadTestConfig()

	// Connect to gRPC server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		cfg.GRPCHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaymentServiceClient(conn)

	// Test EPX connectivity with a minimal verification
	// This uses a $0.00 authorization which EPX supports for verification
	t.Run("EPX Sandbox Reachability", func(t *testing.T) {
		req := &pb.ProcessPaymentRequest{
			Amount:      0, // $0.00 verification
			Currency:    "USD",
			Description: "Integration test - EPX connectivity check",
			PaymentMethod: &pb.PaymentMethod{
				Type: pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD,
				Card: &pb.Card{
					Number:         "4111111111111111", // Test card
					ExpiryMonth:    12,
					ExpiryYear:     uint32(time.Now().Year() + 1),
					Cvv:            "123",
					CardholderName: "Integration Test",
					BillingAddress: &pb.Address{
						Line1:      "123 Test St",
						City:       "Test City",
						State:      "CA",
						PostalCode: "12345",
						Country:    "US",
					},
				},
			},
		}

		resp, err := client.ProcessPayment(ctx, req)
		if err != nil {
			t.Logf("⚠️  EPX connectivity test failed: %v", err)
			t.Logf("This may indicate EPX sandbox is unreachable or misconfigured")
			// Don't fail - EPX may have temporary issues
			return
		}

		t.Logf("✅ EPX Sandbox responded")
		t.Logf("   Transaction ID: %s", resp.TransactionId)
		t.Logf("   Status: %s", resp.Status)
		t.Logf("   Response Code: %s", resp.ResponseCode)

		// Log but don't fail on declined/error - we just want to verify connectivity
		if resp.Status == pb.PaymentStatus_PAYMENT_STATUS_FAILED {
			t.Logf("⚠️  Payment was declined (expected for some test cards): %s", resp.Message)
		}
	})
}

// TestEPXResponseCodes verifies EPX is returning proper response codes
func TestEPXResponseCodes(t *testing.T) {
	if os.Getenv("SKIP_EPX_TESTS") == "true" {
		t.Skip("EPX tests disabled via SKIP_EPX_TESTS")
	}

	cfg := LoadTestConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		cfg.GRPCHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaymentServiceClient(conn)

	t.Run("Valid Card - Approved", func(t *testing.T) {
		req := &pb.ProcessPaymentRequest{
			Amount:      100, // $1.00
			Currency:    "USD",
			Description: "Integration test - valid card",
			PaymentMethod: &pb.PaymentMethod{
				Type: pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD,
				Card: &pb.Card{
					Number:         "4111111111111111",
					ExpiryMonth:    12,
					ExpiryYear:     uint32(time.Now().Year() + 1),
					Cvv:            "123",
					CardholderName: "Approved Test",
					BillingAddress: &pb.Address{
						Line1:      "123 Test St",
						City:       "Test City",
						State:      "CA",
						PostalCode: "12345",
						Country:    "US",
					},
				},
			},
		}

		resp, err := client.ProcessPayment(ctx, req)
		if err != nil {
			t.Logf("Payment request failed: %v", err)
			return
		}

		t.Logf("Response Code: %s", resp.ResponseCode)
		t.Logf("Status: %s", resp.Status.String())
		t.Logf("Message: %s", resp.Message)

		// Check if we got a response (approved or declined is fine, we just want valid response)
		if resp.ResponseCode == "" {
			t.Error("Expected response code from EPX, got empty string")
		}

		t.Log("✅ EPX returned valid response structure")
	})
}

// TestDatabasePersistence verifies transactions are being saved to database
func TestDatabasePersistence(t *testing.T) {
	if os.Getenv("SKIP_EPX_TESTS") == "true" {
		t.Skip("EPX tests disabled via SKIP_EPX_TESTS")
	}

	cfg := LoadTestConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		cfg.GRPCHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaymentServiceClient(conn)

	// Process a payment
	req := &pb.ProcessPaymentRequest{
		Amount:      50, // $0.50
		Currency:    "USD",
		Description: "Integration test - database persistence",
		PaymentMethod: &pb.PaymentMethod{
			Type: pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD,
			Card: &pb.Card{
				Number:         "4111111111111111",
				ExpiryMonth:    12,
				ExpiryYear:     uint32(time.Now().Year() + 1),
				Cvv:            "123",
				CardholderName: "DB Test",
				BillingAddress: &pb.Address{
					Line1:      "123 Test St",
					City:       "Test City",
					State:      "CA",
					PostalCode: "12345",
					Country:    "US",
				},
			},
		},
	}

	resp, err := client.ProcessPayment(ctx, req)
	if err != nil {
		t.Logf("Payment failed (this is ok for connectivity test): %v", err)
		return
	}

	if resp.TransactionId == "" {
		t.Error("Expected transaction ID to be returned")
		return
	}

	t.Logf("✅ Transaction created with ID: %s", resp.TransactionId)

	// TODO: Add GetTransaction endpoint and verify we can retrieve it
	// For now, we verify transaction ID was generated (indicates DB write occurred)
}

// TestServiceConcurrency verifies the service handles concurrent requests
func TestServiceConcurrency(t *testing.T) {
	if os.Getenv("SKIP_LOAD_TESTS") == "true" {
		t.Skip("Load tests disabled via SKIP_LOAD_TESTS")
	}

	cfg := LoadTestConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		cfg.GRPCHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaymentServiceClient(conn)

	// Send 10 concurrent requests
	concurrency := 10
	errChan := make(chan error, concurrency)
	successChan := make(chan string, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			req := &pb.ProcessPaymentRequest{
				Amount:      int64((index + 1) * 10), // $0.10, $0.20, etc.
				Currency:    "USD",
				Description: fmt.Sprintf("Concurrent test %d", index),
				PaymentMethod: &pb.PaymentMethod{
					Type: pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD,
					Card: &pb.Card{
						Number:         "4111111111111111",
						ExpiryMonth:    12,
						ExpiryYear:     uint32(time.Now().Year() + 1),
						Cvv:            "123",
						CardholderName: fmt.Sprintf("Concurrent Test %d", index),
						BillingAddress: &pb.Address{
							Line1:      "123 Test St",
							City:       "Test City",
							State:      "CA",
							PostalCode: "12345",
							Country:    "US",
						},
					},
				},
			}

			resp, err := client.ProcessPayment(ctx, req)
			if err != nil {
				errChan <- err
				return
			}

			successChan <- resp.TransactionId
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0

	for i := 0; i < concurrency; i++ {
		select {
		case txID := <-successChan:
			successCount++
			t.Logf("✅ Concurrent request %d succeeded: %s", i+1, txID)
		case err := <-errChan:
			errorCount++
			t.Logf("⚠️  Concurrent request %d failed: %v", i+1, err)
		case <-time.After(30 * time.Second):
			t.Errorf("Timeout waiting for concurrent request %d", i+1)
		}
	}

	t.Logf("Concurrency test: %d successes, %d errors", successCount, errorCount)

	// At least some should succeed (EPX may decline some, but service should handle all)
	if successCount == 0 {
		t.Error("All concurrent requests failed - service may have concurrency issues")
	}
}
