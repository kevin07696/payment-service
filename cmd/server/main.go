package main

import (
	"context"
	"database/sql"
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
	"github.com/jackc/pgx/v5/stdlib"
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
	authMiddleware "github.com/kevin07696/payment-service/internal/middleware"
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

	// Create sql.DB for auth middleware and cron handlers (needed for standard database/sql interface)
	sqlDB := stdlib.OpenDBFromPool(dbPool)
	defer sqlDB.Close()

	// Initialize dependencies
	deps := initDependencies(dbPool, sqlDB, cfg, logger)

	// Create ConnectRPC HTTP mux
	mux := http.NewServeMux()

	// Initialize authentication interceptor
	var authInterceptor *authMiddleware.AuthInterceptor
	if !cfg.DisableAuth {
		var err error
		authInterceptor, err = authMiddleware.NewAuthInterceptor(sqlDB, logger)
		if err != nil {
			logger.Fatal("Failed to initialize auth interceptor", zap.Error(err))
		}
		logger.Info("Authentication enabled")
	} else {
		logger.Warn("Authentication is DISABLED - for development only!")
	}

	// Create Connect interceptors
	var interceptorList []connect.Interceptor
	interceptorList = append(interceptorList, middleware.RecoveryInterceptor(logger))
	interceptorList = append(interceptorList, middleware.LoggingInterceptor(logger))

	// Add auth interceptor if enabled
	if authInterceptor != nil {
		interceptorList = append(interceptorList, authInterceptor)
	}

	interceptors := connect.WithInterceptors(interceptorList...)

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

	// Initialize EPX callback authentication
	var epxAuth *authMiddleware.EPXCallbackAuth
	if !cfg.DisableAuth && cfg.EPXMacSecret != "" {
		var err error
		epxAuth, err = authMiddleware.NewEPXCallbackAuth(sqlDB, cfg.EPXMacSecret, logger)
		if err != nil {
			logger.Error("Failed to initialize EPX callback auth", zap.Error(err))
		} else {
			logger.Info("EPX callback authentication enabled")
		}
	}

	// Cron endpoints with authentication
	cronAuthMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if cfg.DisableAuth {
				next(w, r)
				return
			}

			// Check cron secret
			secret := r.Header.Get("X-Cron-Secret")
			if secret != cfg.CronSecret {
				logger.Warn("Unauthorized cron request",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}

	httpMux.HandleFunc("/cron/process-billing", cronAuthMiddleware(deps.billingCronHandler.ProcessBilling))
	httpMux.HandleFunc("/cron/sync-disputes", cronAuthMiddleware(deps.disputeSyncCronHandler.SyncDisputes))
	httpMux.HandleFunc("/cron/verify-ach", cronAuthMiddleware(deps.achVerificationCronHandler.VerifyACH))
	httpMux.HandleFunc("/cron/health", cronAuthMiddleware(deps.billingCronHandler.HealthCheck))
	httpMux.HandleFunc("/cron/stats", cronAuthMiddleware(deps.billingCronHandler.Stats))
	httpMux.HandleFunc("/cron/ach/health", cronAuthMiddleware(deps.achVerificationCronHandler.HealthCheck))
	httpMux.HandleFunc("/cron/ach/stats", cronAuthMiddleware(deps.achVerificationCronHandler.Stats))

	// Browser Post endpoints (with rate limiting and EPX auth for callbacks)
	httpMux.HandleFunc("/api/v1/payments/browser-post/form",
		rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.GetPaymentForm))

	// Apply EPX auth to callback endpoint
	var callbackHandler http.HandlerFunc = deps.browserPostCallbackHandler.HandleCallback
	if epxAuth != nil {
		callbackHandler = epxAuth.Middleware(callbackHandler)
	}
	httpMux.HandleFunc("/api/v1/payments/browser-post/callback",
		rateLimiter.HTTPHandlerFunc(callbackHandler))

	// Serve Browser Post demo form (avoids CORS issues with file:// protocol)
	httpMux.HandleFunc("/browser-post-demo", serveBrowserPostDemo)

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

	// Authentication
	AuthSaltPrefix   string // Salt prefix for hashing API keys/secrets
	EPXMacSecret     string // MAC secret for EPX callback signature verification
	DisableAuth      bool   // Disable auth for development/testing
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
	achVerificationCronHandler *cronHandler.ACHVerificationHandler
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
		AuthSaltPrefix:            getEnv("AUTH_SALT_PREFIX", "payment_service_"),
		EPXMacSecret:              getEnv("EPX_MAC_SECRET", ""),
		DisableAuth:               getEnvBool("DISABLE_AUTH", false),
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
func initDependencies(dbPool *pgxpool.Pool, sqlDB *sql.DB, cfg *Config, logger *zap.Logger) *Dependencies {
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
		dbAdapter.Queries(),
		dbAdapter,
		serverPost,
		secretManager,
		merchantResolver,
		logger,
	)

	subscriptionSvc := subscriptionService.NewSubscriptionService(
		dbAdapter.Queries(),
		dbAdapter,
		serverPost,
		secretManager,
		logger,
	)

	paymentMethodSvc := paymentmethodService.NewPaymentMethodService(
		dbAdapter.Queries(),
		dbAdapter,
		browserPost,
		serverPost,
		bricStorage,
		secretManager,
		logger,
	)

	merchantSvc := merchantService.NewMerchantService(
		dbAdapter.Queries(),
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
	achVerificationCronHdlr := cronHandler.NewACHVerificationHandler(sqlDB, logger, cfg.CronSecret)

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
		achVerificationCronHandler: achVerificationCronHdlr,
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// serveBrowserPostDemo serves the Browser Post demo HTML form
// Serving from the server avoids CORS issues with file:// protocol
func serveBrowserPostDemo(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>EPX Browser Post - Demo Form</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 { color: #333; margin-bottom: 10px; }
        .subtitle { color: #666; margin-bottom: 30px; }
        .form-group { margin-bottom: 20px; }
        label {
            display: block;
            margin-bottom: 5px;
            color: #555;
            font-weight: 500;
        }
        input, select {
            width: 100%;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 16px;
            box-sizing: border-box;
        }
        .card-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 10px;
        }
        button {
            background: #4CAF50;
            color: white;
            padding: 12px 30px;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
            width: 100%;
            margin-top: 10px;
        }
        button:hover { background: #45a049; }
        button:disabled {
            background: #ccc;
            cursor: not-allowed;
        }
        .test-cards {
            background: #f9f9f9;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
        }
        .test-card {
            display: flex;
            justify-content: space-between;
            padding: 5px 0;
            font-size: 13px;
        }
        .test-card button {
            padding: 4px 10px;
            font-size: 12px;
            width: auto;
            margin: 0;
        }
        .info {
            background: #e3f2fd;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        .success {
            background: #d4edda;
            border: 1px solid #c3e6cb;
            color: #155724;
            padding: 15px;
            border-radius: 4px;
            margin-top: 20px;
        }
        .error {
            background: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
            padding: 15px;
            border-radius: 4px;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>EPX Browser Post Payment</h1>
        <div class="subtitle">Using TAC Authentication (Working Method)</div>

        <div class="info">
            <strong>How This Works:</strong><br>
            1. Get TAC from payment service<br>
            2. Submit to EPX with TAC + card data<br>
            3. EPX processes and calls back<br>
            4. Transaction complete!
        </div>

        <div class="test-cards">
            <h3>TEST CARDS</h3>
            <div class="test-card">
                <span>Visa (Approved): 4111111111111111</span>
                <button type="button" onclick="fillCard('4111111111111111', '123')">Use</button>
            </div>
            <div class="test-card">
                <span>Visa (Approved): 4788250000028291</span>
                <button type="button" onclick="fillCard('4788250000028291', '123')">Use</button>
            </div>
        </div>

        <div class="form-group">
            <label for="merchantSelect">Merchant</label>
            <select id="merchantSelect">
                <option value="550e8400-e29b-41d4-a716-446655440000">Test Merchant</option>
                <option value="1a20fff8-2cec-48e5-af49-87e501652913">ACME Corporation</option>
            </select>
        </div>

        <div class="form-group">
            <label for="amount">Amount</label>
            <input type="text" id="amount" value="10.00" placeholder="10.00">
        </div>

        <div class="form-group">
            <label for="transactionType">Transaction Type</label>
            <select id="transactionType">
                <option value="SALE">SALE (Auth + Capture)</option>
                <option value="AUTH">AUTH (Hold funds only)</option>
            </select>
        </div>

        <div class="form-group">
            <label for="cardNumber">Card Number</label>
            <input type="text" id="cardNumber" placeholder="4111111111111111" maxlength="16">
        </div>

        <div class="card-row">
            <div class="form-group">
                <label for="expDate">Exp Date (MMYY)</label>
                <input type="text" id="expDate" placeholder="1225" maxlength="4">
            </div>
            <div class="form-group">
                <label for="cvv">CVV</label>
                <input type="text" id="cvv" placeholder="123" maxlength="4">
            </div>
        </div>

        <button id="submitBtn" onclick="processPayment()">Process Payment</button>

        <div id="status"></div>

        <!-- Hidden form that will be auto-submitted to EPX -->
        <form id="epxForm" method="POST" style="display:none;" target="epxWindow">
            <input type="hidden" name="TAC" id="tac">
            <input type="hidden" name="CUST_NBR" id="custNbr">
            <input type="hidden" name="MERCH_NBR" id="merchNbr">
            <input type="hidden" name="DBA_NBR" id="dbaNbr">
            <input type="hidden" name="TERMINAL_NBR" id="terminalNbr">
            <input type="hidden" name="TRAN_NBR" id="tranNbr">
            <input type="hidden" name="TRAN_GROUP" id="tranGroup">
            <input type="hidden" name="AMOUNT" id="amountHidden">
            <input type="hidden" name="CARD_NBR" id="cardNbrHidden">
            <input type="hidden" name="EXP_DATE" id="expDateHidden">
            <input type="hidden" name="CVV" id="cvvHidden">
            <input type="hidden" name="REDIRECT_URL" id="redirectUrl">
            <input type="hidden" name="USER_DATA_1" id="userData1">
            <input type="hidden" name="USER_DATA_2" value="browser-post-demo">
            <input type="hidden" name="USER_DATA_3" id="userData3">
            <input type="hidden" name="INDUSTRY_TYPE" value="E">
        </form>
    </div>

    <script>
        const SERVICE_URL = window.location.origin;

        function fillCard(number, cvv) {
            document.getElementById('cardNumber').value = number;
            document.getElementById('cvv').value = cvv;
            const nextYear = new Date().getFullYear() + 1;
            document.getElementById('expDate').value = '12' + nextYear.toString().substr(-2);
        }

        async function processPayment() {
            const btn = document.getElementById('submitBtn');
            const status = document.getElementById('status');

            btn.disabled = true;
            btn.textContent = 'Processing...';
            status.innerHTML = '<div class="info">Step 1: Getting TAC from payment service...</div>';

            try {
                // Get form values
                const merchantId = document.getElementById('merchantSelect').value;
                const amount = document.getElementById('amount').value;
                const transactionType = document.getElementById('transactionType').value;
                const cardNumber = document.getElementById('cardNumber').value;
                const expDate = document.getElementById('expDate').value;
                const cvv = document.getElementById('cvv').value;

                // Generate transaction ID
                const transactionId = generateUUID();
                const returnUrl = SERVICE_URL + '/api/v1/payments/browser-post/callback';

                // Step 1: Get TAC from payment service
                const formUrl = SERVICE_URL + '/api/v1/payments/browser-post/form?' +
                    'transaction_id=' + transactionId + '&' +
                    'merchant_id=' + merchantId + '&' +
                    'amount=' + amount + '&' +
                    'transaction_type=' + transactionType + '&' +
                    'return_url=' + encodeURIComponent(returnUrl);

                const response = await fetch(formUrl);
                if (!response.ok) {
                    throw new Error('Failed to get TAC: ' + response.status);
                }

                const formConfig = await response.json();

                status.innerHTML += '<div class="success">✅ Got TAC from payment service</div>';
                status.innerHTML += '<div class="info">Step 2: Submitting to EPX...</div>';

                // Step 2: Fill hidden form with EPX data
                const form = document.getElementById('epxForm');
                form.action = formConfig.postURL;

                document.getElementById('tac').value = formConfig.tac;
                document.getElementById('custNbr').value = formConfig.custNbr;
                document.getElementById('merchNbr').value = formConfig.merchNbr;
                document.getElementById('dbaNbr').value = formConfig.dbaName;
                document.getElementById('terminalNbr').value = formConfig.terminalNbr;
                document.getElementById('tranNbr').value = formConfig.epxTranNbr;
                document.getElementById('tranGroup').value = transactionType === 'AUTH' ? 'A' : 'U';
                document.getElementById('amountHidden').value = amount;
                document.getElementById('cardNbrHidden').value = cardNumber;
                document.getElementById('expDateHidden').value = expDate;
                document.getElementById('cvvHidden').value = cvv;
                document.getElementById('redirectUrl').value = returnUrl;
                document.getElementById('userData1').value = returnUrl;
                document.getElementById('userData3').value = merchantId;

                // Step 3: Submit to EPX in popup window
                const epxWindow = window.open('', 'epxWindow', 'width=800,height=600');
                form.submit();

                status.innerHTML += '<div class="success">✅ Form submitted to EPX - check popup window!</div>';
                status.innerHTML += '<div class="info">EPX will process the payment and redirect back to the payment service.</div>';

            } catch (error) {
                status.innerHTML = '<div class="error">❌ Error: ' + error.message + '</div>';
            } finally {
                btn.disabled = false;
                btn.textContent = 'Process Payment';
            }
        }

        function generateUUID() {
            return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
                const r = Math.random() * 16 | 0;
                const v = c == 'x' ? r : (r & 0x3 | 0x8);
                return v.toString(16);
            });
        }

        // Set defaults on load
        window.onload = function() {
            fillCard('4111111111111111', '123');
        };
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
