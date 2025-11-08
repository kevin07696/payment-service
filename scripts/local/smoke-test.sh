#!/bin/bash

# Smoke Test Script for Staging Environment
# Tests critical endpoints to verify deployment succeeded

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
TARGET_HOST="${1:-${ORACLE_CLOUD_HOST}}"
HTTP_PORT="${2:-8081}"
GRPC_PORT="${3:-8080}"
TIMEOUT=10

if [ -z "$TARGET_HOST" ]; then
    echo -e "${RED}❌ Error: TARGET_HOST not provided${NC}"
    echo "Usage: $0 <host> [http_port] [grpc_port]"
    echo "Example: $0 123.45.67.89 8081 8080"
    exit 1
fi

echo "=================================================="
echo "Smoke Test - Staging Deployment Verification"
echo "=================================================="
echo ""
echo "Target: $TARGET_HOST"
echo "HTTP Port: $HTTP_PORT"
echo "gRPC Port: $GRPC_PORT"
echo ""

TESTS_PASSED=0
TESTS_FAILED=0

# Function to test HTTP endpoint
test_http() {
    local name="$1"
    local endpoint="$2"
    local expected_code="${3:-200}"

    echo -n "Testing $name... "

    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -m $TIMEOUT "http://${TARGET_HOST}:${HTTP_PORT}${endpoint}" 2>/dev/null)

    if [ "$HTTP_CODE" = "$expected_code" ]; then
        echo -e "${GREEN}✅ PASS${NC} (HTTP $HTTP_CODE)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}❌ FAIL${NC} (Expected $expected_code, got $HTTP_CODE)"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Function to test gRPC connectivity
test_grpc() {
    local name="$1"

    echo -n "Testing $name... "

    # Use nc (netcat) to check if port is open
    if command -v nc >/dev/null 2>&1; then
        if nc -z -w $TIMEOUT "$TARGET_HOST" "$GRPC_PORT" 2>/dev/null; then
            echo -e "${GREEN}✅ PASS${NC} (Port $GRPC_PORT open)"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        else
            echo -e "${RED}❌ FAIL${NC} (Port $GRPC_PORT not accessible)"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            return 1
        fi
    else
        # Fallback: use telnet if nc not available
        if timeout $TIMEOUT bash -c "echo > /dev/tcp/$TARGET_HOST/$GRPC_PORT" 2>/dev/null; then
            echo -e "${GREEN}✅ PASS${NC} (Port $GRPC_PORT open)"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        else
            echo -e "${RED}❌ FAIL${NC} (Port $GRPC_PORT not accessible)"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            return 1
        fi
    fi
}

# Function to test response time
test_response_time() {
    local name="$1"
    local endpoint="$2"
    local max_time="${3:-2}"

    echo -n "Testing $name response time... "

    RESPONSE_TIME=$(curl -s -o /dev/null -w "%{time_total}" -m $TIMEOUT "http://${TARGET_HOST}:${HTTP_PORT}${endpoint}" 2>/dev/null)

    # Convert to integer comparison (multiply by 1000 for milliseconds)
    RESPONSE_MS=$(echo "$RESPONSE_TIME * 1000" | bc | cut -d. -f1)
    MAX_MS=$(echo "$max_time * 1000" | bc | cut -d. -f1)

    if [ "$RESPONSE_MS" -lt "$MAX_MS" ]; then
        echo -e "${GREEN}✅ PASS${NC} (${RESPONSE_TIME}s < ${max_time}s)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${YELLOW}⚠️  SLOW${NC} (${RESPONSE_TIME}s > ${max_time}s)"
        # Don't fail, just warn
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    fi
}

echo "Running smoke tests..."
echo ""

# Critical Tests
echo "Critical Endpoints:"
test_http "Health Check" "/cron/health" "200"
test_http "Stats Endpoint" "/cron/stats" "200"
echo ""

# Connectivity Tests
echo "Connectivity:"
test_grpc "gRPC Server"
echo ""

# Performance Tests
echo "Performance:"
test_response_time "Health Check" "/cron/health" "1.0"
test_response_time "Stats Endpoint" "/cron/stats" "3.0"
echo ""

# Summary
echo "=================================================="
echo "Summary"
echo "=================================================="
echo ""

TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "Total: $TOTAL_TESTS tests"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $TESTS_FAILED"
    echo ""
    echo "Staging deployment is healthy! 🎉"
    exit 0
else
    echo -e "${RED}❌ Some tests failed!${NC}"
    echo ""
    echo "Total: $TOTAL_TESTS tests"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $TESTS_FAILED"
    echo ""
    echo "Please investigate the failures before proceeding."
    exit 1
fi
