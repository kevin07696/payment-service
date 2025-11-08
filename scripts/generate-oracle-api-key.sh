#!/bin/bash

# Generate Oracle Cloud API Key
# This creates the API key needed for Terraform authentication

set -e

echo "=================================================="
echo "Oracle Cloud API Key Generator"
echo "=================================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Create OCI directory
mkdir -p ~/.oci

echo "🔑 Generating API key pair..."

# Generate RSA key
ssh-keygen -t rsa -b 4096 -f ~/.oci/oci_api_key -N "" -C "Oracle Cloud API Key"

# Convert to PEM format (required by Oracle Cloud)
openssl rsa -in ~/.oci/oci_api_key -out ~/.oci/oci_api_key.pem -traditional

# Set permissions
chmod 600 ~/.oci/oci_api_key
chmod 600 ~/.oci/oci_api_key.pem
chmod 644 ~/.oci/oci_api_key.pub

echo -e "${GREEN}✅ API key generated!${NC}"
echo ""

# Get fingerprint
echo "📋 API Key Fingerprint:"
FINGERPRINT=$(openssl rsa -pubout -outform DER -in ~/.oci/oci_api_key.pem 2>/dev/null | openssl md5 -c | cut -d= -f2 | tr -d ' ')
echo -e "${GREEN}$FINGERPRINT${NC}"
echo ""

# Display public key
echo "📤 Public Key (upload to Oracle Cloud):"
echo "=================================================="
cat ~/.oci/oci_api_key.pub
echo "=================================================="
echo ""

# Save fingerprint to file
echo "$FINGERPRINT" > ~/.oci/oci_fingerprint.txt

echo -e "${YELLOW}Next steps:${NC}"
echo ""
echo "1. Copy the public key above"
echo ""
echo "2. Go to Oracle Cloud Console:"
echo "   https://cloud.oracle.com"
echo ""
echo "3. Click Profile Icon → User Settings"
echo ""
echo "4. Click 'API Keys' (left sidebar)"
echo ""
echo "5. Click 'Add API Key'"
echo ""
echo "6. Select 'Paste Public Key'"
echo ""
echo "7. Paste the public key"
echo ""
echo "8. Click 'Add'"
echo ""
echo "9. Verify the fingerprint matches:"
echo "   Expected: $FINGERPRINT"
echo ""
echo "=================================================="
echo ""
echo "Files created:"
echo "  Private key:  ~/.oci/oci_api_key.pem"
echo "  Public key:   ~/.oci/oci_api_key.pub"
echo "  Fingerprint:  ~/.oci/oci_fingerprint.txt"
echo ""
echo "For GitHub Secrets, you'll need:"
echo "  OCI_FINGERPRINT: $FINGERPRINT"
echo "  OCI_PRIVATE_KEY: (content of ~/.oci/oci_api_key.pem)"
echo ""
echo "To view private key:"
echo "  cat ~/.oci/oci_api_key.pem"
echo ""
echo "=================================================="
