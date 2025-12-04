package main

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/pkg/crypto"
)

// Rate limit constraints to prevent abuse
const (
	MaxRequestsPerSecond = 10000 // Maximum allowed requests per second
	MaxBurstLimit        = 20000 // Maximum allowed burst limit
)

type PayCLI struct {
	ctx           context.Context
	pool          *pgxpool.Pool
	queries       *sqlc.Queries
	secretManager ports.SecretManagerAdapter
	logger        *zap.Logger
}

func main() {
	var (
		dbURL    = flag.String("db", getDefaultDBURL(), "Database URL")
		action   = flag.String("action", "", "Action to perform")
		jsonFile = flag.String("json", "", "JSON file with service/merchant details")
		credsFile  = flag.String("credentials", "", "Service credentials JSON file (for generate-token)")
		credsShort = flag.String("c", "", "Service credentials JSON file (shorthand)")
		expires    = flag.String("expires", "1h", "Token expiry duration (e.g., 5m, 1h, 24h)")
		expiresS   = flag.String("e", "", "Token expiry duration (shorthand)")
		scopes     = flag.String("scopes", "", "Comma-separated scopes for token")
		scopesS    = flag.String("s", "", "Comma-separated scopes (shorthand)")
		outputFmt  = flag.String("output", "token", "Output format: token, json, curl")
		outputS    = flag.String("o", "", "Output format (shorthand)")
		decode     = flag.Bool("decode", false, "Show decoded claims")
	)
	flag.Parse()

	if *action == "" {
		fmt.Println("paycli - Payment Service CLI")
		fmt.Println()
		fmt.Println("Usage: paycli -action=<action> [options]")
		fmt.Println()
		fmt.Println("Management Actions:")
		fmt.Println("  create-service  - Create a new service with RSA keypair")
		fmt.Println("  update-service  - Update service settings (name, rate limits, active status)")
		fmt.Println("  create-merchant - Create a new merchant with API credentials")
		fmt.Println("  grant-access    - Grant service access to merchant")
		fmt.Println("  list-services   - List all registered services")
		fmt.Println("  list-merchants  - List all merchants")
		fmt.Println()
		fmt.Println("Token Actions:")
		fmt.Println("  generate-token  - Generate JWT token from service credentials")
		fmt.Println()
		fmt.Println("Token Generation Options (for generate-token):")
		fmt.Println("  -c, --credentials  Service credentials JSON file (required)")
		fmt.Println("  -e, --expires      Token expiry duration (default: 1h)")
		fmt.Println("  -s, --scopes       Comma-separated scopes (default: all)")
		fmt.Println("  -o, --output       Output format: token, json, curl")
		fmt.Println("      --decode       Show decoded claims")
		fmt.Println()
		fmt.Println("Note: Sandbox merchant is auto-seeded on server startup in development/staging.")
		os.Exit(1)
	}

	// Handle generate-token separately (doesn't need database)
	if *action == "generate-token" {
		creds := resolveFlag(*credsShort, *credsFile)
		exp := resolveFlag(*expiresS, *expires)
		sc := resolveFlag(*scopesS, *scopes)
		out := resolveFlag(*outputS, *outputFmt)
		generateToken(creds, exp, sc, out, *decode)
		return
	}

	// Validate database URL for other actions
	if *dbURL == "" {
		log.Fatal("Database URL not provided. Set DATABASE_URL environment variable or use -db flag.")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer pool.Close()

	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer func() { _ = logger.Sync() }()

	// Initialize secret manager (mock for CLI - real implementation for production)
	secretManager := &mockSecretManager{}

	queries := sqlc.New(pool)
	cli := &PayCLI{
		ctx:           ctx,
		pool:          pool,
		queries:       queries,
		secretManager: secretManager,
		logger:        logger,
	}

	switch *action {
	// Management actions
	case "create-service":
		cli.createService(*jsonFile)
	case "update-service":
		cli.updateService(*jsonFile)
	case "create-merchant":
		cli.createMerchant(*jsonFile)
	case "grant-access":
		cli.grantAccess()
	case "list-services":
		cli.listServices()
	case "list-merchants":
		cli.listMerchants()
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		os.Exit(1)
	}
}

// resolveFlag returns short flag if set, otherwise long flag
func resolveFlag(short, long string) string {
	if short != "" {
		return short
	}
	return long
}

// ============================================================================
// Management Actions
// ============================================================================

func (cli *PayCLI) createService(jsonFile string) {
	var serviceData struct {
		ServiceID         string `json:"service_id"`
		ServiceName       string `json:"service_name"`
		Environment       string `json:"environment"`
		RequestsPerSecond int    `json:"requests_per_second"`
		BurstLimit        int    `json:"burst_limit"`
		GenerateKeypair   bool   `json:"generate_keypair"`
		PublicKey         string `json:"public_key,omitempty"`
	}

	if jsonFile != "" {
		data, err := os.ReadFile(jsonFile)
		if err != nil {
			log.Fatal("Failed to read JSON file:", err)
		}
		if err := json.Unmarshal(data, &serviceData); err != nil {
			log.Fatal("Failed to parse JSON:", err)
		}
		// Validate rate limits from JSON
		if serviceData.RequestsPerSecond > MaxRequestsPerSecond {
			log.Fatalf("Requests per second cannot exceed %d", MaxRequestsPerSecond)
		}
		if serviceData.BurstLimit > MaxBurstLimit {
			log.Fatalf("Burst limit cannot exceed %d", MaxBurstLimit)
		}
	} else {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Service ID (e.g., wordpress-plugin): ")
		serviceData.ServiceID, _ = reader.ReadString('\n')
		serviceData.ServiceID = strings.TrimSpace(serviceData.ServiceID)

		fmt.Print("Service Name: ")
		serviceData.ServiceName, _ = reader.ReadString('\n')
		serviceData.ServiceName = strings.TrimSpace(serviceData.ServiceName)

		fmt.Print("Environment (staging/production) [staging]: ")
		serviceData.Environment, _ = reader.ReadString('\n')
		serviceData.Environment = strings.TrimSpace(serviceData.Environment)
		if serviceData.Environment == "" {
			serviceData.Environment = "staging"
		}

		fmt.Printf("Requests per second [1000] (max %d): ", MaxRequestsPerSecond)
		if _, err := fmt.Fscanf(reader, "%d\n", &serviceData.RequestsPerSecond); err != nil || serviceData.RequestsPerSecond == 0 {
			serviceData.RequestsPerSecond = 1000
		}
		if serviceData.RequestsPerSecond > MaxRequestsPerSecond {
			log.Fatalf("Requests per second cannot exceed %d", MaxRequestsPerSecond)
		}

		fmt.Printf("Burst limit [2000] (max %d): ", MaxBurstLimit)
		if _, err := fmt.Fscanf(reader, "%d\n", &serviceData.BurstLimit); err != nil || serviceData.BurstLimit == 0 {
			serviceData.BurstLimit = 2000
		}
		if serviceData.BurstLimit > MaxBurstLimit {
			log.Fatalf("Burst limit cannot exceed %d", MaxBurstLimit)
		}

		fmt.Print("Generate new RSA keypair? (y/n) [y]: ")
		response, _ := reader.ReadString('\n')
		serviceData.GenerateKeypair = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "n")
	}

	var publicKeyPEM string
	var privateKeyPEM string

	if serviceData.GenerateKeypair {
		// Generate RSA keypair using pkg/crypto
		keypair, err := crypto.GenerateRSAKeyPair()
		if err != nil {
			log.Fatal("Failed to generate RSA keypair:", err)
		}
		publicKeyPEM = keypair.PublicKeyPEM
		privateKeyPEM = keypair.PrivateKeyPEM
	} else if serviceData.PublicKey != "" {
		publicKeyPEM = serviceData.PublicKey
	} else {
		log.Fatal("Either generate_keypair must be true or public_key must be provided")
	}

	// Create service in database using sqlc
	serviceUUID := uuid.New()

	_, err := cli.queries.CreateService(cli.ctx, sqlc.CreateServiceParams{
		ID:                   serviceUUID,
		ServiceID:            serviceData.ServiceID,
		ServiceName:          serviceData.ServiceName,
		PublicKey:            publicKeyPEM,
		PublicKeyFingerprint: generateFingerprint([]byte(publicKeyPEM)),
		Environment:          serviceData.Environment,
		RequestsPerSecond:    pgtype.Int4{Int32: int32(serviceData.RequestsPerSecond), Valid: true},
		BurstLimit:           pgtype.Int4{Int32: int32(serviceData.BurstLimit), Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		log.Fatal("Failed to create service:", err)
	}

	// Save credentials
	outputFile := fmt.Sprintf("service_%s_credentials.json", serviceData.ServiceID)
	output := map[string]interface{}{
		"service_id":   serviceData.ServiceID,
		"service_name": serviceData.ServiceName,
		"environment":  serviceData.Environment,
		"public_key":   publicKeyPEM,
	}

	if privateKeyPEM != "" {
		output["private_key"] = privateKeyPEM
		output["note"] = "Keep the private key secure! Use it to sign JWT tokens."
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	if err := os.WriteFile(outputFile, data, 0600); err != nil {
		log.Printf("Warning: Failed to save credentials file: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Println("✅ SERVICE CREATED SUCCESSFULLY")
	fmt.Println("========================================")
	fmt.Printf("Service ID: %s\n", serviceData.ServiceID)
	fmt.Printf("Service Name: %s\n", serviceData.ServiceName)
	fmt.Printf("Environment: %s\n", serviceData.Environment)
	fmt.Printf("Rate Limit: %d req/s (burst: %d)\n", serviceData.RequestsPerSecond, serviceData.BurstLimit)
	if privateKeyPEM != "" {
		fmt.Printf("\n📁 Credentials saved to: %s\n", outputFile)
		fmt.Println("⚠️  Keep the private key secure!")
	}
	fmt.Println("========================================")
}

func (cli *PayCLI) updateService(jsonFile string) {
	var updateData struct {
		ServiceID         string `json:"service_id"`
		ServiceName       string `json:"service_name,omitempty"`
		RequestsPerSecond *int   `json:"requests_per_second,omitempty"`
		BurstLimit        *int   `json:"burst_limit,omitempty"`
		IsActive          *bool  `json:"is_active,omitempty"`
		RotateKey         bool   `json:"rotate_key,omitempty"`
	}

	if jsonFile != "" {
		data, err := os.ReadFile(jsonFile)
		if err != nil {
			log.Fatal("Failed to read JSON file:", err)
		}
		if err := json.Unmarshal(data, &updateData); err != nil {
			log.Fatal("Failed to parse JSON:", err)
		}
		// Validate rate limits from JSON
		if updateData.RequestsPerSecond != nil && *updateData.RequestsPerSecond > MaxRequestsPerSecond {
			log.Fatalf("Requests per second cannot exceed %d", MaxRequestsPerSecond)
		}
		if updateData.BurstLimit != nil && *updateData.BurstLimit > MaxBurstLimit {
			log.Fatalf("Burst limit cannot exceed %d", MaxBurstLimit)
		}
	} else {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Service ID to update: ")
		updateData.ServiceID, _ = reader.ReadString('\n')
		updateData.ServiceID = strings.TrimSpace(updateData.ServiceID)
	}

	if updateData.ServiceID == "" {
		log.Fatal("Service ID is required")
	}

	// Get existing service
	service, err := cli.queries.GetServiceByServiceID(cli.ctx, updateData.ServiceID)
	if err != nil {
		log.Fatal("Service not found:", updateData.ServiceID)
	}

	// Interactive mode - prompt for updates
	if jsonFile == "" {
		reader := bufio.NewReader(os.Stdin)

		fmt.Printf("\nCurrent settings for '%s':\n", service.ServiceID)
		fmt.Printf("  Name: %s\n", service.ServiceName)
		fmt.Printf("  Rate Limit: %d req/s (burst: %d)\n", service.RequestsPerSecond.Int32, service.BurstLimit.Int32)
		fmt.Printf("  Active: %v\n", service.IsActive.Bool)
		fmt.Println()

		fmt.Printf("New service name [%s]: ", service.ServiceName)
		newName, _ := reader.ReadString('\n')
		newName = strings.TrimSpace(newName)
		if newName != "" {
			updateData.ServiceName = newName
		}

		fmt.Printf("New requests per second [%d] (max %d): ", service.RequestsPerSecond.Int32, MaxRequestsPerSecond)
		var newRPS int
		if _, err := fmt.Fscanf(reader, "%d\n", &newRPS); err == nil && newRPS > 0 {
			if newRPS > MaxRequestsPerSecond {
				log.Fatalf("Requests per second cannot exceed %d", MaxRequestsPerSecond)
			}
			updateData.RequestsPerSecond = &newRPS
		} else {
			// Clear the buffer if scan failed
			_, _ = reader.ReadString('\n')
		}

		fmt.Printf("New burst limit [%d] (max %d): ", service.BurstLimit.Int32, MaxBurstLimit)
		var newBurst int
		if _, err := fmt.Fscanf(reader, "%d\n", &newBurst); err == nil && newBurst > 0 {
			if newBurst > MaxBurstLimit {
				log.Fatalf("Burst limit cannot exceed %d", MaxBurstLimit)
			}
			updateData.BurstLimit = &newBurst
		} else {
			_, _ = reader.ReadString('\n')
		}

		fmt.Printf("Active status (true/false) [%v]: ", service.IsActive.Bool)
		activeStr, _ := reader.ReadString('\n')
		activeStr = strings.TrimSpace(strings.ToLower(activeStr))
		if activeStr == "true" {
			active := true
			updateData.IsActive = &active
		} else if activeStr == "false" {
			active := false
			updateData.IsActive = &active
		}

		fmt.Print("Rotate RSA keypair? (y/n) [n]: ")
		rotateStr, _ := reader.ReadString('\n')
		updateData.RotateKey = strings.HasPrefix(strings.ToLower(strings.TrimSpace(rotateStr)), "y")
	}

	// Handle key rotation
	var newPrivateKeyPEM string
	newPublicKeyPEM := service.PublicKey
	newFingerprint := service.PublicKeyFingerprint

	if updateData.RotateKey {
		keypair, err := crypto.GenerateRSAKeyPair()
		if err != nil {
			log.Fatal("Failed to generate new RSA keypair:", err)
		}
		newPublicKeyPEM = keypair.PublicKeyPEM
		newPrivateKeyPEM = keypair.PrivateKeyPEM
		newFingerprint = generateFingerprint([]byte(newPublicKeyPEM))
	}

	// Prepare update values
	serviceName := service.ServiceName
	if updateData.ServiceName != "" {
		serviceName = updateData.ServiceName
	}

	rps := service.RequestsPerSecond.Int32
	if updateData.RequestsPerSecond != nil {
		rps = int32(*updateData.RequestsPerSecond)
	}

	burst := service.BurstLimit.Int32
	if updateData.BurstLimit != nil {
		burst = int32(*updateData.BurstLimit)
	}

	isActive := service.IsActive.Bool
	if updateData.IsActive != nil {
		isActive = *updateData.IsActive
	}

	// Update service in database
	_, err = cli.queries.UpdateService(cli.ctx, sqlc.UpdateServiceParams{
		ID:                   service.ID,
		ServiceName:          serviceName,
		PublicKey:            newPublicKeyPEM,
		PublicKeyFingerprint: newFingerprint,
		RequestsPerSecond:    pgtype.Int4{Int32: rps, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: burst, Valid: true},
		IsActive:             pgtype.Bool{Bool: isActive, Valid: true},
	})
	if err != nil {
		log.Fatal("Failed to update service:", err)
	}

	// Save new credentials if key was rotated
	if updateData.RotateKey && newPrivateKeyPEM != "" {
		outputFile := fmt.Sprintf("service_%s_credentials.json", updateData.ServiceID)
		output := map[string]interface{}{
			"service_id":   updateData.ServiceID,
			"service_name": serviceName,
			"environment":  service.Environment,
			"public_key":   newPublicKeyPEM,
			"private_key":  newPrivateKeyPEM,
			"note":         "Keep the private key secure! Use it to sign JWT tokens.",
			"rotated_at":   time.Now().Format(time.RFC3339),
		}

		data, _ := json.MarshalIndent(output, "", "  ")
		if err := os.WriteFile(outputFile, data, 0600); err != nil {
			log.Printf("Warning: Failed to save credentials file: %v", err)
		} else {
			fmt.Printf("\n📁 New credentials saved to: %s\n", outputFile)
			fmt.Println("⚠️  Update your application with the new private key!")
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("✅ SERVICE UPDATED SUCCESSFULLY")
	fmt.Println("========================================")
	fmt.Printf("Service ID: %s\n", updateData.ServiceID)
	fmt.Printf("Service Name: %s\n", serviceName)
	fmt.Printf("Rate Limit: %d req/s (burst: %d)\n", rps, burst)
	fmt.Printf("Active: %v\n", isActive)
	if updateData.RotateKey {
		fmt.Println("🔑 RSA keypair rotated")
	}
	fmt.Println("========================================")
}

func (cli *PayCLI) createMerchant(jsonFile string) {
	var merchantData struct {
		Slug              string `json:"slug"`
		Name              string `json:"name"`
		CustNbr           string `json:"cust_nbr"`
		MerchNbr          string `json:"merch_nbr"`
		DbaNbr            string `json:"dba_nbr"`
		TerminalNbr       string `json:"terminal_nbr"`
		MacSecretPath     string `json:"mac_secret_path"`
		Environment       string `json:"environment"`
		Tier              string `json:"tier"`
		RequestsPerSecond int    `json:"requests_per_second"`
	}

	if jsonFile != "" {
		data, err := os.ReadFile(jsonFile)
		if err != nil {
			log.Fatal("Failed to read JSON file:", err)
		}
		if err := json.Unmarshal(data, &merchantData); err != nil {
			log.Fatal("Failed to parse JSON:", err)
		}
	} else {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Merchant slug (unique identifier): ")
		merchantData.Slug, _ = reader.ReadString('\n')
		merchantData.Slug = strings.TrimSpace(merchantData.Slug)

		fmt.Print("Merchant name: ")
		merchantData.Name, _ = reader.ReadString('\n')
		merchantData.Name = strings.TrimSpace(merchantData.Name)

		fmt.Print("Customer number (EPX): ")
		merchantData.CustNbr, _ = reader.ReadString('\n')
		merchantData.CustNbr = strings.TrimSpace(merchantData.CustNbr)

		fmt.Print("Merchant number (EPX): ")
		merchantData.MerchNbr, _ = reader.ReadString('\n')
		merchantData.MerchNbr = strings.TrimSpace(merchantData.MerchNbr)

		fmt.Print("DBA number (EPX): ")
		merchantData.DbaNbr, _ = reader.ReadString('\n')
		merchantData.DbaNbr = strings.TrimSpace(merchantData.DbaNbr)

		fmt.Print("Terminal number (EPX): ")
		merchantData.TerminalNbr, _ = reader.ReadString('\n')
		merchantData.TerminalNbr = strings.TrimSpace(merchantData.TerminalNbr)

		fmt.Print("MAC secret path [/secrets/merchant]: ")
		merchantData.MacSecretPath, _ = reader.ReadString('\n')
		merchantData.MacSecretPath = strings.TrimSpace(merchantData.MacSecretPath)
		if merchantData.MacSecretPath == "" {
			merchantData.MacSecretPath = "/secrets/merchant"
		}

		fmt.Print("Environment (staging/production) [staging]: ")
		merchantData.Environment, _ = reader.ReadString('\n')
		merchantData.Environment = strings.TrimSpace(merchantData.Environment)
		if merchantData.Environment == "" {
			merchantData.Environment = "staging"
		}

		fmt.Print("Tier (standard/premium/enterprise) [standard]: ")
		merchantData.Tier, _ = reader.ReadString('\n')
		merchantData.Tier = strings.TrimSpace(merchantData.Tier)
		if merchantData.Tier == "" {
			merchantData.Tier = "standard"
		}

		fmt.Print("Requests per second [100]: ")
		if _, err := fmt.Fscanf(reader, "%d\n", &merchantData.RequestsPerSecond); err != nil || merchantData.RequestsPerSecond == 0 {
			merchantData.RequestsPerSecond = 100
		}
	}

	// Create merchant using sqlc
	merchantUUID := uuid.New()
	merchant, err := cli.queries.CreateMerchant(cli.ctx, sqlc.CreateMerchantParams{
		ID:            merchantUUID,
		Slug:          merchantData.Slug,
		Name:          merchantData.Name,
		CustNbr:       merchantData.CustNbr,
		MerchNbr:      merchantData.MerchNbr,
		DbaNbr:        merchantData.DbaNbr,
		TerminalNbr:   merchantData.TerminalNbr,
		MacSecretPath: merchantData.MacSecretPath,
		Environment:   merchantData.Environment,
		IsActive:      true,
	})

	if err != nil {
		log.Fatal("Failed to create merchant:", err)
	}

	merchantID := merchant.ID.String()

	// Note: Merchants don't get API keys directly.
	// Create a Service to authenticate and link it to this merchant via grant-access command.
	// This follows the service-based authentication architecture.

	// Save merchant info
	outputFile := fmt.Sprintf("merchant_%s_info.json", merchantData.Slug)
	output := map[string]interface{}{
		"merchant_id": merchantID,
		"slug":        merchantData.Slug,
		"name":        merchantData.Name,
		"environment": merchantData.Environment,
		"tier":        merchantData.Tier,
		"rate_limit":  merchantData.RequestsPerSecond,
		"epx_config": map[string]string{
			"cust_nbr":     merchantData.CustNbr,
			"merch_nbr":    merchantData.MerchNbr,
			"dba_nbr":      merchantData.DbaNbr,
			"terminal_nbr": merchantData.TerminalNbr,
		},
	}

	// Note about authentication
	output["authentication_note"] = "To authenticate API requests for this merchant, create a Service (./admin -action=create-service) and grant it access (./admin -action=grant-access)"

	data, _ := json.MarshalIndent(output, "", "  ")
	if err := os.WriteFile(outputFile, data, 0600); err != nil {
		log.Printf("Warning: Failed to save merchant info file: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Println("✅ MERCHANT CREATED SUCCESSFULLY")
	fmt.Println("========================================")
	fmt.Printf("Merchant ID: %s\n", merchantID)
	fmt.Printf("Slug: %s\n", merchantData.Slug)
	fmt.Printf("Name: %s\n", merchantData.Name)
	fmt.Printf("Environment: %s\n", merchantData.Environment)
	fmt.Printf("Tier: %s\n", merchantData.Tier)
	fmt.Printf("Rate Limit: %d req/s\n", merchantData.RequestsPerSecond)
	fmt.Printf("\n📝 Next Steps:\n")
	fmt.Printf("  1. Create a Service: ./admin -action=create-service\n")
	fmt.Printf("  2. Grant access: ./admin -action=grant-access\n")
	fmt.Printf("  3. Service uses RSA private key to sign JWT tokens\n")
	fmt.Printf("\n📁 Info saved to: %s\n", outputFile)
	fmt.Println("========================================")
}

func (cli *PayCLI) grantAccess() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Service ID (e.g., wordpress-plugin): ")
	serviceID, _ := reader.ReadString('\n')
	serviceID = strings.TrimSpace(serviceID)

	fmt.Print("Merchant slug: ")
	merchantSlug, _ := reader.ReadString('\n')
	merchantSlug = strings.TrimSpace(merchantSlug)

	// Get service and merchant IDs using sqlc
	service, err := cli.queries.GetServiceByServiceID(cli.ctx, serviceID)
	if err != nil {
		log.Fatal("Service not found:", serviceID)
	}

	merchant, err := cli.queries.GetMerchantBySlug(cli.ctx, merchantSlug)
	if err != nil {
		log.Fatal("Merchant not found:", merchantSlug)
	}

	// Define scopes
	scopes := []string{
		"payment:create",
		"payment:read",
		"payment:update",
		"payment:refund",
		"subscription:manage",
		"payment_method:manage",
	}

	fmt.Printf("\nGranting scopes: %v\n", scopes)

	// Grant access using sqlc
	_, err = cli.queries.GrantServiceAccess(cli.ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  service.ID,
		MerchantID: merchant.ID,
		Scopes:     scopes,
		ExpiresAt:  pgtype.Timestamptz{}, // No expiration
	})

	if err != nil {
		log.Fatal("Failed to grant access:", err)
	}

	fmt.Println("\n✅ Access granted successfully!")
	fmt.Printf("Service '%s' now has access to merchant '%s'\n", serviceID, merchantSlug)
}

func (cli *PayCLI) listServices() {
	// List services using sqlc
	services, err := cli.queries.ListServices(cli.ctx, sqlc.ListServicesParams{
		Environment: pgtype.Text{}, // NULL to get all
		IsActive:    pgtype.Bool{}, // NULL to get all
		LimitVal:    100,
		OffsetVal:   0,
	})
	if err != nil {
		log.Fatal("Failed to query services:", err)
	}

	fmt.Println("\n=== REGISTERED SERVICES ===")
	fmt.Printf("%-30s %-40s %-15s %-10s %-20s\n", "Service ID", "Name", "Environment", "Active", "Created")
	fmt.Println(strings.Repeat("-", 120))

	for _, service := range services {
		fmt.Printf("%-30s %-40s %-15s %-10v %-20s\n",
			service.ServiceID, service.ServiceName, service.Environment, service.IsActive,
			service.CreatedAt.Time.Format("2006-01-02 15:04"))
	}
}

func (cli *PayCLI) listMerchants() {
	// List merchants using sqlc
	merchants, err := cli.queries.ListMerchants(cli.ctx, sqlc.ListMerchantsParams{
		Environment: pgtype.Text{}, // NULL to get all
		IsActive:    pgtype.Bool{}, // NULL to get all
		LimitVal:    100,
		OffsetVal:   0,
	})
	if err != nil {
		log.Fatal("Failed to query merchants:", err)
	}

	fmt.Println("\n=== REGISTERED MERCHANTS ===")
	fmt.Printf("%-30s %-40s %-15s %-15s %-20s\n", "Slug", "Name", "Environment", "Active", "Created")
	fmt.Println(strings.Repeat("-", 125))

	for _, merchant := range merchants {
		fmt.Printf("%-30s %-40s %-15s %-15v %-20s\n",
			merchant.Slug, merchant.Name, merchant.Environment, merchant.IsActive,
			merchant.CreatedAt.Format("2006-01-02 15:04"))
	}
}

func generateFingerprint(publicKeyPEM []byte) string {
	// Simple fingerprint generation
	h := sha256.New()
	h.Write(publicKeyPEM)
	return fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]
}

// getDefaultDBURL returns database URL from environment variables.
// First checks DATABASE_URL, then constructs from individual DB_* variables.
func getDefaultDBURL() string {
	// Check for DATABASE_URL first
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// Construct from individual DB_* environment variables (used in Docker)
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	sslMode := os.Getenv("DB_SSL_MODE")

	// Only construct if we have the minimum required variables
	if host != "" && name != "" {
		if port == "" {
			port = "5432"
		}
		if user == "" {
			user = "postgres"
		}
		if sslMode == "" {
			sslMode = "disable"
		}

		if password != "" {
			return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, name, sslMode)
		}
		return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", user, host, port, name, sslMode)
	}

	return ""
}

// ============================================================================
// Token Generation (integrated from jwtgen)
// ============================================================================

// ServiceCredentials represents the structure of the credentials JSON file
type ServiceCredentials struct {
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	Environment string `json:"environment"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
}

// TokenOutput represents the JSON output format
type TokenOutput struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	ServiceID string    `json:"service_id"`
	Scopes    []string  `json:"scopes"`
}

// generateToken generates a JWT token from service credentials
func generateToken(credsFile, expires, scopeStr, outputFmt string, decode bool) {
	// Validate required flags
	if credsFile == "" {
		fmt.Fprintln(os.Stderr, "Error: credentials file is required (-c or --credentials)")
		fmt.Fprintln(os.Stderr, "Usage: admin -action=generate-token -c <credentials.json>")
		os.Exit(1)
	}

	// Parse expiry duration
	expiryDuration, err := time.ParseDuration(expires)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid expiry duration '%s': %v\n", expires, err)
		os.Exit(1)
	}

	// Load credentials
	creds, err := loadServiceCredentials(credsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading credentials: %v\n", err)
		os.Exit(1)
	}

	// Parse scopes
	var tokenScopes []string
	if scopeStr != "" {
		tokenScopes = strings.Split(scopeStr, ",")
		for i := range tokenScopes {
			tokenScopes[i] = strings.TrimSpace(tokenScopes[i])
		}
	} else {
		tokenScopes = domain.AllPaymentScopes()
	}

	// Generate token
	token, err := createJWTToken(creds, tokenScopes, expiryDuration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
		os.Exit(1)
	}

	// Output based on format
	expiresAt := time.Now().Add(expiryDuration)

	switch outputFmt {
	case "json":
		outputTokenJSON(token, expiresAt, creds.ServiceID, tokenScopes)
	case "curl":
		outputTokenCurl(token)
	default:
		fmt.Println(token)
	}

	// Show decoded claims if requested
	if decode {
		fmt.Fprintln(os.Stderr, "\n# Decoded Claims:")
		showDecodedClaims(token)
	}
}

func loadServiceCredentials(path string) (*ServiceCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var creds ServiceCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if creds.ServiceID == "" {
		return nil, fmt.Errorf("missing service_id in credentials file")
	}

	if creds.PrivateKey == "" {
		return nil, fmt.Errorf("missing private_key in credentials file")
	}

	return &creds, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS1 format first
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return privateKey, nil
	}

	// Try PKCS8 format
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA private key")
	}

	return rsaKey, nil
}

// createJWTToken creates a JWT token for service authentication
func createJWTToken(creds *ServiceCredentials, scopes []string, expiry time.Duration) (string, error) {
	privateKey, err := parseRSAPrivateKey(creds.PrivateKey)
	if err != nil {
		return "", err
	}

	now := time.Now()
	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"iss":    creds.ServiceID,
		"sub":    creds.ServiceID,
		"scopes": scopes,
		"exp":    now.Add(expiry).Unix(),
		"iat":    now.Unix(),
		"nbf":    now.Unix(),
		"jti":    jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func outputTokenJSON(token string, expiresAt time.Time, serviceID string, scopes []string) {
	output := TokenOutput{
		Token:     token,
		ExpiresAt: expiresAt,
		ServiceID: serviceID,
		Scopes:    scopes,
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

func outputTokenCurl(token string) {
	fmt.Printf(`# Example curl command:
# NOTE: merchant_id must be included in request body
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer %s" \
  -d '{
    "merchant_id": "your-merchant-uuid",
    "customer_id": "cust_123",
    "amount_cents": 1000,
    "currency": "USD",
    "payment_method_id": "your-payment-method-uuid",
    "idempotency_key": "sale_unique_key_123"
  }'
`, token)
}

func showDecodedClaims(tokenStr string) {
	// Parse without validation to show claims
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing token: %v\n", err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: invalid claims format")
		return
	}

	data, _ := json.MarshalIndent(claims, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}

// ============================================================================
// Mock Secret Manager (for CLI usage)
// ============================================================================

// mockSecretManager is a simple in-memory secret manager for CLI usage.
// In production, use GCP Secret Manager or AWS Secrets Manager.
type mockSecretManager struct {
	secrets map[string]string
}

func (m *mockSecretManager) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	if m.secrets == nil {
		m.secrets = make(map[string]string)
	}
	value, ok := m.secrets[path]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", path)
	}
	return &ports.Secret{Value: value, Version: "1"}, nil
}

func (m *mockSecretManager) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	return m.GetSecret(ctx, path)
}

func (m *mockSecretManager) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	if m.secrets == nil {
		m.secrets = make(map[string]string)
	}
	m.secrets[path] = value
	fmt.Printf("  [mock] Secret stored at: %s\n", path)
	return "1", nil
}

func (m *mockSecretManager) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	_, err := m.PutSecret(ctx, path, newValue, nil)
	if err != nil {
		return nil, err
	}
	return &ports.SecretRotationInfo{CurrentVersion: "2", PreviousVersion: "1"}, nil
}

func (m *mockSecretManager) DeleteSecret(ctx context.Context, path string) error {
	if m.secrets == nil {
		return nil
	}
	delete(m.secrets, path)
	return nil
}
