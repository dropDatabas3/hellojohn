package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	// 1) Envs mínimos (SIGNING_MASTER_KEY >= 32 bytes = 64 hex chars)
	if len(os.Getenv("SIGNING_MASTER_KEY")) < 64 {
		_ = os.Setenv("SIGNING_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	}

	// Ensure seed includes the custom scope we test against
	if os.Getenv("SEED_SCOPES") == "" {
		_ = os.Setenv("SEED_SCOPES", "openid,email,profile,offline_access,profile:read")
	}

	// 1.5) Forzar E2E a Postgres (Fase 3) - SIEMPRE en E2E
	_ = os.Setenv("STORAGE_DRIVER", "postgres")
	if os.Getenv("STORAGE_DSN") == "" {
		// Prefer DATABASE_URL if present
		if v := os.Getenv("DATABASE_URL"); strings.TrimSpace(v) != "" {
			_ = os.Setenv("STORAGE_DSN", v)
		} else {
			// Fallback to configs/config.yaml storage.dsn so the spawned service has a DSN
			if root, err := findRepoRoot(); err == nil {
				cfgPath := filepath.Join(root, "configs", "config.yaml")
				if b, err := os.ReadFile(cfgPath); err == nil && len(b) > 0 {
					// naive parse: look for line starting with "  dsn:" under storage
					// For robustness, try importing the config package if available
					type conf struct {
						Storage struct {
							DSN string `yaml:"dsn"`
						} `yaml:"storage"`
					}
					var c conf
					if err := yaml.Unmarshal(b, &c); err == nil && strings.TrimSpace(c.Storage.DSN) != "" {
						_ = os.Setenv("STORAGE_DSN", c.Storage.DSN)
					}
				}
			}
		}
	}
	_ = os.Setenv("FLAGS_MIGRATE", "false") // Saltar migraciones en E2E (deben ejecutarse antes)
	// Configurar límites de conexión muy conservadores para E2E
	_ = os.Setenv("POSTGRES_MAX_OPEN_CONNS", "3")
	_ = os.Setenv("POSTGRES_MAX_IDLE_CONNS", "1")
	// apuntar al folder nuevo "squashed"
	_ = os.Setenv("MIGRATIONS_DIR", "migrations/postgres")

	// 2) Migrar
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/migrate"); err != nil {
			panic(err)
		}
	}

	// 2.5) Generar llaves JWT
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/keys", "-rotate"); err != nil {
			panic(err)
		}
	}

	// 3) Seed
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/seed"); err != nil {
			panic(err)
		}
		var err error
		seed, err = mustLoadSeedYAML()
		if err != nil {
			panic(err)
		}
	}

	// 4) Forzar uso de .env (o archivo indicado por E2E_ENV_FILE). Ignoramos .env.dev para evitar overrides en E2E.
	envFile := os.Getenv("E2E_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if root, err := findRepoRoot(); err == nil {
		candidate := filepath.Join(root, envFile)
		if _, err := os.Stat(candidate); err == nil {
			if err := godotenv.Load(candidate); err != nil {
				panic(err)
			}
			// Recalcular baseURL según variables cargadas
			if issuer := os.Getenv("JWT_ISSUER"); issuer != "" {
				baseURL = issuer
			} else if e := os.Getenv("EMAIL_BASE_URL"); e != "" {
				baseURL = e
			}
		} else {
			// Si no existe el .env requerido, fallamos temprano para que sea visible.
			panic("env file requerido no encontrado: " + candidate)
		}
	}

	// 4.5) Credenciales básicas para /oauth2/introspect (si no vienen dadas)
	if os.Getenv("INTROSPECT_BASIC_USER") == "" {
		_ = os.Setenv("INTROSPECT_BASIC_USER", "introspect-user")
	}
	if os.Getenv("INTROSPECT_BASIC_PASS") == "" {
		_ = os.Setenv("INTROSPECT_BASIC_PASS", "introspect-pass")
	}

	// 4.6) Password blacklist path (ensure we test rejection)
	if os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH") == "" {
		if root, err := findRepoRoot(); err == nil {
			candidate := filepath.Join(root, "configs", "password_blacklist.txt")
			if _, err := os.Stat(candidate); err == nil {
				_ = os.Setenv("SECURITY_PASSWORD_BLACKLIST_PATH", candidate)
			}
		}
	}

	// 4.7) Enable social debug headers so social login_code test can simulate Google callback without real provider.
	if os.Getenv("SOCIAL_DEBUG_HEADERS") == "" {
		_ = os.Setenv("SOCIAL_DEBUG_HEADERS", "true")
	}

	// 4.8) Deshabilitar Google temporalmente para reducir conexiones DB en testing
	_ = os.Setenv("GOOGLE_ENABLED", "false")
	// Provide safe dummy values (they won't be used because debug shortcut short-circuits real exchange).
	if os.Getenv("GOOGLE_CLIENT_ID") == "" {
		_ = os.Setenv("GOOGLE_CLIENT_ID", "dummy-google-client-id.apps.googleusercontent.com")
	}
	if os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		_ = os.Setenv("GOOGLE_CLIENT_SECRET", "dummy-google-secret")
	}

	var err error
	var startedBase string
	srv, startedBase, err = startServer(context.Background(), envFile)
	if err != nil {
		panic(err)
	}
	// Override baseURL to the actual started server to avoid hitting any stale external instance
	if startedBase != "" {
		baseURL = startedBase
	}
	if err := waitReady(baseURL, 20*time.Second); err != nil {
		if srv != nil && srv.out != nil {
			println(srv.out.String())
		}
		panic(err)
	}

	// Ensure the web client has dynamic redirect URIs allowed (social result and localhost:3000)
	// Acquire admin access token first
	{
		c := newHTTPClient()
		// login admin
		body := map[string]string{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		}
		bb, _ := json.Marshal(body)
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bb))
		if err == nil && resp != nil {
			var tok struct {
				AccessToken string `json:"access_token"`
			}
			if resp.StatusCode == 200 {
				_ = json.NewDecoder(resp.Body).Decode(&tok)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if tok.AccessToken != "" {
				// Seed FS control-plane for tenant/local and client/local-web via Admin FS
				// 1) Upsert tenant with DSN plain (provider encrypts)
				{
					payload := map[string]any{
						"id":   seed.Tenant.ID,
						"slug": "local",
						"name": "Local",
						"settings": map[string]any{
							"userDB": map[string]any{
								"type": "postgres",
								"dsn":  os.Getenv("STORAGE_DSN"),
							},
						},
					}
					b, _ := json.Marshal(payload)
					req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/local", bytes.NewReader(b))
					req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
					req.Header.Set("Content-Type", "application/json")
					if r1, e1 := c.Do(req); e1 == nil && r1 != nil {
						io.Copy(io.Discard, r1.Body)
						r1.Body.Close()
					}
				}

				// 2) Upsert client local-web enabling password and default scopes
				{
					// Use camelCase keys expected by the Admin FS handler (controlplane.ClientInput)
					payload := map[string]any{
						"clientId":       seed.Clients.Web.ClientID,
						"name":           "Web Frontend",
						"type":           "public",
						"redirectUris":   []string{baseURL + "/v1/auth/social/result", "http://localhost:7777/callback"},
						"allowedOrigins": []string{"http://localhost:7777"},
						"providers":      []string{"password"},
						"scopes":         []string{"openid", "profile", "email", "offline_access"},
						"enabled":        true, // ignored by server; kept for compatibility with sample
					}
					b, _ := json.Marshal(payload)
					req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/local/clients/"+seed.Clients.Web.ClientID, bytes.NewReader(b))
					req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
					req.Header.Set("Content-Type", "application/json")
					if r1, e1 := c.Do(req); e1 == nil && r1 != nil {
						io.Copy(io.Discard, r1.Body)
						r1.Body.Close()
					}
				}

				// 3) Optional: declare scopes (idempotent)
				{
					payload := map[string]any{"scopes": []map[string]string{{"name": "openid"}, {"name": "profile"}, {"name": "email"}, {"name": "offline_access"}}}
					b, _ := json.Marshal(payload)
					req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/local/scopes", bytes.NewReader(b))
					req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
					req.Header.Set("Content-Type", "application/json")
					if r1, e1 := c.Do(req); e1 == nil && r1 != nil {
						io.Copy(io.Discard, r1.Body)
						r1.Body.Close()
					}
				}

				// 4) Test-connection and migrate per-tenant
				for _, ep := range []string{"test-connection", "migrate"} {
					req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/local/user-store/"+ep, nil)
					req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
					if r1, e1 := c.Do(req); e1 == nil && r1 != nil {
						io.Copy(io.Discard, r1.Body)
						r1.Body.Close()
					}
				}
				// list clients
				req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/admin/clients?tenant_id="+seed.Tenant.ID, nil)
				req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
				resp2, err2 := c.Do(req)
				if err2 == nil && resp2.StatusCode == 200 {
					var clients []struct {
						ID             string   `json:"id"`
						ClientID       string   `json:"client_id"`
						TenantID       string   `json:"tenant_id"`
						Name           string   `json:"name"`
						ClientType     string   `json:"client_type"`
						RedirectURIs   []string `json:"redirect_uris"`
						AllowedOrigins []string `json:"allowed_origins"`
						Providers      []string `json:"providers"`
						Scopes         []string `json:"scopes"`
					}
					_ = json.NewDecoder(resp2.Body).Decode(&clients)
					io.Copy(io.Discard, resp2.Body)
					resp2.Body.Close()
					// find web client
					for _, cl := range clients {
						if cl.ClientID == seed.Clients.Web.ClientID {
							need := map[string]bool{
								baseURL + "/v1/auth/social/result": true,
								"http://localhost:3000/callback":   true,
							}
							have := map[string]bool{}
							for _, u := range cl.RedirectURIs {
								have[u] = true
							}
							updated := false
							for u := range need {
								if !have[u] {
									cl.RedirectURIs = append(cl.RedirectURIs, u)
									updated = true
								}
							}
							if updated {
								body2, _ := json.Marshal(cl)
								ureq, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/clients/"+cl.ID, bytes.NewReader(body2))
								ureq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
								ureq.Header.Set("Content-Type", "application/json")
								resp3, err3 := c.Do(ureq)
								if err3 == nil && resp3 != nil {
									io.Copy(io.Discard, resp3.Body)
									resp3.Body.Close()
								}
							}
							break
						}
					}
				} else if resp2 != nil {
					io.Copy(io.Discard, resp2.Body)
					resp2.Body.Close()
				}
			}
		}
	}

	code := m.Run()

	if srv != nil {
		// Dump server logs for debugging social debug shortcut
		if srv.out != nil {
			println("--- SERVER LOGS START ---")
			println(srv.out.String())
			println("--- SERVER LOGS END ---")
		}
		srv.stop()
	}
	os.Exit(code)
}
