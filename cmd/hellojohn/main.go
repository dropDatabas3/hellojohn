package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type client struct {
	BaseURL   string
	APIKey    string // deprecated: prefer Bearer
	Bearer    string
	OutFormat string // "json" | "text"
	HTTP      *http.Client

	// Optional login credentials to acquire a Bearer token automatically
	TenantID string
	ClientID string
	Email    string
	Password string
}

func (c *client) do(method, path string, body []byte, headers map[string]string) (int, []byte, error) {
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	// Prefer Bearer if present; keep legacy header for backward compatibility with older servers
	if strings.TrimSpace(c.Bearer) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Bearer))
	} else if strings.TrimSpace(c.APIKey) != "" {
		req.Header.Set("X-Admin-API-Key", c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func (c *client) print(status int, body []byte) {
	if c.OutFormat == "json" {
		var v any
		if json.Unmarshal(body, &v) == nil {
			p, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(p))
			return
		}
	}
	if len(body) > 0 {
		fmt.Println(string(body))
	} else {
		fmt.Printf("status=%d\n", status)
	}
}

// ensureToken logs in using credentials when Bearer is empty and creds are provided.
func (c *client) ensureToken() error {
	if strings.TrimSpace(c.Bearer) != "" {
		return nil
	}
	// Require minimal credentials
	if strings.TrimSpace(c.Email) == "" || strings.TrimSpace(c.Password) == "" {
		return fmt.Errorf("missing credentials: set --email/--password or HELLOJOHN_EMAIL/HELLOJOHN_PASSWORD or provide --bearer")
	}
	tenant := strings.TrimSpace(c.TenantID)
	if tenant == "" {
		tenant = "local"
	}
	clientID := strings.TrimSpace(c.ClientID)
	if clientID == "" {
		clientID = "local-web"
	}
	payload := map[string]string{
		"tenant_id": tenant,
		"client_id": clientID,
		"email":     c.Email,
		"password":  c.Password,
	}
	body, _ := json.Marshal(payload)
	status, resp, err := c.do("POST", "/v1/auth/login", body, nil)
	if err != nil {
		return err
	}
	if status/100 != 2 {
		return fmt.Errorf("login failed: status=%d body=%s", status, string(resp))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(resp, &tok)
	if strings.TrimSpace(tok.AccessToken) == "" {
		return fmt.Errorf("login did not return access_token")
	}
	c.Bearer = tok.AccessToken
	return nil
}

func main() {
	var (
		baseURL = envOr("HELLOJOHN_ADMIN_URL", "http://localhost:8080")
		apiKey  = envOr("HELLOJOHN_ADMIN_KEY", "") // deprecated
		bearer  = envOr("HELLOJOHN_BEARER", "")
		out     = envOr("HELLOJOHN_OUT", "text")
		timeout = 30 * time.Second
	)

	root := &cobra.Command{
		Use:   "hellojohn",
		Short: "CLI admin para HelloJohn (solo /v1/admin)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// If neither Bearer nor API key is provided, we will attempt interactive/login based on flags/envs when needed.
			return nil
		},
	}

	root.PersistentFlags().StringVar(&baseURL, "admin-api-url", baseURL, "URL base del Admin API (env HELLOJOHN_ADMIN_URL)")
	root.PersistentFlags().StringVar(&apiKey, "admin-api-key", apiKey, "[Deprecated] API key del Admin API (env HELLOJOHN_ADMIN_KEY)")
	root.PersistentFlags().StringVar(&bearer, "bearer", bearer, "Bearer token (env HELLOJOHN_BEARER); si falta, intenta login con credenciales")
	root.PersistentFlags().StringVar(&out, "out", out, "Formato de salida: json|text")

	httpClient := &http.Client{Timeout: timeout}
	// Optional login credentials
	tenantID := envOr("HELLOJOHN_TENANT_ID", "")
	clientID := envOr("HELLOJOHN_CLIENT_ID", "")
	email := envOr("HELLOJOHN_EMAIL", "")
	password := envOr("HELLOJOHN_PASSWORD", "")

	root.PersistentFlags().StringVar(&tenantID, "tenant", tenantID, "Tenant ID/slug para login (env HELLOJOHN_TENANT_ID; default local)")
	root.PersistentFlags().StringVar(&clientID, "client-id", clientID, "Client ID para login (env HELLOJOHN_CLIENT_ID; default local-web)")
	root.PersistentFlags().StringVar(&email, "email", email, "Email admin para login (env HELLOJOHN_EMAIL)")
	root.PersistentFlags().StringVar(&password, "password", password, "Password para login (env HELLOJOHN_PASSWORD)")

	cl := &client{BaseURL: baseURL, APIKey: apiKey, Bearer: bearer, OutFormat: out, HTTP: httpClient, TenantID: tenantID, ClientID: clientID, Email: email, Password: password}

	// grupo admin
	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Operaciones administrativas (vía /v1/admin)",
	}

	// ping: usa GET /v1/admin/clients con limit=1
	pingCmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping al Admin API (requiere Bearer o API key legacy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure we have a token if none provided
			if strings.TrimSpace(cl.Bearer) == "" && strings.TrimSpace(cl.APIKey) == "" {
				if err := cl.ensureToken(); err != nil {
					return err
				}
			}
			status, body, err := cl.do("GET", "/v1/admin/clients?limit=1", nil, nil)
			if err != nil {
				return err
			}
			if status/100 != 2 {
				return fmt.Errorf("ping fallo: status=%d body=%s", status, string(body))
			}
			if cl.OutFormat == "text" {
				fmt.Println("ok")
				return nil
			}
			cl.print(status, []byte(`{"ok":true}`))
			return nil
		},
	}

	// tenants set-issuer-mode
	var setSlug, setMode, setOverride string
	setIssuerCmd := &cobra.Command{
		Use:   "set-issuer-mode",
		Short: "Setear issuerMode de un tenant (global|path)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setSlug == "" {
				return fmt.Errorf("--slug es requerido")
			}
			if setMode == "" {
				return fmt.Errorf("--mode es requerido (global|path)")
			}
			payload := map[string]any{
				"slug": setSlug,
				"settings": map[string]any{
					"issuerMode": setMode,
				},
			}
			if setOverride != "" {
				payload["settings"].(map[string]any)["issuerOverride"] = setOverride
			}
			b, _ := json.Marshal(payload)
			h := map[string]string{"If-Match": "*"}
			status, body, err := cl.do("PUT", "/v1/admin/tenants/"+setSlug, b, h)
			if err != nil {
				return err
			}
			if status/100 != 2 {
				return fmt.Errorf("set-issuer-mode fallo: status=%d body=%s", status, string(body))
			}
			cl.print(status, body)
			return nil
		},
	}
	setIssuerCmd.Flags().StringVar(&setSlug, "slug", "", "Slug del tenant (ej. acme)")
	setIssuerCmd.Flags().StringVar(&setMode, "mode", "", "Modo de issuer: global|path")
	setIssuerCmd.Flags().StringVar(&setOverride, "override", "", "Issuer override (opcional, futuro domain)")

	// tenants rotate-keys
	var rotSlug string
	var rotGrace int
	rotateKeysCmd := &cobra.Command{
		Use:   "rotate-keys",
		Short: "Rotar claves de firma del tenant (mantiene ventana de gracia)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(cl.Bearer) == "" && strings.TrimSpace(cl.APIKey) == "" {
				if err := cl.ensureToken(); err != nil {
					return err
				}
			}
			if rotSlug == "" {
				return fmt.Errorf("--slug es requerido")
			}
			path := "/v1/admin/tenants/" + rotSlug + "/keys/rotate"
			if rotGrace >= 0 {
				path = fmt.Sprintf("%s?graceSeconds=%d", path, rotGrace)
			}
			status, body, err := cl.do("POST", path, nil, nil)
			if err != nil {
				return err
			}
			if status/100 != 2 {
				return fmt.Errorf("rotate-keys fallo: status=%d body=%s", status, string(body))
			}
			cl.print(status, body)
			return nil
		},
	}
	rotateKeysCmd.Flags().StringVar(&rotSlug, "slug", "", "Slug del tenant (ej. acme)")
	rotateKeysCmd.Flags().IntVar(&rotGrace, "grace-seconds", -1, "Ventana de gracia en segundos (opcional)")

	// wiring
	adminTenantsCmd := &cobra.Command{Use: "tenants", Short: "Operaciones sobre tenants"}
	adminTenantsCmd.AddCommand(setIssuerCmd)
	adminTenantsCmd.AddCommand(rotateKeysCmd)

	adminCmd.AddCommand(pingCmd)
	adminCmd.AddCommand(adminTenantsCmd)
	root.AddCommand(adminCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
