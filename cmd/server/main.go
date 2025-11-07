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

	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/adapters/epx"
	"github.com/kevin07696/payment-service/internal/adapters/north"
	"github.com/kevin07696/payment-service/internal/adapters/secrets"
	agentHandler "github.com/kevin07696/payment-service/internal/handlers/agent"
	chargebackHandler "github.com/kevin07696/payment-service/internal/handlers/chargeback"
	cronHandler "github.com/kevin07696/payment-service/internal/handlers/cron"
	paymentHandler "github.com/kevin07696/payment-service/internal/handlers/payment"
	paymentmethodHandler "github.com/kevin07696/payment-service/internal/handlers/payment_method"
	subscriptionHandler "github.com/kevin07696/payment-service/internal/handlers/subscription"
	agentService "github.com/kevin07696/payment-service/internal/services/agent"
	paymentService "github.com/kevin07696/payment-service/internal/services/payment"
	paymentmethodService "github.com/kevin07696/payment-service/internal/services/payment_method"
	subscriptionService "github.com/kevin07696/payment-service/internal/services/subscription"
	webhookService "github.com/kevin07696/payment-service/internal/services/webhook"
	"github.com/kevin07696/payment-service/pkg/middleware"
	"github.com/kevin07696/payment-service/pkg/security"
	agentv1 "github.com/kevin07696/payment-service/proto/agent/v1"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	paymentmethodv1 "github.com/kevin07696/payment-service/proto/payment_method/v1"
	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer logger.Sync()

	logger.Info("Starting payment service",
		zap.String("version", "0.1.0"),
	)

	// Load configuration from environment
	cfg := loadConfig(logger)

	// Initialize database connection pool
	dbPool, err := initDatabase(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer dbPool.Close()

	logger.Info("Database connection established",
		zap.String("database", cfg.DBName),
	)

	// Initialize dependencies
	deps := initDependencies(dbPool, cfg, logger)

	// Initialize gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(logger),
			recoveryInterceptor(logger),
		),
	)

	// Register all gRPC services
	paymentv1.RegisterPaymentServiceServer(grpcServer, deps.paymentHandler)
	subscriptionv1.RegisterSubscriptionServiceServer(grpcServer, deps.subscriptionHandler)
	paymentmethodv1.RegisterPaymentMethodServiceServer(grpcServer, deps.paymentMethodHandler)
	agentv1.RegisterAgentServiceServer(grpcServer, deps.agentHandler)
	chargebackv1.RegisterChargebackServiceServer(grpcServer, deps.chargebackHandler)

	// Register reflection service (for tools like grpcurl)
	reflection.Register(grpcServer)

	// Setup HTTP server for cron endpoints and Browser Post callback
	httpMux := http.NewServeMux()

	// Create rate limiter (10 requests per second per IP, burst of 20)
	// Adjust these values based on expected staging traffic
	rateLimiter := middleware.NewRateLimiter(10, 20)

	// Cron endpoints
	httpMux.HandleFunc("/cron/process-billing", deps.billingCronHandler.ProcessBilling)
	httpMux.HandleFunc("/cron/sync-disputes", deps.disputeSyncCronHandler.SyncDisputes)
	httpMux.HandleFunc("/cron/health", deps.billingCronHandler.HealthCheck)
	httpMux.HandleFunc("/cron/stats", deps.billingCronHandler.Stats)

	// Browser Post endpoints (with rate limiting)
	httpMux.HandleFunc("/api/v1/payments/browser-post/form", rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.GetPaymentForm))
	httpMux.HandleFunc("/api/v1/payments/browser-post/callback", rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.HandleCallback))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: rateLimiter.Middleware(httpMux), // Apply rate limiting to all HTTP endpoints
	}

	// Start gRPC server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Start gRPC server in goroutine
	go func() {
		logger.Info("gRPC server listening",
			zap.String("address", listener.Addr().String()),
			zap.Int("port", cfg.Port),
		)
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Start HTTP server for cron in goroutine
	go func() {
		logger.Info("HTTP cron server listening",
			zap.Int("port", cfg.HTTPPort),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve HTTP", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Graceful shutdown
	grpcServer.GracefulStop()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	logger.Info("Servers stopped")
}

// Config holds application configuration
type Config struct {
	Port     int
	HTTPPort int // HTTP port for cron endpoints

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	MaxConns   int32
	MinConns   int32

	// EPX Gateway
	EPXBaseURL     string
	EPXTimeout     int
	EPXCustNbr     string // EPX Customer Number
	EPXMerchNbr    string // EPX Merchant Number
	EPXDBAnbr      string // EPX DBA Number
	EPXTerminalNbr string // EPX Terminal Number

	// North API (Merchant Reporting)
	NorthAPIURL  string
	NorthTimeout int

	// Browser Post Configuration
	CallbackBaseURL string // Base URL for Browser Post callbacks (e.g., "http://localhost:8081")

	// Cron authentication
	CronSecret string
}

// Dependencies holds all initialized services and handlers
type Dependencies struct {
	paymentHandler             paymentv1.PaymentServiceServer
	subscriptionHandler        subscriptionv1.SubscriptionServiceServer
	paymentMethodHandler       paymentmethodv1.PaymentMethodServiceServer
	agentHandler               agentv1.AgentServiceServer
	chargebackHandler          chargebackv1.ChargebackServiceServer
	billingCronHandler         *cronHandler.BillingHandler
	disputeSyncCronHandler     *cronHandler.DisputeSyncHandler
	browserPostCallbackHandler *paymentHandler.BrowserPostCallbackHandler
}

// loadConfig loads configuration from environment variables
func loadConfig(logger *zap.Logger) *Config {
	cfg := &Config{
		Port:            getEnvInt("PORT", 8080),
		HTTPPort:        getEnvInt("HTTP_PORT", 8081),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnvInt("DB_PORT", 5432),
		DBUser:          getEnv("DB_USER", "postgres"),
		DBPassword:      getEnv("DB_PASSWORD", "postgres"),
		DBName:          getEnv("DB_NAME", "payment_service"),
		DBSSLMode:       getEnv("DB_SSL_MODE", "disable"),
		MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 25)),
		MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
		EPXBaseURL:      getEnv("EPX_BASE_URL", "https://sandbox.north.com"),
		EPXTimeout:      getEnvInt("EPX_TIMEOUT", 30),
		EPXCustNbr:      getEnv("EPX_CUST_NBR", "9001"),    // EPX sandbox customer number
		EPXMerchNbr:     getEnv("EPX_MERCH_NBR", "900300"), // EPX sandbox merchant number
		EPXDBAnbr:       getEnv("EPX_DBA_NBR", "2"),        // EPX sandbox DBA number
		EPXTerminalNbr:  getEnv("EPX_TERMINAL_NBR", "77"),  // EPX sandbox terminal number
		NorthAPIURL:     getEnv("NORTH_API_URL", "https://api.north.com"),
		NorthTimeout:    getEnvInt("NORTH_TIMEOUT", 30),
		CallbackBaseURL: getEnv("CALLBACK_BASE_URL", "http://localhost:8081"),
		CronSecret:      getEnv("CRON_SECRET", "change-me-in-production"),
	}

	logger.Info("Configuration loaded",
		zap.Int("port", cfg.Port),
		zap.String("db_host", cfg.DBHost),
		zap.Int("db_port", cfg.DBPort),
		zap.String("epx_base_url", cfg.EPXBaseURL),
	)

	return cfg
}

