#!/bin/bash

# test_user_crud.sh - Test script for User CRUD endpoints
# Run this after starting the V2 server

set -e

# Configuration
SERVER_URL="${SERVER_URL:-http://localhost:8082}"
TENANT_ID="${TENANT_ID:-local}"
TOKEN="${ADMIN_TOKEN:-}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
print_step() {
    echo -e "${YELLOW}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Check if server is running
print_step "Checking if server is running"
if ! curl -s "${SERVER_URL}/readyz" > /dev/null; then
    print_error "Server is not running at ${SERVER_URL}"
    exit 1
fi
print_success "Server is running"

# Check if TOKEN is set
if [ -z "$TOKEN" ]; then
    print_error "ADMIN_TOKEN environment variable is not set"
    echo "Please set it with: export ADMIN_TOKEN='your-admin-token'"
    exit 1
fi

# Test 1: Create user
print_step "Test 1: Create User"
CREATE_RESPONSE=$(curl -s -X POST "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test-user-crud@example.com",
    "password_hash": "$2a$10$abcdefghijklmnopqrstuvwxyz123456",
    "name": "Test CRUD User",
    "given_name": "Test",
    "family_name": "User",
    "locale": "en"
  }')

USER_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id')

if [ "$USER_ID" == "null" ] || [ -z "$USER_ID" ]; then
    print_error "Failed to create user"
    echo "Response: $CREATE_RESPONSE"
    exit 1
fi

print_success "User created with ID: $USER_ID"

# Test 2: List users
print_step "Test 2: List Users (page 1, page_size 10)"
LIST_RESPONSE=$(curl -s "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users?page=1&page_size=10" \
  -H "Authorization: Bearer ${TOKEN}")

USER_COUNT=$(echo "$LIST_RESPONSE" | jq '.users | length')
print_success "Found ${USER_COUNT} users"

# Test 3: Get specific user
print_step "Test 3: Get User by ID"
GET_RESPONSE=$(curl -s "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users/${USER_ID}" \
  -H "Authorization: Bearer ${TOKEN}")

USER_EMAIL=$(echo "$GET_RESPONSE" | jq -r '.email')

if [ "$USER_EMAIL" != "test-user-crud@example.com" ]; then
    print_error "Failed to get user. Expected email: test-user-crud@example.com, got: $USER_EMAIL"
    exit 1
fi

print_success "User retrieved: $USER_EMAIL"

# Test 4: Update user
print_step "Test 4: Update User"
UPDATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users/${USER_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated CRUD User",
    "given_name": "Updated"
  }')

HTTP_CODE=$(echo "$UPDATE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" != "204" ]; then
    print_error "Failed to update user. HTTP code: $HTTP_CODE"
    exit 1
fi

print_success "User updated successfully"

# Test 5: Verify update
print_step "Test 5: Verify Update"
VERIFY_RESPONSE=$(curl -s "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users/${USER_ID}" \
  -H "Authorization: Bearer ${TOKEN}")

UPDATED_NAME=$(echo "$VERIFY_RESPONSE" | jq -r '.name')

if [ "$UPDATED_NAME" != "Updated CRUD User" ]; then
    print_error "Update verification failed. Expected name: Updated CRUD User, got: $UPDATED_NAME"
    exit 1
fi

print_success "Update verified: $UPDATED_NAME"

# Test 6: Search users
print_step "Test 6: Search Users"
SEARCH_RESPONSE=$(curl -s "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users?search=crud" \
  -H "Authorization: Bearer ${TOKEN}")

SEARCH_COUNT=$(echo "$SEARCH_RESPONSE" | jq '.users | length')
print_success "Search found ${SEARCH_COUNT} users matching 'crud'"

# Test 7: Delete user
print_step "Test 7: Delete User"
DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users/${USER_ID}" \
  -H "Authorization: Bearer ${TOKEN}")

HTTP_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" != "204" ]; then
    print_error "Failed to delete user. HTTP code: $HTTP_CODE"
    exit 1
fi

print_success "User deleted successfully"

# Test 8: Verify deletion
print_step "Test 8: Verify Deletion"
VERIFY_DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" "${SERVER_URL}/v2/admin/tenants/${TENANT_ID}/users/${USER_ID}" \
  -H "Authorization: Bearer ${TOKEN}")

HTTP_CODE=$(echo "$VERIFY_DELETE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" != "404" ]; then
    print_error "User still exists after deletion. HTTP code: $HTTP_CODE"
    exit 1
fi

print_success "Deletion verified (404 returned)"

# Summary
echo ""
print_step "All Tests Passed!"
echo -e "${GREEN}✓ Create User${NC}"
echo -e "${GREEN}✓ List Users${NC}"
echo -e "${GREEN}✓ Get User${NC}"
echo -e "${GREEN}✓ Update User${NC}"
echo -e "${GREEN}✓ Verify Update${NC}"
echo -e "${GREEN}✓ Search Users${NC}"
echo -e "${GREEN}✓ Delete User${NC}"
echo -e "${GREEN}✓ Verify Deletion${NC}"
