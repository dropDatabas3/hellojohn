package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/security/totp"
)

// ---------- helpers env ----------
func csvEnv(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func strEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
func boolEnv(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "t", "true", "y", "yes":
		return true
	case "0", "f", "false", "n", "no":
		return false
	default:
		return def
	}
}

// urlEncode removido: se usa url.QueryEscape directamente

func mfaIssuer() string {
	if s := strings.TrimSpace(os.Getenv("MFA_TOTP_ISSUER")); s != "" {
		return s
	}
	return "HelloJohn"
}

// ---------- AES-GCM (mismo esquema que handlers MFA) ----------
func aesgcmEncryptMFA(plainB32 string) (string, error) {
	// Preferir clave dedicada; fallback a SIGNING_MASTER_KEY por compat retro.
	k := []byte(os.Getenv("MFA_ENC_KEY"))
	if len(k) < 32 {
		k = []byte(os.Getenv("SIGNING_MASTER_KEY"))
	}
	// Requerimos 32 bytes (AES-256) siempre.
	if len(k) < 32 {
		return "", errors.New("MFA_ENC_KEY (o SIGNING_MASTER_KEY) faltante o muy corta (min 32 bytes)")
	}
	block, err := aes.NewCipher(k[:32])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plainB32), nil)
	out := append(nonce, ct...)
	return "GCMV1-MFA:" + hex.EncodeToString(out), nil
}

