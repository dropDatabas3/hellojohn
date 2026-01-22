#!/bin/bash
# Setup V2 - Minimal Viable Configuration
# Creates the minimum required setup to run cmd/service_v2/main.go

set -e

echo "🚀 Setting up HelloJohn V2 (Minimal Configuration)..."

# 1. Create directory structure
echo "📁 Creating data directory structure..."
mkdir -p data/hellojohn/tenants/local
mkdir -p data/hellojohn/keys

# 2. Create default tenant
echo "🏢 Creating default tenant (local)..."
cat > data/hellojohn/tenants/local/tenant.yaml <<EOF
id: "00000000-0000-0000-0000-000000000001"
slug: "local"
name: "Local Development"
language: "en"
created_at: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

settings:
  issuer_mode: "path"
EOF

# 3. Create default client
echo "🔑 Creating default OAuth client..."
cat > data/hellojohn/tenants/local/clients.yaml <<EOF
clients:
  - client_id: "dev-client"
    name: "Development Client"
    type: "public"
    redirect_uris:
      - "http://localhost:3000/callback"
      - "http://localhost:8080/callback"
    default_scopes:
      - "openid"
      - "profile"
      - "email"
    providers:
      - "password"
    created_at: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
EOF

# 4. Create default scopes
echo "🎯 Creating default scopes..."
cat > data/hellojohn/tenants/local/scopes.yaml <<EOF
scopes:
  - name: "openid"
    description: "OpenID Connect scope"
  - name: "profile"
    description: "User profile information"
  - name: "email"
    description: "User email address"
  - name: "offline_access"
    description: "Refresh token access"
EOF

# 5. Generate keys if needed
echo "🔐 Generating master keys..."

# Generate SIGNING_MASTER_KEY (64 hex chars = 32 bytes)
SIGNING_KEY=$(openssl rand -hex 32)

# Generate SECRETBOX_MASTER_KEY (32 bytes base64)
SECRETBOX_KEY=$(openssl rand -base64 32)

# 6. Create .env file
echo "📝 Creating .env file..."
cat > .env.v2 <<EOF
# HelloJohn V2 Environment Configuration
# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

# REQUIRED: JWT Signing Master Key (64 hex chars = 32 bytes)
SIGNING_MASTER_KEY=$SIGNING_KEY

# REQUIRED: Secrets Encryption Key (32 bytes base64)
SECRETBOX_MASTER_KEY=$SECRETBOX_KEY

# REQUIRED: FileSystem Root for Control Plane
FS_ROOT=./data/hellojohn

# Server Configuration
V2_SERVER_ADDR=:8082
V2_BASE_URL=http://localhost:8082

# Auth Configuration
REGISTER_AUTO_LOGIN=true
FS_ADMIN_ENABLE=false

# Social Configuration
SOCIAL_DEBUG_PEEK=false

# Optional: Database for tenant 'local' (uncomment to enable)
# LOCAL_DB_DRIVER=postgres
# LOCAL_DB_DSN=postgres://user:pass@localhost:5432/hellojohn_local?sslmode=disable
EOF

echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Load environment variables:"
echo "   source .env.v2"
echo ""
echo "2. Run V2 server:"
echo "   go run cmd/service_v2/main.go"
echo ""
echo "3. Test health endpoint:"
echo "   curl http://localhost:8082/readyz"
echo ""
echo "4. Test OIDC discovery:"
echo "   curl http://localhost:8082/.well-known/openid-configuration"
echo ""
echo "🔒 IMPORTANT: .env.v2 contains sensitive keys. Add to .gitignore!"
echo ""
