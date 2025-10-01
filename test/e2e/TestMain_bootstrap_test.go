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

	"github.com/joho/godotenv"
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

	// 4.8) Enable Google provider in tests with dummy credentials so discovery lists it and debug path works.
	if os.Getenv("GOOGLE_ENABLED") == "" {
		_ = os.Setenv("GOOGLE_ENABLED", "true")
	}
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
