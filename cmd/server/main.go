package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	paymentv1 "github.com/kevin07696/payment-service/api/proto/payment/v1"
	subscriptionv1 "github.com/kevin07696/payment-service/api/proto/subscription/v1"
	"github.com/kevin07696/payment-service/internal/adapters/north"
	"github.com/kevin07696/payment-service/internal/adapters/postgres"
	grpcPayment "github.com/kevin07696/payment-service/internal/api/grpc/payment"
	grpcSubscription "github.com/kevin07696/payment-service/internal/api/grpc/subscription"
	"github.com/kevin07696/payment-service/internal/config"
	"github.com/kevin07696/payment-service/internal/services/payment"
	"github.com/kevin07696/payment-service/internal/services/subscription"
	"github.com/kevin07696/payment-service/pkg/observability"
	"github.com/kevin07696/payment-service/pkg/security"
)

func main() {
	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := initLogger(cfg.Logger)
	defer logger.Sync()

	logger.Info("Starting payment service",
		zap.String("version", "0.1.0"),
		zap.Int("port", cfg.Server.Port))

	// Initialize database connection pool
	dbPool, err := initDatabase(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer dbPool.Close()

	logger.Info("Database connection established",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port))

	// Initialize dependencies
	deps := initDependencies(dbPool, cfg, logger)

	// Start metrics and health check server
	healthChecker := observability.NewHealthChecker(dbPool)
	metricsServer := observability.StartMetricsServer(
		fmt.Sprintf("%d", cfg.Server.MetricsPort),
		healthChecker,
	)
	logger.Info("Metrics server started",
		zap.Int("port", cfg.Server.MetricsPort),
		zap.String("metrics", fmt.Sprintf("http://localhost:%d/metrics", cfg.Server.MetricsPort)),
		zap.String("health", fmt.Sprintf("http://localhost:%d/health", cfg.Server.MetricsPort)))

	// Initialize gRPC server with chained interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			observability.UnaryServerInterceptor(),
			loggingInterceptor(logger),
		),
	)

	// Register services
	paymentv1.RegisterPaymentServiceServer(grpcServer, deps.paymentHandler)
	subscriptionv1.RegisterSubscriptionServiceServer(grpcServer, deps.subscriptionHandler)

	// Register reflection service (for tools like grpcurl)
	reflection.Register(grpcServer)

	// Start server
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Graceful shutdown
	go func() {
		logger.Info("gRPC server listening", zap.String("address", listener.Addr().String()))
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	grpcServer.GracefulStop()

	// Shutdown metrics server
	if err := observability.ShutdownMetricsServer(metricsServer); err != nil {
		logger.Error("Error shutting down metrics server", zap.Error(err))
	}

	logger.Info("Server stopped")
}

// Dependencies holds all initialized services and handlers
type Dependencies struct {
	paymentHandler      *grpcPayment.Handler
	subscriptionHandler *grpcSubscription.Handler
}

// initLogger initializes the logger based on configuration
func initLogger(cfg config.LoggerConfig) *zap.Logger {
	if cfg.Development {
		logger, _ := zap.NewDevelopment()
		return logger
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(parseLogLevel(cfg.Level))

	logger, _ := zapCfg.Build()
	return logger
}

// parseLogLevel converts string log level to zapcore level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// initDatabase initializes the PostgreSQL connection pool
func initDatabase(cfg config.DatabaseConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// initDependencies initializes all services and handlers with dependency injection
func initDependencies(dbPool *pgxpool.Pool, cfg *config.Config, zapLogger *zap.Logger) *Dependencies {
	// Create logger adapter
	logger := security.NewZapLogger(zapLogger)

	// Initialize database port
	dbPort := postgres.NewDBExecutor(dbPool)

	// Initialize repositories
	txRepo := postgres.NewTransactionRepository(dbPort)
	subRepo := postgres.NewSubscriptionRepository(dbPort)

	// Initialize North payment gateway adapter
	authConfig := north.AuthConfig{
		EPIId:  cfg.Gateway.EPIId,
		EPIKey: cfg.Gateway.EPIKey,
	}

	// Create HTTP client for gateway adapters
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Gateway.Timeout) * time.Second,
	}

	// Use BrowserPostAdapter for PCI-compliant tokenized payments
	// Frontend uses North JavaScript SDK to tokenize cards → returns BRIC token
	// Backend receives BRIC token and processes payment (never touches raw card data)
	creditCardGateway := north.NewBrowserPostAdapter(
		authConfig,
		cfg.Gateway.BaseURL,
		httpClient,
		logger,
	)

	// Initialize North Recurring Billing adapter
	recurringGateway := north.NewRecurringBillingAdapter(
		authConfig,
		cfg.Gateway.BaseURL,
		httpClient,
		logger,
	)

	// Initialize Payment Service (business logic)
	paymentService := payment.NewService(
		dbPort,
		txRepo,
		creditCardGateway,
		logger,
	)

	// Initialize Subscription Service (business logic)
	subscriptionService := subscription.NewService(
		dbPort,
		subRepo,
		paymentService,
		recurringGateway,
		logger,
	)

	// Initialize gRPC handlers
	paymentHandler := grpcPayment.NewHandler(paymentService, logger)
	subscriptionHandler := grpcSubscription.NewHandler(subscriptionService, logger)

	return &Dependencies{
		paymentHandler:      paymentHandler,
		subscriptionHandler: subscriptionHandler,
	}
}

// loggingInterceptor logs all gRPC requests
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		// Log the request
		logger.Info("gRPC request",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.Error(err),
		)

		return resp, err
	}
}
