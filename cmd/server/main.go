package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
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
	"github.com/kevin07696/payment-service/internal/db/seed"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackHandler "github.com/kevin07696/payment-service/internal/handlers/chargeback"
	cronHandler "github.com/kevin07696/payment-service/internal/handlers/cron"
	merchantHandler "github.com/kevin07696/payment-service/internal/handlers/merchant"
	paymentHandler "github.com/kevin07696/payment-service/internal/handlers/payment"
	paymentmethodHandler "github.com/kevin07696/payment-service/internal/handlers/payment_method"
	subscriptionHandler "github.com/kevin07696/payment-service/internal/handlers/subscription"
	authMiddleware "github.com/kevin07696/payment-service/internal/middleware"
	browserpostService "github.com/kevin07696/payment-service/internal/services/browser_post"
	merchantService "github.com/kevin07696/payment-service/internal/services/merchant"
	paymentService "github.com/kevin07696/payment-service/internal/services/payment"
	paymentmethodService "github.com/kevin07696/payment-service/internal/services/payment_method"
	subscriptionService "github.com/kevin07696/payment-service/internal/services/subscription"
	webhookService "github.com/kevin07696/payment-service/internal/services/webhook"
	pkghttp "github.com/kevin07696/payment-service/pkg/http"
	"github.com/kevin07696/payment-service/pkg/middleware"
	"github.com/kevin07696/payment-service/pkg/observability"
	"github.com/kevin07696/payment-service/pkg/resilience"
	"github.com/kevin07696/payment-service/pkg/resourcemgmt"
	"github.com/kevin07696/payment-service/pkg/security"
	"github.com/kevin07696/payment-service/pkg/shutdown"
	"github.com/kevin07696/payment-service/proto/chargeback/v1/chargebackv1connect"
	"github.com/kevin07696/payment-service/proto/merchant/v1/merchantv1connect"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/proto/payment_method/v1/paymentmethodv1connect"
	"github.com/kevin07696/payment-service/proto/subscription/v1/subscriptionv1connect"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer func() { _ = logger.Sync() }()

	logger.Info("Starting payment service",
		zap.String("version", "0.1.0"),
	)

	// Load configuration from environment
	cfg := loadConfig(logger)

	// Initialize OpenTelemetry distributed tracing
	tracingConfig := observability.TracingConfig{
		ServiceName:    "payment-service",
		ServiceVersion: "0.1.0",
		Environment:    getEnv("ENVIRONMENT", "development"),
		Endpoint:       getEnv("OTLP_ENDPOINT", "localhost:4318"),
		Enabled:        getEnv("TRACING_ENABLED", "false") == "true",
		SampleRate:     getTracingSampleRate(),
	}

	_, shutdownTracing, err := observability.InitTracing(context.Background(), tracingConfig)
	if err != nil {
		logger.Fatal("Failed to initialize tracing", zap.Error(err))
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			logger.Error("Error shutting down tracing", zap.Error(err))
		}
	}()

	if tracingConfig.Enabled {
		logger.Info("Distributed tracing enabled",
			zap.String("endpoint", tracingConfig.Endpoint),
			zap.Float64("sample_rate", tracingConfig.SampleRate),
		)
	} else {
		logger.Info("Distributed tracing disabled (set TRACING_ENABLED=true to enable)")
	}

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

	// Create sqlc queries object
	queries := sqlc.New(dbPool)

	// Initialize dependencies
	deps := initDependencies(dbPool, sqlDB, queries, cfg, logger)

	// Create ConnectRPC HTTP mux
	mux := http.NewServeMux()

	// Initialize authentication interceptor
	var authInterceptor *authMiddleware.AuthInterceptor
	if cfg.AuthEnabled {
		var err error
		authInterceptor, err = authMiddleware.NewAuthInterceptor(queries, logger)
		if err != nil {
			logger.Fatal("Failed to initialize auth interceptor", zap.Error(err))
		}
		logger.Info("Authentication enabled")
	} else {
		logger.Warn("Authentication is DISABLED - for development only!")
	}

	// Create Connect interceptors
	// Order matters: outermost (first) to innermost (last)
	// 1. Timeout - outermost, enforces request deadlines
	// 2. Tracing - extracts/injects trace context, creates spans
	// 3. Recovery - catches panics before they crash the server
	// 4. Logging - logs request/response with correlation IDs
	// 5. Auth - validates JWT/API key (innermost before handler)
	var interceptorList []connect.Interceptor

	// Add timeout interceptor first (outermost layer)
	timeoutConfig := resilience.DefaultTimeoutConfig()
	timeoutInterceptor := middleware.NewTimeoutInterceptor(timeoutConfig, logger)
	interceptorList = append(interceptorList, timeoutInterceptor)

	// Add tracing interceptor (extracts trace context, creates spans)
	interceptorList = append(interceptorList, observability.TracingInterceptor())

	// Recovery and logging interceptors (logging now includes trace_id, request_id)
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

	// Initialize in-flight request tracking (P2-5) - Zero-downtime deployments
	serverTracker := shutdown.NewHTTPInFlightTracker("server", logger)

	// Create rate limiter (10 requests per second per IP, burst of 20)
	// Adjust these values based on expected staging traffic
	rateLimiter := middleware.NewRateLimiter(10, 20)

	// Note: EPX callback MAC validation is performed in the service layer
	// (BrowserPostService.ProcessCallback) rather than middleware, because:
	// 1. EPX includes MAC in form parameters, not HTTP headers
	// 2. Each merchant has their own MAC secret (per-merchant validation)
	// 3. Browser Post redirects come from user's browser, not EPX servers

	// Cron endpoints with authentication
	cronAuthMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !cfg.AuthEnabled {
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

	// Cron endpoints requiring authentication (added to main mux)
	mux.HandleFunc("/cron/process-billing", cronAuthMiddleware(deps.billingCronHandler.ProcessBilling))
	mux.HandleFunc("/cron/process-expired-past-due", cronAuthMiddleware(deps.billingCronHandler.ProcessExpiredPastDue))
	mux.HandleFunc("/cron/sync-disputes", cronAuthMiddleware(deps.disputeSyncCronHandler.SyncDisputes))
	mux.HandleFunc("/cron/verify-ach", cronAuthMiddleware(deps.achVerificationCronHandler.VerifyACH))
	mux.HandleFunc("/cron/prenote-retry", cronAuthMiddleware(deps.prenoteRetryCronHandler.RetryPrenotes))
	mux.HandleFunc("/cron/cleanup-audit-logs", cronAuthMiddleware(deps.auditCleanupCronHandler.CleanupAuditLogs))
	mux.HandleFunc("/cron/cleanup-rate-limits", cronAuthMiddleware(deps.rateLimitCleanupCronHandler.CleanupRateLimitBuckets))
	mux.HandleFunc("/cron/stats", cronAuthMiddleware(deps.billingCronHandler.Stats))
	mux.HandleFunc("/cron/ach/stats", cronAuthMiddleware(deps.achVerificationCronHandler.Stats))
	mux.HandleFunc("/cron/prenote/stats", cronAuthMiddleware(deps.prenoteRetryCronHandler.Stats))
	mux.HandleFunc("/cron/audit/stats", cronAuthMiddleware(deps.auditCleanupCronHandler.Stats))
	mux.HandleFunc("/cron/rate-limit/stats", cronAuthMiddleware(deps.rateLimitCleanupCronHandler.Stats))

	// Health endpoints (no auth required for monitoring/load balancers)
	// Wrap health checks with draining awareness - returns 503 during shutdown
	// This allows load balancers to remove instances from rotation before draining
	mux.Handle("/cron/health", serverTracker.DrainingHealthCheck(http.HandlerFunc(deps.billingCronHandler.HealthCheck)))
	mux.Handle("/cron/ach/health", serverTracker.DrainingHealthCheck(http.HandlerFunc(deps.achVerificationCronHandler.HealthCheck)))
	mux.Handle("/cron/audit/health", serverTracker.DrainingHealthCheck(http.HandlerFunc(deps.auditCleanupCronHandler.HealthCheck)))
	mux.Handle("/cron/rate-limit/health", serverTracker.DrainingHealthCheck(http.HandlerFunc(deps.rateLimitCleanupCronHandler.HealthCheck)))

	// Browser Post endpoints (with rate limiting)
	// Note: Browser Post callbacks come from user's browser (via EPX 302 redirect)
	// and cannot include custom HTTP headers. MAC signature validation is performed
	// in the service layer (BrowserPostService.ProcessCallback) using the MAC field
	// from form parameters, which is signed by EPX using the merchant's MAC secret.
	mux.HandleFunc("/api/v1/payments/browser-post/form",
		rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.GetPaymentForm))

	mux.HandleFunc("/api/v1/payments/browser-post/callback",
		rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.HandleCallback))

	// Serve Browser Post demo form (avoids CORS issues with file:// protocol)
	mux.HandleFunc("/browser-post-demo", serveBrowserPostDemo)

	// Initialize security headers middleware
	isDevelopment := getEnv("ENVIRONMENT", "development") != "production"
	securityHeaders := authMiddleware.NewSecurityHeaders(isDevelopment)

	// Initialize compression middleware (P2-4) - 40-60% bandwidth reduction
	// NOTE: ConnectRPC paths are excluded because Connect has its own built-in compression
	// that uses Connect-Content-Encoding header. Using both causes double compression.
	gzipConfig := middleware.DefaultGzipConfig()
	gzipConfig.ExcludedPaths = []string{
		"/cron/health",
		"/cron/ach/health",
		// Exclude all ConnectRPC service paths - Connect handles compression internally
		"/payment.v1.PaymentService/",
		"/subscription.v1.SubscriptionService/",
		"/paymentmethod.v1.PaymentMethodService/",
		"/chargeback.v1.ChargebackService/",
		"/merchant.v1.MerchantService/",
	}
	compressionMiddleware := middleware.GzipHandlerWithCustomConfig(gzipConfig, logger)

	// Request size limits for DOS protection
	const maxRequestBodySize = 1 << 20 // 1 MB limit for request bodies

	// Create unified server with H2C support (HTTP/2 without TLS)
	// This allows the server to accept gRPC, Connect, gRPC-Web, HTTP/JSON, and REST requests
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		// Middleware chain (innermost to outermost):
		// 1. mux (ConnectRPC + REST routes)
		// 2. h2c.NewHandler (HTTP/2 cleartext)
		// 3. compressionMiddleware (gzip)
		// 4. securityHeaders (CSP, HSTS, etc.)
		// 5. serverTracker (in-flight request tracking + draining)
		// 6. MaxBytesHandler (request size limit)
		Handler:           http.MaxBytesHandler(serverTracker.Middleware(securityHeaders.Middleware(compressionMiddleware(h2c.NewHandler(mux, &http2.Server{})))), maxRequestBodySize),
		ReadTimeout:       65 * time.Second, // Slightly longer than handler timeout (60s)
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB max header size
	}

	// Initialize graceful shutdown manager (P2-5) - Zero-downtime deployments
	// CRITICAL: Create shutdown manager BEFORE starting servers to handle early SIGTERM
	shutdownMgr := shutdown.NewManager(logger, 30*time.Second)

	// Prepare goroutine leak monitoring (P2-6) - will start after registering shutdown handlers
	// This prevents race condition where early SIGTERM could arrive before handlers are registered
	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	monitorStarted := make(chan struct{})

	// Register shutdown components in proper LIFO order
	// Components registered first shut down LAST
	// This ensures proper dependency ordering

	// 1. Database (shut down last - everything depends on it)
	shutdownMgr.Register("database", func(ctx context.Context) error {
		deps.dbAdapter.Close() // Close doesn't return error
		return nil
	})

	// 2. Background services
	shutdownMgr.Register("goroutine_tracker", func(ctx context.Context) error {
		cancelMonitor() // Stop monitoring
		return nil
	})

	if authInterceptor != nil {
		shutdownMgr.Register("auth_interceptor", func(ctx context.Context) error {
			authInterceptor.Shutdown()
			return nil
		})
	}

	if deps.disputeSyncCronHandler != nil {
		shutdownMgr.Register("dispute_sync_handler", func(ctx context.Context) error {
			deps.disputeSyncCronHandler.Shutdown()
			return nil
		})
	}

	if rateLimiter != nil {
		shutdownMgr.Register("rate_limiter", func(ctx context.Context) error {
			rateLimiter.Shutdown()
			return nil
		})
	}

	// 3. HTTP server (shut down first - stop accepting new requests)
	// Use draining shutdown: first drain in-flight requests, then shutdown server
	// This enables zero-downtime deployments by completing all active requests
	shutdownMgr.RegisterHTTPServerWithDraining("server", server, serverTracker)

	logger.Info("Shutdown manager initialized - all components registered")

	// NOW start goroutine monitoring (after shutdown handler is registered)
	// This ensures clean shutdown even if SIGTERM arrives during startup
	go func() {
		close(monitorStarted) // Signal that goroutine has started
		deps.goroutineTracker.StartMonitoring(monitorCtx)
	}()
	<-monitorStarted // Wait for monitoring to start
	logger.Info("Goroutine leak monitoring started")

	// NOW start server (after shutdown manager is ready)
	go func() {
		logger.Info("Server listening",
			zap.Int("port", cfg.Port),
			zap.String("protocols", "gRPC, Connect, gRPC-Web, HTTP/JSON, REST"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Wait for shutdown signal and execute graceful shutdown
	shutdownMgr.WaitForShutdown()
}

// Config holds application configuration
type Config struct {
	Port int // Server port for all endpoints (ConnectRPC + REST)

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	MaxConns   int32
	MinConns   int32

	// EPX Payment Gateway - endpoints configured via environment variables in adapters
	// Requires: EPX_SERVER_POST_ENDPOINT, EPX_SERVER_POST_SOCKET_ENDPOINT,
	//           EPX_BROWSER_POST_ENDPOINT, EPX_KEY_EXCHANGE_ENDPOINT
	EPXTimeout     int
	EPXCustNbr     string
	EPXMerchNbr    string
	EPXDBAnbr      string
	EPXTerminalNbr string

	// North Merchant Reporting API (for disputes/chargebacks)
	NorthMerchantReportingURL string
	NorthTimeout              int

	// Browser Post Configuration
	CallbackBaseURL string

	// Cron configuration
	CronSecret            string
	CronJobTimeoutSeconds int
	CronDefaultBatchSize  int
	CronMaxBatchSize      int

	// Subscription configuration
	SubscriptionDefaultMaxRetries  int
	SubscriptionDefaultGracePeriod int
	SubscriptionRetryBaseDelaySecs int
	SubscriptionRetryMaxDelaySecs  int
	SubscriptionRetryMultiplier    float64

	// Authentication
	AuthSaltPrefix string
	EPXMacSecret   string
	AuthEnabled    bool
}

// Dependencies holds all initialized services and handlers
type Dependencies struct {
	dbAdapter                   *database.PostgreSQLAdapter
	goroutineTracker            *resourcemgmt.GoroutineTracker
	merchantCache               *merchantService.MerchantCredentialCache
	paymentMethodCache          *paymentmethodService.PaymentMethodCache
	paymentHandler              *paymentHandler.ConnectHandler
	subscriptionHandler         *subscriptionHandler.ConnectHandler
	paymentMethodHandler        *paymentmethodHandler.ConnectHandler
	chargebackHandler           *chargebackHandler.ConnectHandler
	merchantHandler             *merchantHandler.ConnectHandler
	billingCronHandler          *cronHandler.BillingHandler
	disputeSyncCronHandler      *cronHandler.DisputeSyncHandler
	achVerificationCronHandler  *cronHandler.ACHVerificationHandler
	prenoteRetryCronHandler     *cronHandler.PrenoteRetryHandler
	auditCleanupCronHandler     *cronHandler.AuditCleanupHandler
	rateLimitCleanupCronHandler *cronHandler.RateLimitCleanupHandler
	browserPostCallbackHandler  *paymentHandler.BrowserPostCallbackHandler
}

// loadConfig loads configuration from environment variables
// All environment variables are required - no defaults to avoid configuration confusion
func loadConfig(logger *zap.Logger) *Config {
	cfg := &Config{
		// Port (unified for ConnectRPC + REST)
		Port: requireEnvInt("PORT", logger),

		// Database
		DBHost:     requireEnv("DB_HOST", logger),
		DBPort:     requireEnvInt("DB_PORT", logger),
		DBUser:     requireEnv("DB_USER", logger),
		DBPassword: requireEnv("DB_PASSWORD", logger),
		DBName:     requireEnv("DB_NAME", logger),
		DBSSLMode:  requireEnv("DB_SSL_MODE", logger),
		MaxConns:   int32(requireEnvInt("DB_MAX_CONNS", logger)),
		MinConns:   int32(requireEnvInt("DB_MIN_CONNS", logger)),

		// EPX configuration (endpoints configured in adapters via EPX_*_ENDPOINT env vars)
		EPXTimeout:     requireEnvInt("EPX_TIMEOUT", logger),
		EPXCustNbr:     requireEnv("EPX_CUST_NBR", logger),
		EPXMerchNbr:    requireEnv("EPX_MERCH_NBR", logger),
		EPXDBAnbr:      requireEnv("EPX_DBA_NBR", logger),
		EPXTerminalNbr: requireEnv("EPX_TERMINAL_NBR", logger),

		// North Reporting API
		NorthMerchantReportingURL: requireEnv("NORTH_MERCHANT_REPORTING_URL", logger),
		NorthTimeout:              requireEnvInt("NORTH_TIMEOUT", logger),

		// Browser POST callback
		CallbackBaseURL: requireEnv("CALLBACK_BASE_URL", logger),

		// Cron configuration
		CronSecret:            requireEnv("CRON_SECRET", logger),
		CronJobTimeoutSeconds: requireEnvInt("CRON_JOB_TIMEOUT_SECONDS", logger),
		CronDefaultBatchSize:  requireEnvInt("CRON_DEFAULT_BATCH_SIZE", logger),
		CronMaxBatchSize:      requireEnvInt("CRON_MAX_BATCH_SIZE", logger),

		// Subscription configuration
		SubscriptionDefaultMaxRetries:  requireEnvInt("SUBSCRIPTION_DEFAULT_MAX_RETRIES", logger),
		SubscriptionDefaultGracePeriod: requireEnvInt("SUBSCRIPTION_DEFAULT_GRACE_PERIOD_DAYS", logger),
		SubscriptionRetryBaseDelaySecs: requireEnvInt("SUBSCRIPTION_RETRY_BASE_DELAY_SECS", logger),
		SubscriptionRetryMaxDelaySecs:  requireEnvInt("SUBSCRIPTION_RETRY_MAX_DELAY_SECS", logger),
		SubscriptionRetryMultiplier:    requireEnvFloat("SUBSCRIPTION_RETRY_MULTIPLIER", logger),

		// Authentication
		AuthSaltPrefix: requireEnv("AUTH_SALT_PREFIX", logger),
		EPXMacSecret:   getEnv("EPX_SANDBOX_MAC", ""), // Optional: Merchant Authorization Code for sandbox callback auth
		AuthEnabled:    requireEnvBool("AUTH_ENABLED", logger),
	}

	// Validate CRON_SECRET security requirements
	// Empty check - redundant with requireEnv but explicit for test verification
	if cfg.CronSecret == "" {
		logger.Fatal("CRON_SECRET environment variable is required",
			zap.String("suggestion", "Generate with: openssl rand -base64 32"),
		)
	}

	// Default value check - prevent production use of placeholder
	if cfg.CronSecret == "change-me-in-production" {
		logger.Fatal("CRON_SECRET must be changed from default value",
			zap.String("suggestion", "Generate with: openssl rand -base64 32"),
		)
	}

	// Minimum length check - ensure sufficient entropy (256 bits recommended)
	if len(cfg.CronSecret) < 32 {
		logger.Fatal("CRON_SECRET must be at least 32 characters for sufficient entropy",
			zap.Int("current_length", len(cfg.CronSecret)),
			zap.Int("required_length", 32),
			zap.String("suggestion", "Generate with: openssl rand -base64 32"),
		)
	}

	logger.Info("Configuration loaded",
		zap.Int("port", cfg.Port),
		zap.String("db_host", cfg.DBHost),
		zap.Int("db_port", cfg.DBPort),
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
func initDependencies(dbPool *pgxpool.Pool, sqlDB *sql.DB, queries *sqlc.Queries, cfg *Config, logger *zap.Logger) *Dependencies {
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

	// Start database connection pool monitoring (checks every 30 seconds)
	dbAdapter.StartPoolMonitoring(context.Background(), 30*time.Second)

	// Initialize EPX adapters with environment-specific configuration
	epxEnv := "sandbox"
	if getEnv("ENVIRONMENT", "development") == "production" {
		epxEnv = "production"
	}

	// Server Post adapter configuration
	serverPostCfg, err := epx.DefaultServerPostConfig(epxEnv)
	if err != nil {
		logger.Fatal("Failed to create Server Post config", zap.Error(err))
	}
	serverPost := epx.NewServerPostAdapter(serverPostCfg, logger)

	// Browser Post adapter configuration
	browserPostCfg, err := epx.DefaultBrowserPostConfig(epxEnv)
	if err != nil {
		logger.Fatal("Failed to create Browser Post config", zap.Error(err))
	}
	browserPost := epx.NewBrowserPostAdapter(browserPostCfg, logger)

	// Key Exchange adapter configuration
	keyExchangeCfg, err := epx.DefaultKeyExchangeConfig(epxEnv)
	if err != nil {
		logger.Fatal("Failed to create Key Exchange config", zap.Error(err))
	}
	keyExchange := epx.NewKeyExchangeAdapter(keyExchangeCfg, logger)

	// Business Reporting adapter configuration (for ACH return checks)
	businessReportingCfg := epx.DefaultBusinessReportingConfig(epxEnv)
	businessReportingCfg.BaseURL = cfg.NorthMerchantReportingURL
	businessReportingCfg.CustNbr = cfg.EPXCustNbr
	businessReportingCfg.MerchNbr = cfg.EPXMerchNbr
	businessReportingCfg.DBAnbr = cfg.EPXDBAnbr
	businessReportingCfg.Timeout = time.Duration(cfg.NorthTimeout) * time.Second
	// API credentials will be needed from environment if using API key auth
	businessReportingCfg.APIKey = getEnv("EPX_API_KEY", "")
	businessReportingCfg.APISecret = getEnv("EPX_API_SECRET", "")
	businessReporting := epx.NewBusinessReportingAdapter(businessReportingCfg, logger)

	// Initialize secret manager based on environment
	// Supports: GCP Secret Manager (production) or Mock (development)
	secretManager := initSecretManager(context.Background(), cfg, logger)

	// Auto-seed sandbox merchant in development/staging (not production)
	seeder := seed.NewSeeder(queries, secretManager, logger)
	if err := seeder.SeedIfNeeded(context.Background()); err != nil {
		logger.Error("Failed to auto-seed sandbox merchant", zap.Error(err))
		// Non-fatal: continue startup even if seeding fails
	}

	// Seed test data for API documentation (service, subscriptions, etc.)
	if err := seeder.SeedTestData(context.Background()); err != nil {
		logger.Error("Failed to seed test data", zap.Error(err))
		// Non-fatal: continue startup even if seeding fails
	}

	// Initialize P2 optimizations

	// Goroutine leak detection (P2-6)
	goroutineTracker := resourcemgmt.NewGoroutineTracker(logger, resourcemgmt.DefaultConfig())
	// Monitoring will be started in main()

	// Merchant credential cache (P2-1) - 70% DB load reduction
	merchantCache := merchantService.NewMerchantCredentialCache(
		queries,
		secretManager,
		logger,
		5*time.Minute, // 5 minute TTL
		1000,          // 1000 merchants max
	)

	// Payment method cache (P2-2) - 60% faster lookups
	paymentMethodCache := paymentmethodService.NewPaymentMethodCache(
		queries,
		logger,
		2*time.Minute, // 2 minute TTL (shorter for fresher data)
		10000,         // 10,000 payment methods max
	)

	// Initialize North merchant reporting adapter
	merchantReportingCfg := &north.MerchantReportingConfig{
		BaseURL: cfg.NorthMerchantReportingURL,
		Timeout: time.Duration(cfg.NorthTimeout) * time.Second,
	}
	// Use optimized HTTP client config (P2-3) - 90%+ connection reuse
	northHTTPClient := pkghttp.NewHTTPClient(
		pkghttp.DefaultClientConfig(),
		time.Duration(cfg.NorthTimeout)*time.Second,
	)
	loggerAdapter := security.NewZapLogger(logger)
	merchantReporting := north.NewMerchantReportingAdapter(merchantReportingCfg, northHTTPClient, loggerAdapter)

	// Initialize services with caches
	paymentSvc := paymentService.NewPaymentService(
		dbAdapter.Queries(),
		dbAdapter,
		serverPost,
		secretManager,
		merchantCache, // P2-1: Merchant credential cache (70% DB load reduction)
		logger,
		nil, // Use default config (DEFAULT_ACH_CLASS env var or "WEB")
	)

	subscriptionSvc := subscriptionService.NewSubscriptionService(
		dbAdapter.Queries(),
		dbAdapter,
		serverPost,
		secretManager,
		logger,
		&subscriptionService.BillingRetryConfig{
			BaseDelaySecs: cfg.SubscriptionRetryBaseDelaySecs,
			MaxDelaySecs:  cfg.SubscriptionRetryMaxDelaySecs,
			Multiplier:    cfg.SubscriptionRetryMultiplier,
		},
	)

	paymentMethodSvc := paymentmethodService.NewPaymentMethodService(
		dbAdapter.Queries(),
		dbAdapter,
		browserPost,
		serverPost,
		secretManager,
		paymentMethodCache, // P2-2: Payment method cache (60% faster lookups)
		logger,
	)

	merchantSvc := merchantService.NewMerchantService(
		dbAdapter.Queries(),
		dbAdapter,
		secretManager,
		logger,
	)

	// Initialize webhook delivery service with optimized HTTP client (P2-3)
	webhookHTTPClient := pkghttp.NewHTTPClient(
		pkghttp.WebhookClientConfig(), // Optimized for many different hosts
		10*time.Second,                // Request timeout
	)
	webhookSvc := webhookService.NewWebhookDeliveryService(dbAdapter, webhookHTTPClient, logger)

	// Initialize ConnectRPC handlers
	paymentHdlr := paymentHandler.NewConnectHandler(paymentSvc, logger)
	subscriptionHdlr := subscriptionHandler.NewConnectHandler(subscriptionSvc, logger, subscriptionHandler.ConnectHandlerConfig{
		DefaultMaxRetries: cfg.SubscriptionDefaultMaxRetries,
	})
	paymentMethodHdlr := paymentmethodHandler.NewConnectHandler(paymentMethodSvc, logger)
	chargebackHdlr := chargebackHandler.NewConnectHandler(dbAdapter, logger)
	merchantHdlr := merchantHandler.NewConnectHandler(merchantSvc, logger)

	// Initialize cron handlers (for HTTP endpoints)
	billingCronHdlr := cronHandler.NewBillingHandler(subscriptionSvc, queries, logger, cronHandler.BillingHandlerConfig{
		CronSecret:       cfg.CronSecret,
		JobTimeoutSecs:   cfg.CronJobTimeoutSeconds,
		DefaultBatchSize: cfg.CronDefaultBatchSize,
		MaxBatchSize:     cfg.CronMaxBatchSize,
	})
	disputeSyncCronHdlr := cronHandler.NewDisputeSyncHandler(merchantReporting, dbAdapter, webhookSvc, logger, cfg.CronSecret)
	achVerificationCronHdlr := cronHandler.NewACHVerificationHandler(queries, businessReporting, logger, cfg.CronSecret)
	prenoteRetryCronHdlr := cronHandler.NewPrenoteRetryHandler(queries, serverPost, logger, cfg.CronSecret)
	auditCleanupCronHdlr := cronHandler.NewAuditCleanupHandler(queries, logger, cfg.CronSecret)
	rateLimitCleanupCronHdlr := cronHandler.NewRateLimitCleanupHandler(queries, logger, cfg.CronSecret)

	// Initialize Browser Post service and handler
	browserPostSvc := browserpostService.NewBrowserPostService(
		queries,
		keyExchange,
		browserPost,
		secretManager,
		logger,
		browserPostCfg.PostURL,
		cfg.CallbackBaseURL,
	)

	templateRenderer, err := paymentHandler.NewTemplateRenderer()
	if err != nil {
		logger.Fatal("Failed to initialize template renderer", zap.Error(err))
	}

	browserPostCallbackHdlr := paymentHandler.NewBrowserPostCallbackHandler(
		browserPostSvc,
		paymentMethodSvc,
		templateRenderer,
		logger,
	)

	return &Dependencies{
		dbAdapter:                   dbAdapter,
		goroutineTracker:            goroutineTracker,
		merchantCache:               merchantCache,
		paymentMethodCache:          paymentMethodCache,
		paymentHandler:              paymentHdlr,
		subscriptionHandler:         subscriptionHdlr,
		paymentMethodHandler:        paymentMethodHdlr,
		chargebackHandler:           chargebackHdlr,
		merchantHandler:             merchantHdlr,
		billingCronHandler:          billingCronHdlr,
		disputeSyncCronHandler:      disputeSyncCronHdlr,
		achVerificationCronHandler:  achVerificationCronHdlr,
		prenoteRetryCronHandler:     prenoteRetryCronHdlr,
		auditCleanupCronHandler:     auditCleanupCronHdlr,
		rateLimitCleanupCronHandler: rateLimitCleanupCronHdlr,
		browserPostCallbackHandler:  browserPostCallbackHdlr,
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
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultMinutes int) time.Duration {
	minutes := getEnvInt(key, defaultMinutes)
	return time.Duration(minutes) * time.Minute
}

// getTracingSampleRate returns the tracing sample rate from environment
// Default is 0.1 (10%) for production, can be set to 1.0 for full sampling in dev
func getTracingSampleRate() float64 {
	value := os.Getenv("TRACING_SAMPLE_RATE")
	if value == "" {
		return 0.1 // Default 10% sampling
	}
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil || rate < 0 || rate > 1 {
		return 0.1 // Invalid value, use default
	}
	return rate
}

// requireEnv returns the environment variable value or fails if not set
func requireEnv(key string, logger *zap.Logger) string {
	value := os.Getenv(key)
	if value == "" {
		logger.Fatal("Required environment variable not set", zap.String("variable", key))
	}
	return value
}

// requireEnvInt returns the environment variable as int or fails if not set/invalid
func requireEnvInt(key string, logger *zap.Logger) int {
	value := os.Getenv(key)
	if value == "" {
		logger.Fatal("Required environment variable not set", zap.String("variable", key))
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		logger.Fatal("Environment variable must be an integer",
			zap.String("variable", key),
			zap.String("value", value),
		)
	}
	return intValue
}

// requireEnvFloat returns the environment variable as float64 or fails if not set/invalid
func requireEnvFloat(key string, logger *zap.Logger) float64 {
	value := os.Getenv(key)
	if value == "" {
		logger.Fatal("Required environment variable not set", zap.String("variable", key))
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Fatal("Environment variable must be a float",
			zap.String("variable", key),
			zap.String("value", value),
		)
	}
	return floatValue
}

// requireEnvBool returns the environment variable as bool or fails if not set/invalid
func requireEnvBool(key string, logger *zap.Logger) bool {
	value := os.Getenv(key)
	if value == "" {
		logger.Fatal("Required environment variable not set", zap.String("variable", key))
	}
	if value == "true" || value == "1" {
		return true
	}
	if value == "false" || value == "0" {
		return false
	}
	logger.Fatal("Environment variable must be 'true', 'false', '1', or '0'",
		zap.String("variable", key),
		zap.String("value", value),
	)
	return false
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
            <input type="hidden" name="ACCOUNT_NBR" id="cardNbrHidden">
            <input type="hidden" name="EXP_DATE" id="expDateHidden">
            <input type="hidden" name="CVV2" id="cvvHidden">
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
	_, _ = w.Write([]byte(html))
}