// ---------- files ----------
func mustWriteFile(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ---------- main ----------
func main() {
	// .env (opcional) - prioridad .env.dev > .env
	_ = godotenv.Load(".env")     // base
	_ = godotenv.Load(".env.dev") // dev overrides

	// config.yaml (si está)
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Printf("config: %v (se intentará usar STORAGE_DSN de env)", err)
	}

	// DSN: env > config
	dsn := strings.TrimSpace(os.Getenv("STORAGE_DSN"))
	if dsn == "" && cfg != nil {
		dsn = cfg.Storage.DSN
	}
	if dsn == "" {
		log.Fatal("no hay DSN (STORAGE_DSN o configs/config.yaml)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// ------------------ Defaults (overrideables por ENV) ------------------
	tenantSlug := strEnv("SEED_TENANT_SLUG", "local")
	tenantName := strEnv("SEED_TENANT_NAME", "Local Tenant")

	adminEmail := strEnv("SEED_ADMIN_EMAIL", "admin@local.test")
	adminPass := strEnv("SEED_ADMIN_PASSWORD", "SuperS3creta!")

	mfaEmail := strEnv("SEED_MFA_EMAIL", "mfa@local.test")
	mfaPass := strEnv("SEED_MFA_PASSWORD", "CorrectHorseBatteryStaple1!")

	unvEmail := strEnv("SEED_UNVERIFIED_EMAIL", "new@local.test")
	unvPass := strEnv("SEED_UNVERIFIED_PASSWORD", "Password.1234")

	clientID := strEnv("SEED_CLIENT_ID", "web-frontend")
	clientName := strEnv("SEED_CLIENT_NAME", "Web Frontend")
	clientType := strEnv("SEED_CLIENT_TYPE", "public") // public|confidential

	backendClientEnabled := boolEnv("SEED_BACKEND_CLIENT_ENABLED", true)
	beClientID := strEnv("SEED_BACKEND_CLIENT_ID", "backend-api")
	beClientName := strEnv("SEED_BACKEND_CLIENT_NAME", "Backend API")
	beClientType := strEnv("SEED_BACKEND_CLIENT_TYPE", "confidential")

	allowedOrigins := csvEnv("SEED_ALLOWED_ORIGINS",
		[]string{"http://localhost:3000", "http://127.0.0.1:3000"},
	)
	redirectURIs := csvEnv("SEED_REDIRECT_URIS",
		[]string{
			"http://localhost:3000/callback",
		},
	)
	providers := csvEnv("SEED_PROVIDERS",
		[]string{"password", "google"},
	)
	scopes := csvEnv("SEED_SCOPES",
		[]string{"openid", "email", "profile", "offline_access"},
	)

	// emailBase: prioridad ENV > config.yaml > default
	emailBase := strEnv("EMAIL_BASE_URL", "")
	if emailBase == "" && cfg != nil && strings.TrimSpace(cfg.Email.BaseURL) != "" {
		emailBase = strings.TrimRight(cfg.Email.BaseURL, "/")
	}
	if emailBase == "" {
		emailBase = strEnv("JWT_ISSUER", "http://localhost:8080")
	}
	emailBase = strings.TrimRight(emailBase, "/")

	// Social result URL usa la misma base
	socialResultURL := emailBase + "/v1/auth/social/result"
	redirectURIs = append(redirectURIs, socialResultURL)
	// ---------------------------------------------------------------------

	// 1) Tenant (upsert por slug)
	var tenantID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO tenant (name, slug, settings)
		VALUES ($1, $2, '{}'::jsonb)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, tenantName, tenantSlug).Scan(&tenantID); err != nil {
		log.Fatalf("upsert tenant: %v", err)
	}

	// 2) Users + identities
	type userOut struct {
		ID       string
		Email    string
		Password string
	}
	var admin userOut
	var mfa userOut
	var unv userOut

	insUser := func(email, pwd string, verified bool) (userOut, error) {
		phc, _ := password.Hash(password.Default, pwd)
		// user
		if _, err := pool.Exec(ctx, `
			INSERT INTO app_user (tenant_id, email, email_verified, status, metadata)
			VALUES ($1, LOWER($2), $3, 'active', '{}'::jsonb)
			ON CONFLICT (tenant_id, email) DO NOTHING
		`, tenantID, email, verified); err != nil {
			return userOut{}, fmt.Errorf("insert user %s: %w", email, err)
		}
		// identity: upsert y ACTUALIZAR password_hash para que sea idempotente entre corridas
		if _, err := pool.Exec(ctx, `
			INSERT INTO identity (user_id, provider, email, email_verified, password_hash)
			SELECT id, 'password', LOWER($1), $2, $3
			FROM app_user WHERE tenant_id = $4 AND email = LOWER($1)
			ON CONFLICT (user_id, provider)
			DO UPDATE SET email = EXCLUDED.email,
						  email_verified = EXCLUDED.email_verified,
						  password_hash = EXCLUDED.password_hash
		`, email, verified, phc, tenantID); err != nil {
			return userOut{}, fmt.Errorf("insert identity %s: %w", email, err)
		}
		// id back
		var id string
		if err := pool.QueryRow(ctx, `
			SELECT id FROM app_user WHERE tenant_id = $1 AND email = LOWER($2) LIMIT 1
		`, tenantID, email).Scan(&id); err != nil {
			return userOut{}, fmt.Errorf("select user id %s: %w", email, err)
		}
		return userOut{ID: id, Email: email, Password: pwd}, nil
	}

	var e error
	admin, e = insUser(adminEmail, adminPass, true)
	if e != nil {
		log.Fatal(e)
	}
	mfa, e = insUser(mfaEmail, mfaPass, true)
	if e != nil {
		log.Fatal(e)
	}
	unv, e = insUser(unvEmail, unvPass, false)
	if e != nil {
		log.Fatal(e)
	}

	// 3) Clients (frontend y opcional backend)
	upsertClient := func(name, cid, ctype string) (string, error) {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO client (
				tenant_id, name, client_id, client_type,
				redirect_uris, allowed_origins, providers, scopes
			)
			VALUES ($1, $2, $3, $4, $5::text[], $6::text[], $7::text[], $8::text[])
			ON CONFLICT (client_id) DO UPDATE SET
				tenant_id = EXCLUDED.tenant_id,
				name      = EXCLUDED.name,
				client_type = EXCLUDED.client_type,
				redirect_uris = ARRAY(SELECT DISTINCT x FROM unnest(client.redirect_uris || EXCLUDED.redirect_uris) AS x),
				allowed_origins = ARRAY(SELECT DISTINCT x FROM unnest(client.allowed_origins || EXCLUDED.allowed_origins) AS x),
				providers = ARRAY(SELECT DISTINCT x FROM unnest(client.providers || EXCLUDED.providers) AS x),
				scopes = ARRAY(SELECT DISTINCT x FROM unnest(client.scopes || EXCLUDED.scopes) AS x)
			RETURNING id
		`, tenantID, name, cid, ctype, redirectURIs, allowedOrigins, providers, scopes).Scan(&id)
		return id, err
	}
	clientUUID, err := upsertClient(clientName, clientID, clientType)
	if err != nil {
		log.Fatalf("upsert client: %v", err)
	}

	var beUUID string
	if backendClientEnabled {
		beUUID, err = upsertClient(beClientName, beClientID, beClientType)
		if err != nil {
			log.Fatalf("upsert backend client: %v", err)
		}
	}

	// 4) client_version activa si falta
	ensureActiveVersion := func(clientDBID string) {
		var hasActive bool
		if err := pool.QueryRow(ctx, `SELECT active_version_id IS NOT NULL FROM client WHERE id = $1`, clientDBID).Scan(&hasActive); err != nil {
			log.Fatalf("check active version: %v", err)
		}
		if !hasActive {
			var cvID string
			if err := pool.QueryRow(ctx, `
				INSERT INTO client_version (client_id, version, claim_schema_json, claim_mapping_json, crypto_config_json, status)
				VALUES ($1, 'v1', '{}'::jsonb, '{}'::jsonb, '{"alg":"EdDSA"}'::jsonb, 'active')
				RETURNING id
			`, clientDBID).Scan(&cvID); err != nil {
				log.Fatalf("insert client_version: %v", err)
			}
			if _, err := pool.Exec(ctx, `UPDATE client SET active_version_id = $2 WHERE id = $1`, clientDBID, cvID); err != nil {
				log.Fatalf("activate client_version: %v", err)
			}
		}
	}
	ensureActiveVersion(clientUUID)
	if beUUID != "" {
		ensureActiveVersion(beUUID)
	}

	// 5) MFA para user mfa@
	//    - secreto aleatorio base32
	//    - cifrado AES-GCM (SIGNING_MASTER_KEY)
	//    - confirmed_at = now()
	//    - recovery codes (hash SHA256 base64url)
	_, b32, err := totp.GenerateSecret()
	if err != nil {
		log.Fatalf("totp secret: %v", err)
	}
	enc, err := aesgcmEncryptMFA(b32)
	if err != nil {
		log.Fatalf("mfa encrypt: %v", err)
	}
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_mfa_totp (user_id, secret_encrypted, confirmed_at, last_used_at)
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (user_id) DO UPDATE SET
		  secret_encrypted = EXCLUDED.secret_encrypted,
		  confirmed_at = EXCLUDED.confirmed_at,
		  last_used_at = NULL
	`, mfa.ID, enc, now); err != nil {
		log.Fatalf("upsert user_mfa_totp: %v", err)
	}

	// Recovery codes
	var recovPlain []string
	if true {
		for i := 0; i < 8; i++ {
			rc, _ := tokens.GenerateOpaqueToken(12) // ~16 chars
			recovPlain = append(recovPlain, rc)
			h := tokens.SHA256Base64URL(rc)
			if _, err := pool.Exec(ctx, `
				INSERT INTO mfa_recovery_code (user_id, code_hash, used_at)
				VALUES ($1, $2, NULL)
				ON CONFLICT DO NOTHING
			`, mfa.ID, h); err != nil {
				log.Fatalf("insert recovery code: %v", err)
			}
		}
	}

	// 6) Email flows seeds:
	//    - verify email token (para user unverified)
	//    - reset password token (para admin)
	makeToken := func() (raw string, sha []byte) {
		t, _ := tokens.GenerateOpaqueToken(32)
		sum := sha256.Sum256([]byte(t))
		return t, sum[:]
	}
	verTok, verHash := makeToken()
	expVer := now.Add(48 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO email_verification_token (tenant_id, user_id, token_hash, sent_to, ip, user_agent, created_at, expires_at, used_at)
		VALUES ($1,$2,$3,$4,NULL,NULL,now(),$5,NULL)
	`, tenantID, unv.ID, verHash, unv.Email, expVer); err != nil {
		log.Fatalf("insert email_verification_token: %v", err)
	}
	resetTok, resetHash := makeToken()
	expReset := now.Add(1 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO password_reset_token (tenant_id, user_id, token_hash, sent_to, ip, user_agent, created_at, expires_at, used_at)
		VALUES ($1,$2,$3,$4,NULL,NULL,now(),$5,NULL)
	`, tenantID, admin.ID, resetHash, admin.Email, expReset); err != nil {
		log.Fatalf("insert password_reset_token: %v", err)
	}

	// Trusted device (recuerda MFA) para usuario mfa@
	devToken, _ := tokens.GenerateOpaqueToken(32)
	devHash := tokens.SHA256Base64URL(devToken)
	if _, err := pool.Exec(ctx, `
		INSERT INTO trusted_device (tenant_id, user_id, device_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, device_hash) DO UPDATE SET expires_at = EXCLUDED.expires_at
	`, tenantID, mfa.ID, devHash, now.Add(30*24*time.Hour)); err != nil {
		log.Fatalf("insert trusted_device: %v", err)
	}

	// OTPAuth URL para fácil escaneo
	otpURL := totp.OTPAuthURL(mfaIssuer(), mfa.Email, b32)

	// ───────────────────────── RBAC (seed) ─────────────────────────
	rbacRoles := map[string][]string{
		"sys:admin": {"rbac:read", "rbac:write", "clients:read", "clients:write", "scopes:read", "scopes:write", "consents:read", "consents:write"},
		"admin":     {"rbac:read", "rbac:write"},
		"viewer":    {"rbac:read"},
	}
	roleDesc := map[string]string{
		"sys:admin": "System administrator",
		"admin":     "Tenant administrator",
		"viewer":    "Read only",
	}
	permDesc := map[string]string{
		"rbac:read":      "Read RBAC resources",
		"rbac:write":     "Write RBAC resources",
		"clients:read":   "Read clients",
		"clients:write":  "Write clients",
		"scopes:read":    "Read scopes",
		"scopes:write":   "Write scopes",
		"consents:read":  "Read consents",
		"consents:write": "Write consents",
	}

	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	upsertRole := func(role, desc string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO rbac_role (tenant_id, role, description)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, role) DO UPDATE
			  SET description = EXCLUDED.description
		`, tenantID, role, desc)
		return err
	}
	upsertPerm := func(perm, desc string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO rbac_perm (tenant_id, perm, description)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, perm) DO UPDATE
			  SET description = EXCLUDED.description
		`, tenantID, perm, desc)
		return err
	}
	linkRolePerm := func(role, perm string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO rbac_role_perm (tenant_id, role, perm)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, role, perm) DO NOTHING
		`, tenantID, role, perm)
		return err
	}

	var roleList []string
	permSet := map[string]struct{}{}
	for r, perms := range rbacRoles {
		nr := norm(r)
		roleList = append(roleList, nr)
		for _, p := range perms {
			permSet[norm(p)] = struct{}{}
		}
	}

	for _, r := range roleList {
		if err := upsertRole(r, roleDesc[r]); err != nil {
			log.Fatalf("rbac_role upsert (%s): %v", r, err)
		}
	}
	for p := range permSet {
		if err := upsertPerm(p, permDesc[p]); err != nil {
			log.Fatalf("rbac_perm upsert (%s): %v", p, err)
		}
	}
	for r, perms := range rbacRoles {
		nr := norm(r)
		for _, p := range perms {
			np := norm(p)
			if err := linkRolePerm(nr, np); err != nil {
				log.Fatalf("rbac_role_perm insert (%s/%s): %v", nr, np, err)
			}
		}
	}

	assign := map[string][]string{
		admin.ID: {"sys:admin", "admin"},
		mfa.ID:   {"viewer"},
	}
	for userID, roles := range assign {
		for _, role := range roles {
			nr := norm(role)
			_, err := pool.Exec(ctx, `
				INSERT INTO rbac_user_role (tenant_id, user_id, role)
				VALUES ($1, $2, $3)
				ON CONFLICT (tenant_id, user_id, role) DO NOTHING
			`, tenantID, userID, nr)
			if err != nil {
				log.Fatalf("rbac_user_role insert (user=%s, role=%s): %v", userID, nr, err)
			}
		}
	}

	getUserRoles := func(userID string) []string {
		rows, err := pool.Query(ctx, `
			SELECT role FROM rbac_user_role
			WHERE user_id = $1
			ORDER BY role
		`, userID)
		if err != nil {
			return nil
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var r string
			_ = rows.Scan(&r)
			out = append(out, r)
		}
		return out
	}
	adminRolesNow := getUserRoles(admin.ID)
	mfaRolesNow := getUserRoles(mfa.ID)
	unvRolesNow := getUserRoles(unv.ID)
	// ─────────────────────── fin RBAC (seed) ───────────────────────

	// 7) Output (consola + archivos)
	verifyURL := fmt.Sprintf("%s/v1/auth/verify-email?token=%s", emailBase, url.QueryEscape(verTok))
	resetURL := fmt.Sprintf("%s/reset?token=%s", emailBase, url.QueryEscape(resetTok))

	fmt.Println()
	fmt.Println("Seed listo ✅")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Tenant:     %s (slug=%s)\n", tenantID, tenantSlug)
	fmt.Printf("Client web: %s (id=%s, type=%s)\n", clientName, clientID, clientType)
	if beUUID != "" {
		fmt.Printf("Client API: %s (id=%s, type=%s)\n", beClientName, beClientID, beClientType)
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Admin:      %s / %s\n", admin.Email, admin.Password)
	fmt.Printf("Admin SUB:  %s\n", admin.ID)
	fmt.Printf("MFA user:   %s / %s\n", mfa.Email, mfa.Password)
	fmt.Printf("Unverified: %s / %s\n", unv.Email, unv.Password)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("MFA secret (base32): %s\n", b32)
	fmt.Printf("Recovery codes (%d): %v\n", len(recovPlain), recovPlain)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Verify link (unverified): %s\n", verifyURL)
	fmt.Printf("Reset  link (admin):      %s\n", resetURL)
	fmt.Println("--------------------------------------------------")
	// RBAC print extra
	fmt.Println("RBAC:")
	fmt.Printf("- Roles por usuario:\n")
	fmt.Printf("  - Admin     : %v\n", adminRolesNow)
	fmt.Printf("  - MFA       : %v\n", mfaRolesNow)
	fmt.Printf("  - Unverified: %v\n", unvRolesNow)
	fmt.Printf("- Permisos por rol:\n")
	fmt.Printf("  - sys:admin : %v\n", rbacRoles["sys:admin"])
	fmt.Printf("  - admin     : %v\n", rbacRoles["admin"])
	fmt.Printf("  - viewer    : %v\n", rbacRoles["viewer"])
	fmt.Println("Sugerencia ENV para admin endpoints protegidos:")
	fmt.Printf("  ADMIN_ENFORCE=1  ADMIN_SUBS=%s\n", admin.ID)
	fmt.Println("--------------------------------------------------")

	md := fmt.Sprintf(
		"# Seed Data\n\n"+
			"**Generated:** %s\n\n"+
			"## Tenant\n"+
			"- id: `%s`\n"+
			"- slug: `%s`\n\n"+
			"## Clients\n"+
			"- Web: **%s**\n"+
			"  - client_id: `%s`\n"+
			"  - type: `%s`\n"+
			"- Backend: **%s**\n"+
			"  - client_id: `%s`\n"+
			"  - type: `%s`\n\n"+
			"## Users\n"+
			"- **Admin**: `%s` / `%s`\n"+
			"  - sub: `%s`\n"+
			"- **MFA**  : `%s` / `%s`\n"+
			"  - TOTP secret (base32): `%s`\n"+
			"  - OTPAuth URL: %s\n"+
			"  - Recovery codes: %v\n"+
			"  - Trusted device token: `%s`\n"+
			"- **Unverified**: `%s` / `%s`\n\n"+
			"## Email Flows\n"+
			"- Verify URL (unverified): %s\n"+
			"- Reset  URL (admin):      %s\n",
		time.Now().Format(time.RFC3339),
		tenantID, tenantSlug,
		clientName, clientID, clientType,
		beClientName, beClientID, beClientType,
		admin.Email, admin.Password, admin.ID,
		mfa.Email, mfa.Password, b32, otpURL, recovPlain, devToken,
		unv.Email, unv.Password,
		verifyURL, resetURL,
	)

	// Append RBAC section to MD
	mdRBAC := fmt.Sprintf(
		"\n## RBAC\n"+
			"### Roles por usuario\n"+
			"- Admin: `%s`\n"+
			"- MFA: `%s`\n"+
			"- Unverified: `%s`\n\n"+
			"### Permisos por rol\n"+
			"- sys:admin: %v\n"+
			"- admin: %v\n"+
			"- viewer: %v\n\n"+
			"### ENV sugerido (admin endpoints)\n"+
			"- `ADMIN_ENFORCE=1`\n"+
			"- `ADMIN_SUBS=%s`\n",
		strings.Join(adminRolesNow, ", "),
		strings.Join(mfaRolesNow, ", "),
		strings.Join(unvRolesNow, ", "),
		rbacRoles["sys:admin"],
		rbacRoles["admin"],
		rbacRoles["viewer"],
		admin.ID,
	)
	md += mdRBAC

	yaml := fmt.Sprintf(`generated: %q
