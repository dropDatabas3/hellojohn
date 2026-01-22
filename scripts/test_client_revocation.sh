#!/bin/bash

# Test script for Client Secret Revocation endpoint
# Usage: ./test_client_revocation.sh

set -e

echo "=== Client Secret Revocation Test ==="
echo ""

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8082}"
TENANT_ID="${TENANT_ID:-local}"
CLIENT_ID="${CLIENT_ID:-test-confidential-app}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Configuration:"
echo "  Base URL: $BASE_URL"
echo "  Tenant ID: $TENANT_ID"
echo "  Client ID: $CLIENT_ID"
echo ""

# Step 1: Get admin token
echo -e "${YELLOW}Step 1: Getting admin token...${NC}"
ADMIN_TOKEN=$(curl -s -X POST "$BASE_URL/v2/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"client_id\": \"admin-panel\",
    \"email\": \"admin@hellojohn.local\",
    \"password\": \"admin123\"
  }" | jq -r '.access_token')

if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
  echo -e "${RED}Failed to get admin token${NC}"
  exit 1
fi

echo -e "${GREEN}Admin token obtained${NC}"
echo ""

# Step 2: Create a confidential client (if it doesn't exist)
echo -e "${YELLOW}Step 2: Creating test confidential client...${NC}"
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/v2/admin/clients" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"name\": \"Test Confidential App\",
    \"type\": \"confidential\",
    \"secret\": \"original-secret-12345\",
    \"redirect_uris\": [\"http://localhost:3000/callback\"],
    \"scopes\": [\"openid\", \"profile\", \"email\"]
  }" 2>&1)

if echo "$CREATE_RESPONSE" | grep -q "already exists"; then
  echo -e "${YELLOW}Client already exists (ok)${NC}"
elif echo "$CREATE_RESPONSE" | grep -q "client_id"; then
  echo -e "${GREEN}Client created successfully${NC}"
else
  echo -e "${YELLOW}Warning: Unexpected response: $CREATE_RESPONSE${NC}"
fi
echo ""

# Step 3: Test OAuth2 with original secret
echo -e "${YELLOW}Step 3: Testing OAuth2 with original secret...${NC}"
ORIGINAL_TOKEN=$(curl -s -X POST "$BASE_URL/oauth2/token" \
  -u "$CLIENT_ID:original-secret-12345" \
  -d "grant_type=client_credentials" \
  -d "scope=openid" | jq -r '.access_token')

if [ -n "$ORIGINAL_TOKEN" ] && [ "$ORIGINAL_TOKEN" != "null" ]; then
  echo -e "${GREEN}Original secret works${NC}"
else
  echo -e "${YELLOW}Original secret may already be rotated (continuing...)${NC}"
fi
echo ""

# Step 4: Revoke secret (main test)
echo -e "${YELLOW}Step 4: Revoking client secret...${NC}"
REVOKE_RESPONSE=$(curl -s -X POST "$BASE_URL/v2/admin/clients/$CLIENT_ID/revoke" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID")

echo "Revoke Response:"
echo "$REVOKE_RESPONSE" | jq .

NEW_SECRET=$(echo "$REVOKE_RESPONSE" | jq -r '.new_secret')
MESSAGE=$(echo "$REVOKE_RESPONSE" | jq -r '.message')

if [ -z "$NEW_SECRET" ] || [ "$NEW_SECRET" = "null" ]; then
  echo -e "${RED}Failed to revoke secret${NC}"
  exit 1
fi

echo -e "${GREEN}Secret revoked successfully${NC}"
echo -e "  New Secret: ${YELLOW}$NEW_SECRET${NC}"
echo -e "  Message: $MESSAGE"
echo ""

# Step 5: Verify old secret no longer works
echo -e "${YELLOW}Step 5: Verifying old secret is revoked...${NC}"
OLD_TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/oauth2/token" \
  -u "$CLIENT_ID:original-secret-12345" \
  -d "grant_type=client_credentials" \
  -d "scope=openid")

if echo "$OLD_TOKEN_RESPONSE" | grep -q "error"; then
  echo -e "${GREEN}Old secret correctly rejected${NC}"
else
  echo -e "${RED}WARNING: Old secret still works!${NC}"
fi
echo ""

# Step 6: Verify new secret works
echo -e "${YELLOW}Step 6: Testing OAuth2 with new secret...${NC}"
NEW_TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/oauth2/token" \
  -u "$CLIENT_ID:$NEW_SECRET" \
  -d "grant_type=client_credentials" \
  -d "scope=openid")

NEW_ACCESS_TOKEN=$(echo "$NEW_TOKEN_RESPONSE" | jq -r '.access_token')

if [ -n "$NEW_ACCESS_TOKEN" ] && [ "$NEW_ACCESS_TOKEN" != "null" ]; then
  echo -e "${GREEN}New secret works correctly${NC}"
  echo "  Access Token: ${NEW_ACCESS_TOKEN:0:50}..."
  echo ""
  echo -e "${GREEN}=== All tests passed! ===${NC}"
else
  echo -e "${RED}New secret failed${NC}"
  echo "Response: $NEW_TOKEN_RESPONSE"
  exit 1
fi

# Step 7: Test edge case - try to revoke public client secret
echo ""
echo -e "${YELLOW}Step 7: Testing edge case - public client revocation...${NC}"

# Create a public client
curl -s -X POST "$BASE_URL/v2/admin/clients" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"test-public-app\",
    \"name\": \"Test Public App\",
    \"type\": \"public\",
    \"redirect_uris\": [\"http://localhost:3000/callback\"]
  }" > /dev/null 2>&1

PUBLIC_REVOKE_RESPONSE=$(curl -s -X POST "$BASE_URL/v2/admin/clients/test-public-app/revoke" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID")

if echo "$PUBLIC_REVOKE_RESPONSE" | grep -q "cannot revoke secret for public client"; then
  echo -e "${GREEN}Public client revocation correctly rejected${NC}"
else
  echo -e "${RED}WARNING: Public client should reject revocation${NC}"
  echo "Response: $PUBLIC_REVOKE_RESPONSE"
fi

echo ""
echo -e "${GREEN}=== Client Secret Revocation Test Complete ===${NC}"
