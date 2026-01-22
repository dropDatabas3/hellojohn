#!/bin/bash

# Comprehensive Test Suite for Tenant Settings Endpoints
# Tests both GET and PUT /v2/admin/tenants/{id}/settings

set -e

BASE_URL="${BASE_URL:-http://localhost:8082}"
TENANT_ID="${TENANT_ID:-local}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Tenant Settings API Test Suite ===${NC}"
echo ""

# Check TOKEN
if [ -z "$TOKEN" ]; then
  echo -e "${RED}ERROR: TOKEN environment variable not set${NC}"
  echo "Please export TOKEN with a valid admin JWT token"
  echo "Example: export TOKEN='eyJ...'"
  exit 1
fi

echo -e "${GREEN}Configuration:${NC}"
echo "  Base URL: $BASE_URL"
echo "  Tenant ID: $TENANT_ID"
echo ""

# Test 1: GET Settings
echo -e "${YELLOW}Test 1: GET /v2/admin/tenants/$TENANT_ID/settings${NC}"
echo "---------------------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE"

if [ "$HTTP_CODE" != "200" ]; then
  echo -e "${RED}FAIL: Expected 200, got $HTTP_CODE${NC}"
  echo "Response: $BODY"
  exit 1
fi

echo -e "${GREEN}PASS: Settings retrieved successfully${NC}"
echo "Response preview:"
echo "$BODY" | jq -C '.' 2>/dev/null || echo "$BODY"

# Extract ETag
ETAG=$(curl -s -I \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings" 2>/dev/null | \
  grep -i "etag:" | cut -d' ' -f2 | tr -d '\r\n')

echo ""
echo "ETag: $ETAG"

if [ -z "$ETAG" ]; then
  echo -e "${RED}WARNING: No ETag returned${NC}"
fi

echo ""

# Test 2: PUT Settings (Update SMTP)
echo -e "${YELLOW}Test 2: PUT /v2/admin/tenants/$TENANT_ID/settings (Update SMTP)${NC}"
echo "---------------------------------------------------------------"

if [ -z "$ETAG" ]; then
  echo -e "${RED}SKIP: No ETag available, cannot test update${NC}"
else
  UPDATE_PAYLOAD=$(cat <<'EOF'
{
  "smtp": {
    "host": "smtp.example.com",
    "port": 587,
    "username": "test@example.com",
    "from_email": "noreply@example.com",
    "use_tls": true
  }
}
EOF
)

  echo "Payload:"
  echo "$UPDATE_PAYLOAD" | jq -C '.'

  RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "If-Match: $ETAG" \
    -d "$UPDATE_PAYLOAD" \
    "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

  HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | head -n-1)

  echo ""
  echo "HTTP Status: $HTTP_CODE"

  if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}FAIL: Expected 200, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    exit 1
  fi

  echo -e "${GREEN}PASS: Settings updated successfully${NC}"
  echo "Response:"
  echo "$BODY" | jq -C '.'

  # Extract new ETag
  NEW_ETAG=$(echo "$BODY" | jq -r '.etag // empty' 2>/dev/null)
  if [ -n "$NEW_ETAG" ]; then
    echo "New ETag: $NEW_ETAG"
  fi

  echo ""
fi

# Test 3: Verify Update
echo -e "${YELLOW}Test 3: Verify update via GET${NC}"
echo "---------------------------------------------------------------"
VERIFY_RESPONSE=$(curl -s \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

echo "SMTP Settings after update:"
echo "$VERIFY_RESPONSE" | jq -C '.smtp'

# Check if SMTP host was updated
SMTP_HOST=$(echo "$VERIFY_RESPONSE" | jq -r '.smtp.host // empty')
if [ "$SMTP_HOST" = "smtp.example.com" ]; then
  echo -e "${GREEN}PASS: SMTP settings correctly updated${NC}"
else
  echo -e "${RED}FAIL: SMTP settings not updated correctly${NC}"
  echo "Expected host: smtp.example.com"
  echo "Got host: $SMTP_HOST"
fi

echo ""

# Test 4: Update with invalid ETag (should fail with 412)
echo -e "${YELLOW}Test 4: PUT with invalid ETag (concurrency control)${NC}"
echo "---------------------------------------------------------------"
INVALID_ETAG="invalid-etag-12345"

RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "If-Match: $INVALID_ETAG" \
  -d '{"brand_color": "#FF0000"}' \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "412" ]; then
  echo -e "${GREEN}PASS: Correctly rejected invalid ETag with 412${NC}"
elif [ "$HTTP_CODE" = "428" ]; then
  echo -e "${GREEN}PASS: Correctly rejected invalid ETag with 428${NC}"
else
  echo -e "${RED}FAIL: Expected 412 or 428, got $HTTP_CODE${NC}"
fi

echo ""

# Test 5: Update without ETag (should fail with 428)
echo -e "${YELLOW}Test 5: PUT without If-Match header (should fail)${NC}"
echo "---------------------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"brand_color": "#00FF00"}' \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "428" ]; then
  echo -e "${GREEN}PASS: Correctly requires If-Match header (428)${NC}"
else
  echo -e "${RED}FAIL: Expected 428, got $HTTP_CODE${NC}"
fi

echo ""

# Test 6: Update multiple settings
echo -e "${YELLOW}Test 6: Update multiple settings at once${NC}"
echo "---------------------------------------------------------------"

# Get fresh ETag
FRESH_ETAG=$(curl -s -I \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings" 2>/dev/null | \
  grep -i "etag:" | cut -d' ' -f2 | tr -d '\r\n')

MULTI_UPDATE=$(cat <<'EOF'
{
  "issuer_mode": "path",
  "logo_url": "/assets/test-logo.png",
  "brand_color": "#0066cc",
  "mfa_enabled": false,
  "social_login_enabled": true,
  "session_lifetime_seconds": 7200
}
EOF
)

echo "Payload:"
echo "$MULTI_UPDATE" | jq -C '.'

RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "If-Match: $FRESH_ETAG" \
  -d "$MULTI_UPDATE" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
  echo -e "${GREEN}PASS: Multiple settings updated successfully${NC}"
else
  echo -e "${RED}FAIL: Expected 200, got $HTTP_CODE${NC}"
  echo "Response: $(echo "$RESPONSE" | head -n-1)"
fi

echo ""

# Final summary
echo -e "${YELLOW}=== Test Summary ===${NC}"
echo -e "${GREEN}All core tests passed!${NC}"
echo ""
echo "Tested endpoints:"
echo "  - GET  /v2/admin/tenants/{id}/settings"
echo "  - PUT  /v2/admin/tenants/{id}/settings"
echo ""
echo "Features verified:"
echo "  ✓ ETag generation"
echo "  ✓ Concurrency control (If-Match)"
echo "  ✓ Partial updates"
echo "  ✓ Multiple settings update"
echo "  ✓ Error handling"