// initLogger initializes the logger
func initLogger() *zap.Logger {
	env := getEnv("ENVIRONMENT", "development")

	if env == "production" {
		zapCfg := zap.NewProductionConfig()
		zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		logger, _ := zapCfg.Build()
		return logger
	}

	logger, _ := zap.NewDevelopment()
	return logger
}

// initDatabase initializes the PostgreSQL connection pool
func initDatabase(cfg *Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
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
func initDependencies(dbPool *pgxpool.Pool, cfg *Config, logger *zap.Logger) *Dependencies {
	// Initialize database adapter
	dbCfg := database.DefaultPostgreSQLConfig(
		fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.DBUser,
			cfg.DBPassword,
			cfg.DBHost,
			cfg.DBPort,
			cfg.DBName,
			cfg.DBSSLMode,
		),
	)

	dbAdapter, err := database.NewPostgreSQLAdapter(context.Background(), dbCfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database adapter", zap.Error(err))
	}

	// Initialize EPX adapters
	epxEnv := "sandbox"
	if getEnv("ENVIRONMENT", "development") == "production" {
		epxEnv = "production"
	}

	browserPostCfg := epx.DefaultBrowserPostConfig(epxEnv)
	browserPost := epx.NewBrowserPostAdapter(browserPostCfg, logger)

	serverPostCfg := epx.DefaultServerPostConfig(epxEnv)
	serverPost := epx.NewServerPostAdapter(serverPostCfg, logger)

	bricStorageCfg := epx.DefaultBRICStorageConfig(epxEnv)
	bricStorage := epx.NewBRICStorageAdapter(bricStorageCfg, logger)

	// Initialize secret manager (using local file system for development)
	secretManager := secrets.NewLocalSecretManager("./secrets", logger)

	// Initialize North merchant reporting adapter
	merchantReportingCfg := &north.MerchantReportingConfig{
		BaseURL: cfg.NorthAPIURL,
		Timeout: time.Duration(cfg.NorthTimeout) * time.Second,
	}
	httpClient := &http.Client{Timeout: time.Duration(cfg.NorthTimeout) * time.Second}
	loggerAdapter := security.NewZapLogger(logger)
	merchantReporting := north.NewMerchantReportingAdapter(merchantReportingCfg, httpClient, loggerAdapter)

	// Initialize services
	paymentSvc := paymentService.NewPaymentService(
		dbAdapter,
		serverPost,
		secretManager,
		logger,
	)

	subscriptionSvc := subscriptionService.NewSubscriptionService(
		dbAdapter,
		serverPost,
		secretManager,
		logger,
	)

	paymentMethodSvc := paymentmethodService.NewPaymentMethodService(
		dbAdapter,
		browserPost,
		serverPost,
		bricStorage,
		secretManager,
		logger,
	)

	agentSvc := agentService.NewAgentService(
		dbAdapter,
		secretManager,
		logger,
	)

	// Initialize webhook delivery service
	webhookSvc := webhookService.NewWebhookDeliveryService(dbAdapter, nil, logger)

	// Initialize handlers
	paymentHdlr := paymentHandler.NewHandler(paymentSvc, logger)
	subscriptionHdlr := subscriptionHandler.NewHandler(subscriptionSvc, logger)
	paymentMethodHdlr := paymentmethodHandler.NewHandler(paymentMethodSvc, logger)
	agentHdlr := agentHandler.NewHandler(agentSvc, logger)
	chargebackHdlr := chargebackHandler.NewHandler(dbAdapter, logger)

	// Initialize cron handlers (for HTTP endpoints)
	billingCronHdlr := cronHandler.NewBillingHandler(subscriptionSvc, logger, cfg.CronSecret)
	disputeSyncCronHdlr := cronHandler.NewDisputeSyncHandler(merchantReporting, dbAdapter, webhookSvc, logger, cfg.CronSecret)

	// Initialize Browser Post callback handler
	browserPostCallbackHdlr := paymentHandler.NewBrowserPostCallbackHandler(
		dbAdapter,
		browserPost,
		paymentMethodSvc,
		logger,
		browserPostCfg.PostURL, // EPX Browser Post endpoint URL
		cfg.EPXCustNbr,         // EPX Customer Number
		cfg.EPXMerchNbr,        // EPX Merchant Number
		cfg.EPXDBAnbr,          // EPX DBA Number
		cfg.EPXTerminalNbr,     // EPX Terminal Number
		cfg.CallbackBaseURL,    // Base URL for callbacks
	)

	return &Dependencies{
		paymentHandler:             paymentHdlr,
		subscriptionHandler:        subscriptionHdlr,
		paymentMethodHandler:       paymentMethodHdlr,
		agentHandler:               agentHdlr,
		chargebackHandler:          chargebackHdlr,
		billingCronHandler:         billingCronHdlr,
		disputeSyncCronHandler:     disputeSyncCronHdlr,
		browserPostCallbackHandler: browserPostCallbackHdlr,
	}
}

// Interceptors

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
		if err != nil {
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", time.Since(start)),
				zap.Error(err),
			)
		} else {
			logger.Info("gRPC request",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", time.Since(start)),
			)
		}

		return resp, err
	}
}

func recoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered in gRPC handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
				)
				err = fmt.Errorf("internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}
