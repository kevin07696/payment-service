package main

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

type AdminCLI struct {
	ctx     context.Context
	queries sqlc.Querier
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

	queries := sqlc.New(pool)
	cli := &AdminCLI{
		ctx:     ctx,
		queries: queries,
	}

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

	// Verify admin credentials using sqlc
	admin, err := cli.queries.GetAdminByEmail(cli.ctx, email)
	if err != nil {
		cli.createAuditLog("admin.login.failed", "", "", false, "Admin not found: "+email)
		log.Fatal("Admin not found or inactive")
	}

	if !admin.IsActive.Bool {
		cli.createAuditLog("admin.login.failed", "", admin.ID.String(), false, "Admin account is not active")
		log.Fatal("Admin account is not active")
	}

	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), password)
	if err != nil {
		cli.createAuditLog("admin.login.failed", "", admin.ID.String(), false, "Invalid password")
		log.Fatal("Invalid password")
	}

	cli.adminID = admin.ID.String()
	cli.createAuditLog("admin.login", "", admin.ID.String(), true, "")

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

	// Create service in database using sqlc
	serviceUUID := uuid.New()
	createdByUUID := pgtype.UUID{}
	if cli.adminID != "" {
		parsedUUID, err := uuid.Parse(cli.adminID)
		if err == nil {
			createdByUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
		}
	}

	service, err := cli.queries.CreateService(cli.ctx, sqlc.CreateServiceParams{
		ID:                   serviceUUID,
		ServiceID:            serviceData.ServiceID,
		ServiceName:          serviceData.ServiceName,
		PublicKey:            string(publicKeyPEM),
		PublicKeyFingerprint: generateFingerprint(publicKeyPEM),
		Environment:          serviceData.Environment,
		RequestsPerSecond:    pgtype.Int4{Int32: int32(serviceData.RequestsPerSecond), Valid: true},
		BurstLimit:           pgtype.Int4{Int32: int32(serviceData.BurstLimit), Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
		CreatedBy:            createdByUUID,
	})

	if err != nil {
		cli.createAuditLog("service.create.failed", "service", serviceData.ServiceID, false, "Failed to create service: "+err.Error())
		log.Fatal("Failed to create service:", err)
	}

	cli.createAuditLog("service.create", "service", service.ID.String(), true, "")

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
		fmt.Fscanf(reader, "%d\n", &merchantData.RequestsPerSecond)
		if merchantData.RequestsPerSecond == 0 {
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
		cli.createAuditLog("merchant.create.failed", "merchant", merchantData.Slug, false, "Failed to create merchant: "+err.Error())
		log.Fatal("Failed to create merchant:", err)
	}

	merchantID := merchant.ID.String()
	cli.createAuditLog("merchant.create", "merchant", merchantID, true, "")

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

	// Get service and merchant IDs using sqlc
	service, err := cli.queries.GetServiceByServiceID(cli.ctx, serviceID)
	if err != nil {
		cli.createAuditLog("service.grant_access.failed", "service", serviceID, false, "Service not found: "+serviceID)
		log.Fatal("Service not found:", serviceID)
	}

	merchant, err := cli.queries.GetMerchantBySlug(cli.ctx, merchantSlug)
	if err != nil {
		cli.createAuditLog("service.grant_access.failed", "merchant", merchantSlug, false, "Merchant not found: "+merchantSlug)
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
	grantedByUUID := pgtype.UUID{}
	if cli.adminID != "" {
		parsedUUID, err := uuid.Parse(cli.adminID)
		if err == nil {
			grantedByUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
		}
	}

	_, err = cli.queries.GrantServiceAccess(cli.ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  service.ID,
		MerchantID: merchant.ID,
		Scopes:     scopes,
		GrantedBy:  grantedByUUID,
		ExpiresAt:  pgtype.Timestamp{}, // No expiration
	})

	if err != nil {
		cli.createAuditLog("service.grant_access.failed", "service_merchant", service.ID.String()+":"+merchant.ID.String(), false, "Failed to grant access: "+err.Error())
		log.Fatal("Failed to grant access:", err)
	}

	cli.createAuditLog("service.grant_access", "service_merchant", service.ID.String()+":"+merchant.ID.String(), true, "")

	fmt.Println("\n✅ Access granted successfully!")
	fmt.Printf("Service '%s' now has access to merchant '%s'\n", serviceID, merchantSlug)
}