tenant:
  id: %q
  slug: %q
clients:
  web:
    name: %q
    client_id: %q
    type: %q
  backend:
    name: %q
    client_id: %q
    type: %q
users:
  admin:
    email: %q
    password: %q
    sub: %q
  mfa:
    email: %q
    password: %q
    totp_secret_base32: %q
    otpauth_url: %q
    recovery_codes: [%s]
    trusted_device_token: %q
  unverified:
    email: %q
    password: %q
email_flows:
  verify_url: %q
  reset_url: %q
`,
		time.Now().Format(time.RFC3339),
		tenantID, tenantSlug,
		clientName, clientID, clientType,
		beClientName, beClientID, beClientType,
		admin.Email, admin.Password, admin.ID,
		mfa.Email, mfa.Password, b32, otpURL, strings.Join(quoteAll(recovPlain), ", "), devToken,
		unv.Email, unv.Password,
		verifyURL, resetURL,
	)

	// Append RBAC section to YAML (nueva sección)
	yamlRBAC := fmt.Sprintf(`rbac:
  roles:
    "sys:admin": [%s]
    "admin": [%s]
    "viewer": [%s]
  assignments:
    admin: [%s]
    mfa: [%s]
    unverified: [%s]
admin_env:
  ADMIN_ENFORCE: "1"
  ADMIN_SUBS: %q
