package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/adapters/epx"
	"github.com/kevin07696/payment-service/internal/adapters/north"
	chargebackHandler "github.com/kevin07696/payment-service/internal/handlers/chargeback"
	cronHandler "github.com/kevin07696/payment-service/internal/handlers/cron"
	merchantHandler "github.com/kevin07696/payment-service/internal/handlers/merchant"
	paymentHandler "github.com/kevin07696/payment-service/internal/handlers/payment"
	paymentmethodHandler "github.com/kevin07696/payment-service/internal/handlers/payment_method"
	subscriptionHandler "github.com/kevin07696/payment-service/internal/handlers/subscription"
	"github.com/kevin07696/payment-service/internal/services/authorization"
	merchantService "github.com/kevin07696/payment-service/internal/services/merchant"
	paymentService "github.com/kevin07696/payment-service/internal/services/payment"
	paymentmethodService "github.com/kevin07696/payment-service/internal/services/payment_method"
	subscriptionService "github.com/kevin07696/payment-service/internal/services/subscription"
	webhookService "github.com/kevin07696/payment-service/internal/services/webhook"
	"github.com/kevin07696/payment-service/pkg/middleware"
	"github.com/kevin07696/payment-service/pkg/security"
	"github.com/kevin07696/payment-service/proto/chargeback/v1/chargebackv1connect"
	"github.com/kevin07696/payment-service/proto/merchant/v1/merchantv1connect"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/proto/payment_method/v1/paymentmethodv1connect"
	"github.com/kevin07696/payment-service/proto/subscription/v1/subscriptionv1connect"
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

	// Create ConnectRPC HTTP mux
	mux := http.NewServeMux()

	// Create Connect interceptors
	interceptors := connect.WithInterceptors(
		middleware.RecoveryInterceptor(logger),
		middleware.LoggingInterceptor(logger),
	)

	// Register all ConnectRPC services
	paymentPath, paymentHandler := paymentv1connect.NewPaymentServiceHandler(
		deps.paymentHandler,
		interceptors,
	)
	mux.Handle(paymentPath, paymentHandler)

	subscriptionPath, subscriptionHandler := subscriptionv1connect.NewSubscriptionServiceHandler(
		deps.subscriptionHandler,
		interceptors,
	)
	mux.Handle(subscriptionPath, subscriptionHandler)

	paymentMethodPath, paymentMethodHandler := paymentmethodv1connect.NewPaymentMethodServiceHandler(
		deps.paymentMethodHandler,
		interceptors,
	)
	mux.Handle(paymentMethodPath, paymentMethodHandler)

	chargebackPath, chargebackHandler := chargebackv1connect.NewChargebackServiceHandler(
		deps.chargebackHandler,
		interceptors,
	)
	mux.Handle(chargebackPath, chargebackHandler)

	merchantPath, merchantHandler := merchantv1connect.NewMerchantServiceHandler(
		deps.merchantHandler,
		interceptors,
	)
	mux.Handle(merchantPath, merchantHandler)

	// Add health check
	checker := grpchealth.NewStaticChecker(
		paymentv1connect.PaymentServiceName,
		subscriptionv1connect.SubscriptionServiceName,
		paymentmethodv1connect.PaymentMethodServiceName,
		chargebackv1connect.ChargebackServiceName,
		merchantv1connect.MerchantServiceName,
	)
	mux.Handle(grpchealth.NewHandler(checker))

	// Add reflection
	reflector := grpcreflect.NewStaticReflector(
		paymentv1connect.PaymentServiceName,
		subscriptionv1connect.SubscriptionServiceName,
		paymentmethodv1connect.PaymentMethodServiceName,
		chargebackv1connect.ChargebackServiceName,
		merchantv1connect.MerchantServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	logger.Info("ConnectRPC services registered",
		zap.String("protocols", "gRPC, Connect, gRPC-Web, HTTP/JSON"),
	)

	// Setup separate HTTP server for cron endpoints and Browser Post callback
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

	// Create ConnectRPC server with H2C support (HTTP/2 without TLS)
	// This allows the server to accept gRPC, Connect, gRPC-Web, and HTTP/JSON requests
	connectServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start ConnectRPC server in goroutine
	go func() {
		logger.Info("ConnectRPC server listening",
			zap.Int("port", cfg.Port),
			zap.String("protocols", "gRPC, Connect, gRPC-Web, HTTP/JSON"),
		)
		if err := connectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve ConnectRPC", zap.Error(err))
		}
	}()

	// Start HTTP server for cron and browser post in goroutine
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown ConnectRPC server
	if err := connectServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("ConnectRPC server shutdown error", zap.Error(err))
	}

	// Shutdown HTTP server
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

	// EPX Payment Gateway (Server Post API for transactions)
	EPXServerPostURL  string // EPX Server Post API URL (e.g., https://secure.epxuap.com)
	EPXKeyExchangeURL string // EPX Key Exchange URL (e.g., https://keyexch.epxuap.com)
	EPXBrowserPostURL string // EPX Browser Post URL (e.g., https://services.epxuap.com/browserpost/)
	EPXTimeout        int
	EPXCustNbr        string // EPX Customer Number
	EPXMerchNbr       string // EPX Merchant Number
	EPXDBAnbr         string // EPX DBA Number
	EPXTerminalNbr    string // EPX Terminal Number

	// North Merchant Reporting API (for disputes/chargebacks, NOT payments)
	NorthMerchantReportingURL string // North Reporting API URL (e.g., https://api.north.com)
	NorthTimeout              int

	// Browser Post Configuration
	CallbackBaseURL string // Base URL for Browser Post callbacks (e.g., "http://localhost:8081")

	// Cron authentication
	CronSecret string
}

