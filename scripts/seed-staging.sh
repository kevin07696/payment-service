#!/bin/bash

# Seed Staging Database
# Applies test data to staging environment

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SEED_DIR="${SEED_DIR:-./db/seeds/staging}"
DB_HOST="${DB_HOST:-${ORACLE_CLOUD_HOST}}"
DB_PORT="${DB_PORT:-1522}"
DB_USER="${DB_USER:-payment_service}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-paymentdb}"
TNS_ADMIN="${TNS_ADMIN:-${HOME}/oracle-wallet}"

# Parse arguments
FORCE=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --force)
            FORCE=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --force      Force seed even if data exists"
            echo "  --dry-run    Show what would be done without executing"
            echo "  --help       Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  DB_HOST      Database host (default: \$ORACLE_CLOUD_HOST)"
            echo "  DB_PORT      Database port (default: 1522)"
            echo "  DB_USER      Database user (default: payment_service)"
            echo "  DB_PASSWORD  Database password (required)"
            echo "  DB_NAME      Database name (default: paymentdb)"
            echo "  TNS_ADMIN    Oracle wallet directory (default: \$HOME/oracle-wallet)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo "=================================================="
echo "Seed Staging Database"
echo "=================================================="
echo ""

# Check for required tools
if ! command -v psql &> /dev/null && ! command -v sqlplus &> /dev/null; then
    echo -e "${RED}❌ Error: Neither psql nor sqlplus found${NC}"
    echo "Please install PostgreSQL client or Oracle SQL*Plus"
    exit 1
fi

# Check for seed files
if [ ! -d "$SEED_DIR" ]; then
    echo -e "${RED}❌ Error: Seed directory not found: $SEED_DIR${NC}"
    exit 1
fi

SEED_FILES=$(find "$SEED_DIR" -name "*.sql" -type f | sort)

if [ -z "$SEED_FILES" ]; then
    echo -e "${RED}❌ Error: No seed files found in $SEED_DIR${NC}"
    exit 1
fi

echo -e "${BLUE}Seed files found:${NC}"
echo "$SEED_FILES" | while read file; do
    echo "  - $(basename "$file")"
done
echo ""

# Check database password
if [ -z "$DB_PASSWORD" ]; then
    echo -e "${YELLOW}⚠️  DB_PASSWORD not set${NC}"
    read -sp "Enter database password: " DB_PASSWORD
    echo ""
fi

# Dry run mode
if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}🔍 DRY RUN MODE${NC}"
    echo "Would execute the following SQL files:"
    echo "$SEED_FILES" | while read file; do
        echo "  - $file"
    done
    echo ""
    echo "No changes will be made."
    exit 0
fi

# Check if data already exists (unless --force)
if [ "$FORCE" != true ]; then
    echo -n "Checking if seed data already exists... "

    # Try to connect and check for test data
    if command -v psql &> /dev/null; then
        # PostgreSQL
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM customers WHERE id LIKE 'cust_test_%'" 2>/dev/null | grep -q "0" || {
            echo -e "${YELLOW}EXISTS${NC}"
            echo ""
            echo -e "${YELLOW}⚠️  Test data already exists in database${NC}"
            echo "Use --force to override and reseed"
            exit 1
        }
    elif command -v sqlplus &> /dev/null; then
        # Oracle
        export TNS_ADMIN
        echo "exit" | sqlplus -S "$DB_USER/$DB_PASSWORD@${DB_NAME}_tp" <<EOF 2>/dev/null | grep -q "no rows selected" || {
            SELECT COUNT(*) FROM customers WHERE id LIKE 'cust_test_%';
            exit;
EOF
            echo -e "${YELLOW}EXISTS${NC}"
            echo ""
            echo -e "${YELLOW}⚠️  Test data already exists in database${NC}"
            echo "Use --force to override and reseed"
            exit 1
        }
    fi

    echo -e "${GREEN}NONE${NC}"
    echo ""
fi

# Apply seed files
echo -e "${BLUE}Applying seed files...${NC}"
echo ""

SUCCESS_COUNT=0
FAIL_COUNT=0

echo "$SEED_FILES" | while read seed_file; do
    filename=$(basename "$seed_file")
    echo -n "  $filename... "

    if command -v psql &> /dev/null; then
        # PostgreSQL
        if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$seed_file" > /tmp/seed_output.log 2>&1; then
            echo -e "${GREEN}✅ SUCCESS${NC}"
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            echo -e "${RED}❌ FAILED${NC}"
            echo "Error output:"
            cat /tmp/seed_output.log
            FAIL_COUNT=$((FAIL_COUNT + 1))
        fi
    elif command -v sqlplus &> /dev/null; then
        # Oracle
        export TNS_ADMIN
        if sqlplus -S "$DB_USER/$DB_PASSWORD@${DB_NAME}_tp" @"$seed_file" > /tmp/seed_output.log 2>&1; then
            echo -e "${GREEN}✅ SUCCESS${NC}"
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            echo -e "${RED}❌ FAILED${NC}"
            echo "Error output:"
            cat /tmp/seed_output.log
            FAIL_COUNT=$((FAIL_COUNT + 1))
        fi
    fi
done

echo ""
echo "=================================================="
echo "Summary"
echo "=================================================="
echo ""

if [ $FAIL_COUNT -eq 0 ]; then
    echo -e "${GREEN}✅ All seed files applied successfully!${NC}"
    echo ""
    echo "Successful: $SUCCESS_COUNT"
    echo "Failed: $FAIL_COUNT"
    echo ""
    echo "Your staging database is now populated with test data."
    echo ""
    echo "Test users:"
    echo "  - john.doe@example.com"
    echo "  - jane.smith@example.com"
    echo "  - bob.wilson@example.com"
    echo ""
    echo "EPX test scenarios:"
    echo "  - approved@epxtest.com (will approve)"
    echo "  - declined@epxtest.com (will decline)"
    echo "  - insufficient@epxtest.com (insufficient funds)"
    echo ""
    exit 0
else
    echo -e "${RED}❌ Some seed files failed!${NC}"
    echo ""
    echo "Successful: $SUCCESS_COUNT"
    echo "Failed: $FAIL_COUNT"
    echo ""
    echo "Please check the error messages above."
    exit 1
fi