`,
		strings.Join(quoteAll(rbacRoles["sys:admin"]), ", "),
		strings.Join(quoteAll(rbacRoles["admin"]), ", "),
		strings.Join(quoteAll(rbacRoles["viewer"]), ", "),
		strings.Join(quoteAll(adminRolesNow), ", "),
		strings.Join(quoteAll(mfaRolesNow), ", "),
		strings.Join(quoteAll(unvRolesNow), ", "),
		admin.ID,
	)
	yaml = yaml + yamlRBAC

	// Detectar si los archivos ya existen
	mdPath := "cmd/seed/seed_data.md"
	yamlPath := "cmd/seed/seed_data.yaml"

	mdExists := fileExists(mdPath)
	yamlExists := fileExists(yamlPath)

	mustWriteFile(mdPath, md)
	mustWriteFile(yamlPath, yaml)

	if mdExists || yamlExists {
		fmt.Println("Archivos actualizados:")
	} else {
		fmt.Println("Archivos generados:")
	}

	if mdExists {
		fmt.Println(" - cmd/seed/seed_data.md (actualizado)")
	} else {
		fmt.Println(" - cmd/seed/seed_data.md (creado)")
	}

	if yamlExists {
		fmt.Println(" - cmd/seed/seed_data.yaml (actualizado)")
	} else {
		fmt.Println(" - cmd/seed/seed_data.yaml (creado)")
	}
}

func quoteAll(v []string) []string {
	out := make([]string, len(v))
	for i, s := range v {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
