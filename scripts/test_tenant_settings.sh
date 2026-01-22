#!/bin/bash

# Test Tenant Settings Endpoints
# Requires: jq, curl, running HelloJohn instance

set -e

BASE_URL="${BASE_URL:-http://localhost:8082}"
TENANT_ID="${TENANT_ID:-local}"

echo "=== Testing Tenant Settings Endpoints ==="
echo ""

# 1. Get admin token (assuming admin user exists)
# For testing, you might need to manually get a token
# TOKEN="your_admin_token_here"

if [ -z "$TOKEN" ]; then
  echo "ERROR: TOKEN environment variable not set"
  echo "Please export TOKEN with a valid admin JWT token"
  echo "Example: export TOKEN='eyJ...'"
  exit 1
fi

echo "1. GET /v2/admin/tenants/$TENANT_ID/settings"
echo "-------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE"

if [ "$HTTP_CODE" = "200" ]; then
  echo "Response:"
  echo "$BODY" | jq '.'

  # Extract ETag for next request
  ETAG=$(curl -s -I \
    -H "Authorization: Bearer $TOKEN" \
    "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings" | \
    grep -i "etag:" | cut -d' ' -f2 | tr -d '\r')

  echo ""
  echo "ETag: $ETAG"
else
  echo "Error: $BODY"
  exit 1
fi

echo ""
echo "2. PUT /v2/admin/tenants/$TENANT_ID/settings (Update SMTP)"
echo "-----------------------------------------------------------"

# Update SMTP settings
UPDATE_PAYLOAD=$(cat <<'EOF'
{
  "issuer_mode": "path",
  "smtp": {
    "host": "smtp.example.com",
    "port": 587,
    "username": "test@example.com",
    "from_email": "noreply@example.com",
    "use_tls": true
  },
  "branding": {
    "logo_url": "/assets/logo.png",
    "brand_color": "#0066cc"
  }
}
EOF
)

echo "Payload:"
echo "$UPDATE_PAYLOAD" | jq '.'

if [ -z "$ETAG" ]; then
  echo "ERROR: No ETag available, skipping update"
  exit 1
fi

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

if [ "$HTTP_CODE" = "200" ]; then
  echo "Response:"
  echo "$BODY" | jq '.'

  # Extract new ETag
  NEW_ETAG=$(echo "$BODY" | jq -r '.etag // empty')
  echo "New ETag: $NEW_ETAG"
else
  echo "Error: $BODY"
  exit 1
fi

echo ""
echo "3. Verify update - GET /v2/admin/tenants/$TENANT_ID/settings"
echo "-------------------------------------------------------------"
VERIFY_RESPONSE=$(curl -s \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/v2/admin/tenants/$TENANT_ID/settings")

echo "$VERIFY_RESPONSE" | jq '.smtp'

echo ""
echo "=== All tests passed! ==="
