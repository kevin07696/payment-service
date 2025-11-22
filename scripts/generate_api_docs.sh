#!/bin/bash
# generate_api_docs.sh - Generates API documentation from proto files
# Usage: ./scripts/generate_api_docs.sh

set -e

echo "📡 Generating API documentation from proto files..."

OUTPUT_FILE="docs/integration/API_SPECS.md"
PROTO_DIR="proto"

# Check if protoc-gen-doc is installed
if ! command -v protoc-gen-doc &> /dev/null; then
    echo "⚠️  protoc-gen-doc not found. Installing..."
    echo "   Run: go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest"
    echo ""
    echo "   Falling back to basic documentation generation..."
    echo ""
fi

# Create basic header
cat > "$OUTPUT_FILE" <<EOF
# API Specification (Auto-Generated)

**Auto-Generated:** $(date '+%Y-%m-%d %H:%M:%S')
**Source:** Protocol Buffer definitions in \`$PROTO_DIR\`

---

## Overview

This document is auto-generated from Protocol Buffer (proto) definitions. For hand-written integration guides and examples, see:
- **[PAYMENT_CLI.md](./PAYMENT_CLI.md)** - Service and merchant management
- **[REACT_INTEGRATION.md](./REACT_INTEGRATION.md)** - React/TypeScript integration
- **[BROWSER_POST_FORM_SETUP.md](./BROWSER_POST_FORM_SETUP.md)** - PCI-compliant tokenization

---

## Services

EOF

# Function to extract service names from proto files
extract_services() {
    local proto_file=$1
    local service_name=$(basename "$proto_file" .proto)

    echo "Processing $proto_file..."

    # Extract service definitions
    if grep -q "^service " "$proto_file"; then
        echo "### ${service_name^} Service" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "**Proto File:** \`$proto_file\`" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"

        # Extract service documentation comments
        awk '/^service / {
            service_name=$2
            print "**Service Name:** " service_name
            print ""
        }' "$proto_file" >> "$OUTPUT_FILE"

        # Extract RPC methods
        echo "**RPC Methods:**" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"

        awk '
        /rpc / {
            # Extract method name, request, and response
            match($0, /rpc ([A-Za-z0-9_]+)\s*\(([^)]+)\)\s*returns\s*\(([^)]+)\)/, arr)
            if (arr[1]) {
                print "- **" arr[1] "**"
                print "  - Request: `" arr[2] "`"
                print "  - Response: `" arr[3] "`"
                print ""
            }
        }
        ' "$proto_file" >> "$OUTPUT_FILE"

        echo "" >> "$OUTPUT_FILE"
        echo "---" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    fi
}

# Process all proto files
for proto_file in "$PROTO_DIR"/**/*.proto; do
    [ -f "$proto_file" ] || continue
    extract_services "$proto_file"
done

# Add message types section
cat >> "$OUTPUT_FILE" <<EOF

## Message Types

For detailed message field definitions, refer to the proto files directly:

EOF

for proto_file in "$PROTO_DIR"/**/*.proto; do
    [ -f "$proto_file" ] || continue
    filename=$(basename "$proto_file")
    echo "- [\`$filename\`](../../$proto_file)" >> "$OUTPUT_FILE"
done

# Add footer
cat >> "$OUTPUT_FILE" <<EOF

---

## Using the API

### ConnectRPC (Recommended)

\`\`\`typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from './gen/payment/v1/payment_connect';

const transport = createConnectTransport({
  baseUrl: 'https://api.example.com',
});

const client = createPromiseClient(PaymentService, transport);

// Example: Create sale transaction
const response = await client.sale({
  amountCents: BigInt(10000),
  paymentMethodId: 'pm_abc123',
  idempotencyKey: \\\`sale_\\\${Date.now()}_\\\${crypto.randomUUID()}\\\`,
});
\`\`\`

### gRPC (Go)

\`\`\`go
import (
    "context"
    "google.golang.org/grpc"
    paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
)

conn, err := grpc.Dial("api.example.com:8080", grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := paymentv1.NewPaymentServiceClient(conn)

response, err := client.Sale(context.Background(), &paymentv1.SaleRequest{
    AmountCents: 10000,
    PaymentMethodId: "pm_abc123",
    IdempotencyKey: "sale_1234567890",
})
\`\`\`

---

## Generating Updated Documentation

This file is auto-generated. To regenerate:

\`\`\`bash
# Regenerate all docs
make docs

# Or just API docs
make docs-api
\`\`\`

For detailed API documentation with request/response examples, install protoc-gen-doc:

\`\`\`bash
go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
\`\`\`

---

## Related Documentation

- **[Authentication](../development/AUTH.md)** - JWT token authentication
- **[PAYMENT_CLI.md](./PAYMENT_CLI.md)** - Creating services and obtaining credentials
- **[REACT_INTEGRATION.md](./REACT_INTEGRATION.md)** - Frontend integration guide
EOF

echo "✅ API documentation generated: $OUTPUT_FILE"
echo "   Processed $(find $PROTO_DIR -name "*.proto" | wc -l) proto files"
echo ""
echo "💡 Tip: Install protoc-gen-doc for richer documentation:"
echo "   go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest"