// Dependencies holds all initialized services and handlers
type Dependencies struct {
	paymentHandler             *paymentHandler.ConnectHandler
	subscriptionHandler        *subscriptionHandler.ConnectHandler
	paymentMethodHandler       *paymentmethodHandler.ConnectHandler
	chargebackHandler          *chargebackHandler.ConnectHandler
	merchantHandler            *merchantHandler.ConnectHandler
	billingCronHandler         *cronHandler.BillingHandler
	disputeSyncCronHandler     *cronHandler.DisputeSyncHandler
	browserPostCallbackHandler *paymentHandler.BrowserPostCallbackHandler
}

// loadConfig loads configuration from environment variables
func loadConfig(logger *zap.Logger) *Config {
	cfg := &Config{
		Port:       getEnvInt("PORT", 8080),
		HTTPPort:   getEnvInt("HTTP_PORT", 8081),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "payment_service"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),
		MaxConns:   int32(getEnvInt("DB_MAX_CONNS", 25)),
		MinConns:   int32(getEnvInt("DB_MIN_CONNS", 5)),
		// EPX URLs are required - no fallbacks to ensure proper configuration
		EPXServerPostURL:          getEnvWithFallback("EPX_SERVER_POST_URL", "EPX_BASE_URL", ""),
		EPXKeyExchangeURL:         getEnv("EPX_KEY_EXCHANGE_URL", ""),
		EPXBrowserPostURL:         getEnv("EPX_BROWSER_POST_URL", ""),
		EPXTimeout:                getEnvInt("EPX_TIMEOUT", 30),
		EPXCustNbr:                getEnv("EPX_CUST_NBR", "9001"),    // EPX sandbox customer number
		EPXMerchNbr:               getEnv("EPX_MERCH_NBR", "900300"), // EPX sandbox merchant number
		EPXDBAnbr:                 getEnv("EPX_DBA_NBR", "2"),        // EPX sandbox DBA number
		EPXTerminalNbr:            getEnv("EPX_TERMINAL_NBR", "77"),  // EPX sandbox terminal number
		NorthMerchantReportingURL: getEnvWithFallback("NORTH_MERCHANT_REPORTING_URL", "NORTH_API_URL", "https://api.north.com"),
		NorthTimeout:              getEnvInt("NORTH_TIMEOUT", 30),
		CallbackBaseURL:           getEnv("CALLBACK_BASE_URL", "http://localhost:8081"),
		CronSecret:                getEnv("CRON_SECRET", "change-me-in-production"),
	}

	logger.Info("Configuration loaded",
		zap.Int("port", cfg.Port),
		zap.String("db_host", cfg.DBHost),
		zap.Int("db_port", cfg.DBPort),
		zap.String("epx_server_post_url", cfg.EPXServerPostURL),
		zap.String("north_merchant_reporting_url", cfg.NorthMerchantReportingURL),
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

	// Initialize EPX adapters with environment-specific configuration
	epxEnv := "sandbox"
	if getEnv("ENVIRONMENT", "development") == "production" {
		epxEnv = "production"
	}

	// Server Post adapter configuration
	serverPostCfg := epx.DefaultServerPostConfig(epxEnv)
	serverPostCfg.BaseURL = cfg.EPXServerPostURL // Override with env var
	serverPost := epx.NewServerPostAdapter(serverPostCfg, logger)

	// Browser Post adapter configuration
	browserPostCfg := epx.DefaultBrowserPostConfig(epxEnv)
	browserPostCfg.PostURL = cfg.EPXBrowserPostURL // Use env var directly - no fallback
	browserPost := epx.NewBrowserPostAdapter(browserPostCfg, logger)

	// Key Exchange adapter configuration
	keyExchangeCfg := epx.DefaultKeyExchangeConfig(epxEnv)
	keyExchangeCfg.BaseURL = cfg.EPXKeyExchangeURL // Use env var directly - no fallback
	keyExchange := epx.NewKeyExchangeAdapter(keyExchangeCfg, logger)

	// BRIC Storage adapter configuration
	bricStorageCfg := epx.DefaultBRICStorageConfig(epxEnv)
	bricStorageCfg.BaseURL = cfg.EPXServerPostURL // Same as Server Post
	bricStorage := epx.NewBRICStorageAdapter(bricStorageCfg, logger)

	// Initialize secret manager based on environment
	// Supports: GCP Secret Manager (production) or Mock (development)
	secretManager := initSecretManager(context.Background(), cfg, logger)

	// Initialize North merchant reporting adapter
	merchantReportingCfg := &north.MerchantReportingConfig{
		BaseURL: cfg.NorthMerchantReportingURL,
		Timeout: time.Duration(cfg.NorthTimeout) * time.Second,
	}
	httpClient := &http.Client{Timeout: time.Duration(cfg.NorthTimeout) * time.Second}
	loggerAdapter := security.NewZapLogger(logger)
	merchantReporting := north.NewMerchantReportingAdapter(merchantReportingCfg, httpClient, loggerAdapter)

	// Initialize authorization resolver
	merchantResolver := authorization.NewMerchantResolver()

	// Initialize services
	paymentSvc := paymentService.NewPaymentService(
		dbAdapter,
		serverPost,
		secretManager,
		merchantResolver,
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

	merchantSvc := merchantService.NewMerchantService(
		dbAdapter,
		secretManager,
		logger,
	)

	// Initialize webhook delivery service
	webhookSvc := webhookService.NewWebhookDeliveryService(dbAdapter, nil, logger)

	// Initialize ConnectRPC handlers
	paymentHdlr := paymentHandler.NewConnectHandler(paymentSvc, logger)
	subscriptionHdlr := subscriptionHandler.NewConnectHandler(subscriptionSvc, logger)
	paymentMethodHdlr := paymentmethodHandler.NewConnectHandler(paymentMethodSvc, logger)
	chargebackHdlr := chargebackHandler.NewConnectHandler(dbAdapter, logger)
	merchantHdlr := merchantHandler.NewConnectHandler(merchantSvc, logger)

	// Initialize cron handlers (for HTTP endpoints)
	billingCronHdlr := cronHandler.NewBillingHandler(subscriptionSvc, logger, cfg.CronSecret)
	disputeSyncCronHdlr := cronHandler.NewDisputeSyncHandler(merchantReporting, dbAdapter, webhookSvc, logger, cfg.CronSecret)

	// Initialize Browser Post callback handler
	browserPostCallbackHdlr := paymentHandler.NewBrowserPostCallbackHandler(
		dbAdapter,
		browserPost,
		keyExchange,
		secretManager, // Secret manager for fetching merchant-specific MACs
		paymentMethodSvc,
		logger,
		browserPostCfg.PostURL, // EPX Browser Post endpoint URL
		cfg.CallbackBaseURL,    // Base URL for callbacks
	)

	return &Dependencies{
		paymentHandler:             paymentHdlr,
		subscriptionHandler:        subscriptionHdlr,
		paymentMethodHandler:       paymentMethodHdlr,
		chargebackHandler:          chargebackHdlr,
		merchantHandler:            merchantHdlr,
		billingCronHandler:         billingCronHdlr,
		disputeSyncCronHandler:     disputeSyncCronHdlr,
		browserPostCallbackHandler: browserPostCallbackHdlr,
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

// getEnvWithFallback tries the primary key first, then fallback key, then default value
// This provides backwards compatibility when renaming environment variables
func getEnvWithFallback(primaryKey, fallbackKey, defaultValue string) string {
	if value := os.Getenv(primaryKey); value != "" {
		return value
	}
	if value := os.Getenv(fallbackKey); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultMinutes int) time.Duration {
	minutes := getEnvInt(key, defaultMinutes)
	return time.Duration(minutes) * time.Minute
}
