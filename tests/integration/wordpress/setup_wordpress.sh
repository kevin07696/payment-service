#!/bin/bash
set -e

echo "🔧 WordPress E2E Test Setup"
echo "═══════════════════════════════════════════════════════════════"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
WP_URL="http://localhost:8082"
WP_ADMIN_USER="admin"
WP_ADMIN_PASS="admin"
WP_ADMIN_EMAIL="admin@example.com"
WP_TITLE="North Payments Test Site"

# Check if WordPress is already installed
echo -e "${BLUE}📍 Checking WordPress installation status...${NC}"
if podman exec north-payments-wpcli wp core is-installed --path=/var/www/html 2>/dev/null; then
    echo -e "${GREEN}✅ WordPress is already installed${NC}"
else
    echo -e "${BLUE}📦 Installing WordPress...${NC}"
    podman exec north-payments-wpcli wp core install \
        --path=/var/www/html \
        --url="$WP_URL" \
        --title="$WP_TITLE" \
        --admin_user="$WP_ADMIN_USER" \
        --admin_password="$WP_ADMIN_PASS" \
        --admin_email="$WP_ADMIN_EMAIL" \
        --skip-email
    echo -e "${GREEN}✅ WordPress installed${NC}"
fi

# Install and activate WooCommerce
echo -e "${BLUE}🛒 Setting up WooCommerce...${NC}"
if podman exec north-payments-wpcli wp plugin is-installed woocommerce --path=/var/www/html 2>/dev/null; then
    echo -e "${GREEN}✅ WooCommerce is already installed${NC}"
else
    podman exec north-payments-wpcli wp plugin install woocommerce --activate --path=/var/www/html
    echo -e "${GREEN}✅ WooCommerce installed and activated${NC}"
fi

# Activate WooCommerce if not active
if ! podman exec north-payments-wpcli wp plugin is-active woocommerce --path=/var/www/html 2>/dev/null; then
    podman exec north-payments-wpcli wp plugin activate woocommerce --path=/var/www/html
    echo -e "${GREEN}✅ WooCommerce activated${NC}"
fi

# Set up WooCommerce basic settings
echo -e "${BLUE}⚙️  Configuring WooCommerce...${NC}"
podman exec north-payments-wpcli wp option update woocommerce_store_address "123 Test Street" --path=/var/www/html
podman exec north-payments-wpcli wp option update woocommerce_store_city "Test City" --path=/var/www/html
podman exec north-payments-wpcli wp option update woocommerce_default_country "US:CA" --path=/var/www/html
podman exec north-payments-wpcli wp option update woocommerce_store_postcode "90210" --path=/var/www/html
podman exec north-payments-wpcli wp option update woocommerce_currency "USD" --path=/var/www/html
podman exec north-payments-wpcli wp option update woocommerce_calc_taxes "no" --path=/var/www/html
echo -e "${GREEN}✅ WooCommerce configured${NC}"

# Check if North Payments plugin exists
echo -e "${BLUE}💳 Setting up North Payments plugin...${NC}"
if podman exec north-payments-wpcli wp plugin is-installed north-payments --path=/var/www/html 2>/dev/null; then
    echo -e "${GREEN}✅ North Payments plugin found${NC}"
    # Activate if not active
    if ! podman exec north-payments-wpcli wp plugin is-active north-payments --path=/var/www/html 2>/dev/null; then
        podman exec north-payments-wpcli wp plugin activate north-payments --path=/var/www/html
        echo -e "${GREEN}✅ North Payments activated${NC}"
    fi
else
    echo -e "${BLUE}ℹ️  North Payments plugin not found - please install manually${NC}"
fi

# Configure North Payments settings
echo -e "${BLUE}⚙️  Configuring North Payments...${NC}"
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings enabled "yes" --path=/var/www/html 2>/dev/null || true
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings title "North Payments" --path=/var/www/html 2>/dev/null || true
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings description "Pay securely with your credit card" --path=/var/www/html 2>/dev/null || true
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings service_url "http://payment-server:8081" --path=/var/www/html 2>/dev/null || true
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings merchant_id "00000000-0000-0000-0000-000000000001" --path=/var/www/html 2>/dev/null || true
podman exec north-payments-wpcli wp option patch update woocommerce_north_payments_settings service_id "test-service-001" --path=/var/www/html 2>/dev/null || true
echo -e "${GREEN}✅ North Payments configured${NC}"

# Create a test product
echo -e "${BLUE}📦 Creating test product...${NC}"
PRODUCT_ID=$(podman exec north-payments-wpcli wp post list \
    --post_type=product \
    --title="Test Product" \
    --format=ids \
    --path=/var/www/html 2>/dev/null | tr -d '\r')

if [ -z "$PRODUCT_ID" ]; then
    PRODUCT_ID=$(podman exec north-payments-wpcli wp wc product create \
        --name="Test Product" \
        --type=simple \
        --regular_price=46.00 \
        --status=publish \
        --user="$WP_ADMIN_USER" \
        --porcelain \
        --path=/var/www/html)
    echo -e "${GREEN}✅ Test product created (ID: $PRODUCT_ID)${NC}"
else
    echo -e "${GREEN}✅ Test product already exists (ID: $PRODUCT_ID)${NC}"
fi

# Flush WordPress rewrite rules
echo -e "${BLUE}🔄 Flushing rewrite rules...${NC}"
podman exec north-payments-wpcli wp rewrite flush --path=/var/www/html
echo -e "${GREEN}✅ Rewrite rules flushed${NC}"

# Clear all caches
echo -e "${BLUE}🧹 Clearing caches...${NC}"
podman exec north-payments-wpcli wp cache flush --path=/var/www/html 2>/dev/null || true
echo -e "${GREEN}✅ Caches cleared${NC}"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo -e "${GREEN}🎉 WordPress E2E test environment is ready!${NC}"
echo ""
echo "📋 Details:"
echo "   URL:      $WP_URL"
echo "   Username: $WP_ADMIN_USER"
echo "   Password: $WP_ADMIN_PASS"
echo ""
echo "   Test Product: Test Product (\$46.00)"
echo ""
echo "You can now run the automated tests:"
echo "   cd /home/kevinlam/Documents/projects/payments"
echo "   go test -v -tags=integration ./tests/integration/wordpress/..."
echo "═══════════════════════════════════════════════════════════════"
