package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/kevin07696/payment-service/internal/handlers/payment"
	"github.com/kevin07696/payment-service/pkg/middleware"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting ConnectRPC POC Server")

	// For POC, we'll use a nil service (would normally initialize with DB, adapters, etc.)
	// This is just to verify the Connect setup works
	// In a real implementation, you'd initialize your actual payment service here
	logger.Warn("POC mode: Payment service not initialized - handlers will return errors")

	// Create payment handler (with nil service for POC)
	paymentHandler := payment.NewConnectHandler(nil, logger)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Create Connect interceptors
	interceptors := connect.WithInterceptors(
		middleware.RecoveryInterceptor(logger),
		middleware.LoggingInterceptor(logger),
	)

	// Register payment service handler
	path, handler := paymentv1connect.NewPaymentServiceHandler(
		paymentHandler,
		interceptors,
	)
	mux.Handle(path, handler)

	// Add health check
	checker := grpchealth.NewStaticChecker(
		paymentv1connect.PaymentServiceName,
	)
	mux.Handle(grpchealth.NewHandler(checker))

	// Add reflection
	reflector := grpcreflect.NewStaticReflector(
		paymentv1connect.PaymentServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	// Create server with H2C support (HTTP/2 without TLS)
	addr := ":8080"
	server := &http.Server{
		Addr:              addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("ConnectRPC server listening",
			zap.String("addr", addr),
			zap.String("protocols", "Connect, gRPC, gRPC-Web"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	logger.Info("POC Server started successfully",
		zap.String("health_check", "http://localhost:8080/grpc.health.v1.Health/Check"),
		zap.String("reflection", "http://localhost:8080/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"),
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
