package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

// Respuesta del discovery
type ProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

type ProviderInfo struct {
	Name     string  `json:"name"`
	Enabled  bool    `json:"enabled"`
	Ready    bool    `json:"ready"`               // nuevo: indica si el provider está correctamente configurado
	Popup    bool    `json:"popup"`               // hint para el front
	StartURL *string `json:"start_url,omitempty"` // cuando aplique (p.ej. Google)
	Reason   string  `json:"reason,omitempty"`    // SOLO para problemas de configuración del provider (no de runtime)
}

type providersHandler struct {
	c   *app.Container
	cfg *config.Config

	validator redirectValidatorAdapter
}

func NewProvidersHandler(c *app.Container, cfg *config.Config) http.Handler {
	return &providersHandler{
		c:         c,
		cfg:       cfg,
		validator: redirectValidatorAdapter{repo: c.Store},
	}
}

// GET /v1/auth/providers?tenant_id=...&client_id=...&redirect_uri=...
//   - Devuelve qué providers están disponibles.
//   - Si Google está habilitado y listo (ready) y redirect_uri es válido para ese tenant/client,
//     incluye start_url listo para usar.
//   - Ya NO muestra "tenant_id/client_id faltantes" en reason. Ese es tema del flujo, no del provider.
func (h *providersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1701)
		return
	}

	q := r.URL.Query()
	tidStr := strings.TrimSpace(q.Get("tenant_id"))
	cid := strings.TrimSpace(q.Get("client_id"))
	redirect := strings.TrimSpace(q.Get("redirect_uri"))

	resp := ProvidersResponse{Providers: make([]ProviderInfo, 0, 3)}

	// Password: informativo (si querés, podés hacerlo dinámico por client).
	resp.Providers = append(resp.Providers, ProviderInfo{
		Name:    "password",
		Enabled: true,
		Ready:   true,
		Popup:   false,
	})

	// GOOGLE
	if h.cfg.Providers.Google.Enabled {
		pi := ProviderInfo{
			Name:    "google",
			Enabled: true,
			Popup:   true,
		}

		// Determinar si el provider está listo (config correcta)
		// Regla: client_id y client_secret deben existir y el redirect interno (callback) debe ser resoluble.
		googleReady := strings.TrimSpace(h.cfg.Providers.Google.ClientID) != "" &&
			strings.TrimSpace(h.cfg.Providers.Google.ClientSecret) != ""
		// Consideramos "resoluble": si RedirectURL está vacío, debe existir jwt.issuer para derivarlo.
		if strings.TrimSpace(h.cfg.Providers.Google.RedirectURL) == "" && strings.TrimSpace(h.cfg.JWT.Issuer) == "" {
			googleReady = false
		}
		pi.Ready = googleReady

		if !googleReady {
			pi.Reason = "google provider no configurado (client_id/secret o redirect_url/jwt.issuer faltantes)"
			resp.Providers = append(resp.Providers, pi)
			writeJSON(w, resp)
			return
		}

		// Si el caller NO mandó tenant/client o mandó inválidos, no mostramos reason;
		// simplemente no generamos start_url.
		var (
			tid uuid.UUID
			err error
		)
		if tidStr != "" {
			tid, err = uuid.Parse(tidStr)
			if err != nil {
				// tid inválido ⇒ omitimos start_url
				tid = uuid.Nil
			}
		}

		// Si no viene redirect, usamos el result por defecto del servicio
		if redirect == "" {
			base := strings.TrimRight(h.cfg.JWT.Issuer, "/")
			if base != "" {
				redirect = base + "/v1/auth/social/result"
			}
		} else {
			redirect = strings.TrimSpace(redirect)
		}

		// Si tenemos tid y cid válidos y un redirect no vacío, validamos contra el client.
		if tid != uuid.Nil && cid != "" && redirect != "" {
			if h.validator.ValidateRedirectURI(tid, cid, redirect) {
				v := url.Values{}
				v.Set("tenant_id", tid.String())
				v.Set("client_id", cid)
				v.Set("redirect_uri", redirect)
				start := "/v1/auth/social/google/start?" + v.Encode()
				pi.StartURL = &start
			} else {
				// redirect_uri no permitido para ese client ⇒ omitimos start_url
				// (sin reason, para no ensuciar el discovery con errores de runtime)
			}
		}

		resp.Providers = append(resp.Providers, pi)
	} else {
		resp.Providers = append(resp.Providers, ProviderInfo{
			Name:    "google",
			Enabled: false,
			Ready:   false,
			Popup:   true,
		})
	}

	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