func (cli *AdminCLI) listServices() {
	// List services using sqlc
	services, err := cli.queries.ListServices(cli.ctx, sqlc.ListServicesParams{
		Environment: pgtype.Text{}, // NULL to get all
		IsActive:    pgtype.Bool{},  // NULL to get all
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

func (cli *AdminCLI) listMerchants() {
	// List merchants using sqlc
	merchants, err := cli.queries.ListMerchants(cli.ctx, sqlc.ListMerchantsParams{
		Environment: pgtype.Text{}, // NULL to get all
		IsActive:    pgtype.Bool{},  // NULL to get all
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
			merchant.CreatedAt.Time.Format("2006-01-02 15:04"))
	}
}

func (cli *AdminCLI) autoLogin() {
	// Try to auto-login with first admin account using sqlc
	admins, err := cli.queries.ListAdmins(cli.ctx, sqlc.ListAdminsParams{
		Role:      pgtype.Text{},                        // NULL to get all roles
		IsActive:  pgtype.Bool{Bool: true, Valid: true}, // Only active admins
		LimitVal:  1,
		OffsetVal: 0,
	})

	if err != nil || len(admins) == 0 {
		log.Fatal("No admin account found. Please login first with -action=login")
	}

	cli.adminID = admins[0].ID.String()
	cli.createAuditLog("admin.auto_login", "", cli.adminID, true, "")
}

// createAuditLog creates an audit log entry for admin operations
func (cli *AdminCLI) createAuditLog(action, entityType, entityID string, success bool, errorMsg string) {
	auditUUID := uuid.New()

	actorID := pgtype.Text{}
	actorName := pgtype.Text{}
	if cli.adminID != "" {
		actorID = pgtype.Text{String: cli.adminID, Valid: true}
		// Get admin info for actor name
		parsedAdminID, err := uuid.Parse(cli.adminID)
		if err == nil {
			admin, err := cli.queries.GetAdminByID(cli.ctx, parsedAdminID)
			if err == nil {
				actorName = pgtype.Text{String: admin.Email, Valid: true}
			}
		}
	}

	entityTypeText := pgtype.Text{}
	if entityType != "" {
		entityTypeText = pgtype.Text{String: entityType, Valid: true}
	}

	entityIDText := pgtype.Text{}
	if entityID != "" {
		entityIDText = pgtype.Text{String: entityID, Valid: true}
	}

	errorMsgText := pgtype.Text{}
	if errorMsg != "" {
		errorMsgText = pgtype.Text{String: errorMsg, Valid: true}
	}

	err := cli.queries.CreateAuditLog(cli.ctx, sqlc.CreateAuditLogParams{
		ID:           pgtype.UUID{Bytes: auditUUID, Valid: true},
		ActorType:    pgtype.Text{String: "admin", Valid: true},
		ActorID:      actorID,
		ActorName:    actorName,
		Action:       action,
		EntityType:   entityTypeText,
		EntityID:     entityIDText,
		Changes:      []byte{}, // No changes tracking for CLI operations
		Metadata:     []byte{}, // No metadata for CLI operations
		IpAddress:    nil,
		UserAgent:    pgtype.Text{},
		RequestID:    pgtype.Text{},
		Success:      pgtype.Bool{Bool: success, Valid: true},
		ErrorMessage: errorMsgText,
	})

	if err != nil {
		// Don't fail operations if audit logging fails, just log the error
		log.Printf("Warning: Failed to create audit log: %v", err)
	}
}

func generateFingerprint(publicKeyPEM []byte) string {
	// Simple fingerprint generation
	h := sha256.New()
	h.Write(publicKeyPEM)
	return fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]
}
