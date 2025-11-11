#!/bin/bash
#
# Script to list and clean up Oracle Autonomous Databases
# This helps free up the Oracle Free Tier quota (max 2 Always Free databases)
#

set -e

echo "🔍 Checking Oracle Autonomous Databases..."
echo ""

# Check if OCI CLI is configured
if ! command -v oci &> /dev/null; then
    echo "❌ OCI CLI not found. Please install it first:"
    echo "   https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm"
    exit 1
fi

# Get compartment OCID from environment or prompt user
if [ -z "$OCI_COMPARTMENT_OCID" ]; then
    echo "❌ OCI_COMPARTMENT_OCID environment variable not set"
    echo ""
    echo "To find your compartment OCID:"
    echo "1. Go to: https://cloud.oracle.com"
    echo "2. Navigate to: Identity & Security > Compartments"
    echo "3. Click on your compartment and copy the OCID"
    echo ""
    echo "Then export it: export OCI_COMPARTMENT_OCID=ocid1.compartment.oc1..."
    exit 1
fi

# List all Always Free databases
echo "📊 Always Free Autonomous Databases in your compartment:"
echo ""

ALL_DBS=$(oci db autonomous-database list \
    --compartment-id "$OCI_COMPARTMENT_OCID" \
    --all 2>/dev/null | \
    jq -r '.data[]? | select(.["is-free-tier"] == true) |
        {
            name: ."display-name",
            id: .id,
            state: ."lifecycle-state",
            created: ."time-created"
        } |
        "\(.name)\n  ID: \(.id)\n  State: \(.state)\n  Created: \(.created)\n"' \
    || echo "No databases found or error querying OCI")

if [ -z "$ALL_DBS" ]; then
    echo "✅ No Always Free databases found!"
    echo "You can proceed with deployment."
    exit 0
fi

echo "$ALL_DBS"

# Count databases
DB_COUNT=$(oci db autonomous-database list \
    --compartment-id "$OCI_COMPARTMENT_OCID" \
    --lifecycle-state AVAILABLE \
    --all 2>/dev/null | \
    jq '([.data[]? | select(.["is-free-tier"] == true)] | length) // 0')

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Total Always Free databases: $DB_COUNT / 2 (Oracle limit)"
echo ""

if [ "$DB_COUNT" -lt 2 ]; then
    echo "✅ You have quota available for new databases"
    exit 0
fi

echo "⚠️  Quota Full: You must delete at least 1 database to deploy"
echo ""
echo "To delete a database manually:"
echo "1. Go to: https://cloud.oracle.com"
echo "2. Navigate to: Databases > Autonomous Database"
echo "3. Select the database you want to delete"
echo "4. Click 'More Actions' > 'Terminate'"
echo ""
echo "Or use the OCI CLI:"
echo "  oci db autonomous-database delete --autonomous-database-id <ID> --force"
echo ""

# Offer to delete via script
read -p "Would you like to delete a database now? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Exiting. Please manually delete a database before deploying."
    exit 1
fi

# List databases for deletion
echo ""
echo "Available databases:"
oci db autonomous-database list \
    --compartment-id "$OCI_COMPARTMENT_OCID" \
    --lifecycle-state AVAILABLE \
    --all 2>/dev/null | \
    jq -r '.data[]? | select(.["is-free-tier"] == true) |
        "\nName: \(."display-name")\nID: \(.id)\n"'

echo ""
read -p "Enter the database ID to delete (or 'cancel' to exit): " DB_ID

if [ "$DB_ID" = "cancel" ] || [ -z "$DB_ID" ]; then
    echo "Cancelled."
    exit 1
fi

# Confirm deletion
read -p "⚠️  Are you sure you want to delete database $DB_ID? (yes/NO): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "Cancelled."
    exit 1
fi

# Delete the database
echo ""
echo "🗑️  Deleting database..."
if oci db autonomous-database delete --autonomous-database-id "$DB_ID" --force 2>&1; then
    echo "✅ Database deletion initiated"
    echo ""
    echo "⏳ Waiting for deletion to complete (this may take a few minutes)..."

    for i in {1..30}; do
        STATE=$(oci db autonomous-database get --autonomous-database-id "$DB_ID" 2>/dev/null | \
                jq -r '.data["lifecycle-state"]' || echo "TERMINATED")

        if [ "$STATE" = "TERMINATED" ]; then
            echo "✅ Database successfully deleted!"
            break
        fi

        echo "  Status: $STATE (attempt $i/30)"
        sleep 10
    done

    echo ""
    echo "✅ Quota freed! You can now proceed with deployment."
else
    echo "❌ Failed to delete database"
    exit 1
fi
