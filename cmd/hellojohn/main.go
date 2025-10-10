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
	APIKey    string
	OutFormat string // "json" | "text"
	HTTP      *http.Client
}

func (c *client) do(method, path string, body []byte, headers map[string]string) (int, []byte, error) {
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Admin-API-Key", c.APIKey)
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

func main() {
	var (
		baseURL = envOr("HELLOJOHN_ADMIN_URL", "http://localhost:8080")
		apiKey  = envOr("HELLOJOHN_ADMIN_KEY", "")
		out     = envOr("HELLOJOHN_OUT", "text")
		timeout = 30 * time.Second
	)

	root := &cobra.Command{
		Use:   "hellojohn",
		Short: "CLI admin para HelloJohn (solo /v1/admin)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if apiKey == "" {
				return fmt.Errorf("falta API key (flag --admin-api-key o env HELLOJOHN_ADMIN_KEY)")
			}
			return nil
		},
	}

	root.PersistentFlags().StringVar(&baseURL, "admin-api-url", baseURL, "URL base del Admin API (env HELLOJOHN_ADMIN_URL)")
	root.PersistentFlags().StringVar(&apiKey, "admin-api-key", apiKey, "API key del Admin API (env HELLOJOHN_ADMIN_KEY)")
	root.PersistentFlags().StringVar(&out, "out", out, "Formato de salida: json|text")

	httpClient := &http.Client{Timeout: timeout}
	cl := &client{BaseURL: baseURL, APIKey: apiKey, OutFormat: out, HTTP: httpClient}

	// grupo admin
	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Operaciones administrativas (vía /v1/admin)",
	}

	// ping: usa GET /v1/admin/clients con limit=1
	pingCmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping al Admin API (requiere X-Admin-API-Key)",
		RunE: func(cmd *cobra.Command, args []string) error {
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
