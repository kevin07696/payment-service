package main

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kevin07696/payment-service/internal/auth"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

type AdminCLI struct {
	db      *sql.DB
	adminID string
}

func main() {
	var (
		dbURL    = flag.String("db", "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable", "Database URL")
		action   = flag.String("action", "", "Action to perform: login, create-service, create-merchant, grant-access")
		email    = flag.String("email", "", "Admin email for login")
		jsonFile = flag.String("json", "", "JSON file with service/merchant details")
	)
	flag.Parse()

	if *action == "" {
		fmt.Println("Usage: admin -action=<action> [options]")
		fmt.Println("Actions:")
		fmt.Println("  login          - Login as admin")
		fmt.Println("  create-service - Create a new service with RSA keypair")
		fmt.Println("  create-merchant - Create a new merchant with API credentials")
		fmt.Println("  grant-access   - Grant service access to merchant")
		fmt.Println("  list-services  - List all registered services")
		fmt.Println("  list-merchants - List all merchants")
		os.Exit(1)
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	cli := &AdminCLI{db: sqlDB}

	switch *action {
	case "login":
		cli.login(*email)
	case "create-service":
		cli.createService(*jsonFile)
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

func (cli *AdminCLI) login(email string) {
	if email == "" {
		fmt.Print("Admin email: ")
		reader := bufio.NewReader(os.Stdin)
		email, _ = reader.ReadString('\n')
		email = strings.TrimSpace(email)
	}

	fmt.Print("Password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal("Failed to read password:", err)
	}
	fmt.Println()

	// Verify admin credentials
	var passwordHash string
	err = cli.db.QueryRow(`
		SELECT id, password_hash FROM admins
		WHERE email = $1 AND is_active = true
	`, email).Scan(&cli.adminID, &passwordHash)

	if err != nil {
		log.Fatal("Admin not found or inactive")
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), password)
	if err != nil {
		log.Fatal("Invalid password")
	}

	fmt.Printf("✅ Logged in as admin: %s (ID: %s)\n", email, cli.adminID)
}

func (cli *AdminCLI) createService(jsonFile string) {
	if cli.adminID == "" {
		// Auto-login with first admin if not logged in
		cli.autoLogin()
	}

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

		fmt.Print("Requests per second [1000]: ")
		fmt.Fscanf(reader, "%d\n", &serviceData.RequestsPerSecond)
		if serviceData.RequestsPerSecond == 0 {
			serviceData.RequestsPerSecond = 1000
		}

		fmt.Print("Burst limit [2000]: ")
		fmt.Fscanf(reader, "%d\n", &serviceData.BurstLimit)
		if serviceData.BurstLimit == 0 {
			serviceData.BurstLimit = 2000
		}

		fmt.Print("Generate new RSA keypair? (y/n) [y]: ")
		response, _ := reader.ReadString('\n')
		serviceData.GenerateKeypair = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "n")
	}

	var publicKeyPEM []byte
	var privateKey *rsa.PrivateKey

	if serviceData.GenerateKeypair {
		// Generate RSA keypair
		var err error
		privateKey, _, err = auth.GenerateRSAKeyPair(2048)
		if err != nil {
			log.Fatal("Failed to generate RSA keypair:", err)
		}

		publicKeyPEM, err = auth.PublicKeyToPEM(&privateKey.PublicKey)
		if err != nil {
			log.Fatal("Failed to convert public key to PEM:", err)
		}
	} else if serviceData.PublicKey != "" {
		publicKeyPEM = []byte(serviceData.PublicKey)
	} else {
		log.Fatal("Either generate_keypair must be true or public_key must be provided")
	}

	// Create service in database
	serviceID := uuid.New().String()
	_, err := cli.db.Exec(`
		INSERT INTO registered_services (
			id, service_id, service_name, public_key,
			public_key_fingerprint, environment,
			requests_per_second, burst_limit, is_active, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, serviceID, serviceData.ServiceID, serviceData.ServiceName,
		string(publicKeyPEM), generateFingerprint(publicKeyPEM),
		serviceData.Environment, serviceData.RequestsPerSecond,
		serviceData.BurstLimit, true, cli.adminID)

	if err != nil {
		log.Fatal("Failed to create service:", err)
	}

	// Save credentials
	outputFile := fmt.Sprintf("service_%s_credentials.json", serviceData.ServiceID)
	output := map[string]interface{}{
		"service_id":   serviceData.ServiceID,
		"service_name": serviceData.ServiceName,
		"environment":  serviceData.Environment,
		"public_key":   string(publicKeyPEM),
	}

	if privateKey != nil {
		output["private_key"] = string(auth.PrivateKeyToPEM(privateKey))
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
	if privateKey != nil {
		fmt.Printf("\n📁 Credentials saved to: %s\n", outputFile)
		fmt.Println("⚠️  Keep the private key secure!")
	}
	fmt.Println("========================================")
}

func (cli *AdminCLI) createMerchant(jsonFile string) {
	if cli.adminID == "" {
		cli.autoLogin()
	}

	var merchantData struct {
		Slug                string `json:"slug"`
		Name                string `json:"name"`
		CustNbr             string `json:"cust_nbr"`
		MerchNbr            string `json:"merch_nbr"`
		DbaNbr              string `json:"dba_nbr"`
		TerminalNbr         string `json:"terminal_nbr"`
		MacSecretPath       string `json:"mac_secret_path"`
		Environment         string `json:"environment"`
		Tier                string `json:"tier"`
		RequestsPerSecond   int    `json:"requests_per_second"`
		GenerateCredentials bool   `json:"generate_credentials"`
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
		fmt.Fscanf(reader, "%d\n", &merchantData.RequestsPerSecond)
		if merchantData.RequestsPerSecond == 0 {
			merchantData.RequestsPerSecond = 100
		}

		fmt.Print("Generate API credentials? (y/n) [y]: ")
		response, _ := reader.ReadString('\n')
		merchantData.GenerateCredentials = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "n")
	}

	// Create merchant
	merchantID := uuid.New().String()
	err := cli.db.QueryRow(`
		INSERT INTO merchants (
			id, slug, name, cust_nbr, merch_nbr, dba_nbr,
			terminal_nbr, mac_secret_path, environment,
			is_active, status, tier, requests_per_second,
			burst_limit, created_by, approved_by, approved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
		RETURNING id
	`, merchantID, merchantData.Slug, merchantData.Name,
		merchantData.CustNbr, merchantData.MerchNbr, merchantData.DbaNbr,
		merchantData.TerminalNbr, merchantData.MacSecretPath, merchantData.Environment,
		true, "active", merchantData.Tier, merchantData.RequestsPerSecond,
		merchantData.RequestsPerSecond*2, cli.adminID, cli.adminID).Scan(&merchantID)

	if err != nil {
		log.Fatal("Failed to create merchant:", err)
	}

	var apiKey, apiSecret string

	// Generate API credentials if requested
	if merchantData.GenerateCredentials {
		apiKeyGen := auth.NewAPIKeyGenerator(cli.db, "payment_service_")
		creds, err := apiKeyGen.GenerateCredentials(
			merchantID,
			merchantData.Environment,
			fmt.Sprintf("API Key for %s", merchantData.Name),
			0, // No expiry
		)
		if err != nil {
			log.Printf("Warning: Failed to generate API credentials: %v", err)
		} else {
			apiKey = creds.APIKey
			apiSecret = creds.APISecret
		}
	}

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

	if apiKey != "" {
		output["api_credentials"] = map[string]string{
			"api_key":    apiKey,
			"api_secret": apiSecret,
			"note":       "Keep these credentials secure!",
		}
	}

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
	if apiKey != "" {
		fmt.Printf("\n🔑 API Credentials:\n")
		fmt.Printf("  API Key: %s\n", apiKey)
		fmt.Printf("  API Secret: %s\n", apiSecret)
		fmt.Println("  ⚠️  Save these credentials - they cannot be recovered!")
	}
	fmt.Printf("\n📁 Info saved to: %s\n", outputFile)
	fmt.Println("========================================")
}

func (cli *AdminCLI) grantAccess() {
	if cli.adminID == "" {
		cli.autoLogin()
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Service ID (e.g., wordpress-plugin): ")
	serviceID, _ := reader.ReadString('\n')
	serviceID = strings.TrimSpace(serviceID)

	fmt.Print("Merchant slug: ")
	merchantSlug, _ := reader.ReadString('\n')
	merchantSlug = strings.TrimSpace(merchantSlug)

	// Get service and merchant IDs
	var registeredServiceID string
	err := cli.db.QueryRow(`
		SELECT id FROM registered_services WHERE service_id = $1
	`, serviceID).Scan(&registeredServiceID)
	if err != nil {
		log.Fatal("Service not found:", serviceID)
	}

	var merchantID string
	err = cli.db.QueryRow(`
		SELECT id FROM merchants WHERE slug = $1
	`, merchantSlug).Scan(&merchantID)
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

	// Grant access
	_, err = cli.db.Exec(`
		INSERT INTO service_merchants (
			service_id, merchant_id, scopes, granted_by
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (service_id, merchant_id) DO UPDATE SET
			scopes = EXCLUDED.scopes,
			granted_at = NOW()
	`, registeredServiceID, merchantID,
		"{"+strings.Join(scopes, ",")+"}",
		cli.adminID)

	if err != nil {
		log.Fatal("Failed to grant access:", err)
	}

	fmt.Println("\n✅ Access granted successfully!")
	fmt.Printf("Service '%s' now has access to merchant '%s'\n", serviceID, merchantSlug)
}

func (cli *AdminCLI) listServices() {
	rows, err := cli.db.Query(`
		SELECT service_id, service_name, environment, is_active, created_at
		FROM registered_services
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Fatal("Failed to query services:", err)
	}
	defer rows.Close()

	fmt.Println("\n=== REGISTERED SERVICES ===")
	fmt.Printf("%-30s %-40s %-15s %-10s %-20s\n", "Service ID", "Name", "Environment", "Active", "Created")
	fmt.Println(strings.Repeat("-", 120))

	for rows.Next() {
		var serviceID, name, env string
		var isActive bool
		var createdAt sql.NullTime

		rows.Scan(&serviceID, &name, &env, &isActive, &createdAt)

		fmt.Printf("%-30s %-40s %-15s %-10v %-20s\n",
			serviceID, name, env, isActive,
			createdAt.Time.Format("2006-01-02 15:04"))
	}
}

func (cli *AdminCLI) listMerchants() {
	rows, err := cli.db.Query(`
		SELECT slug, name, environment, status, tier, created_at
		FROM merchants
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Fatal("Failed to query merchants:", err)
	}
	defer rows.Close()

	fmt.Println("\n=== REGISTERED MERCHANTS ===")
	fmt.Printf("%-30s %-40s %-15s %-15s %-15s %-20s\n", "Slug", "Name", "Environment", "Status", "Tier", "Created")
	fmt.Println(strings.Repeat("-", 140))

	for rows.Next() {
		var slug, name, env, status, tier string
		var createdAt sql.NullTime

		rows.Scan(&slug, &name, &env, &status, &tier, &createdAt)

		fmt.Printf("%-30s %-40s %-15s %-15s %-15s %-20s\n",
			slug, name, env, status, tier,
			createdAt.Time.Format("2006-01-02 15:04"))
	}
}

func (cli *AdminCLI) autoLogin() {
	// Try to auto-login with first admin account
	err := cli.db.QueryRow(`
		SELECT id FROM admins WHERE is_active = true LIMIT 1
	`).Scan(&cli.adminID)

	if err != nil {
		log.Fatal("No admin account found. Please login first with -action=login")
	}
}

func generateFingerprint(publicKeyPEM []byte) string {
	// Simple fingerprint generation
	h := sha256.New()
	h.Write(publicKeyPEM)
	return fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]
}
