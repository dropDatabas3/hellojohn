package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminConsentsHandler struct{ c *app.Container }

func NewAdminConsentsHandler(c *app.Container) *AdminConsentsHandler {
	return &AdminConsentsHandler{c: c}
}

// resolveClientID acepta UUID interno o client_id público.
// Si viene client_id público, lo resuelve a UUID (core.Client.ID).
func (h *AdminConsentsHandler) resolveClientID(r *http.Request, in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", core.ErrInvalid
	}
	// ¿Parece UUID?
	if _, err := uuid.Parse(in); err == nil {
		return in, nil
	}
	// Resolver por client_id público
	cl, _, err := h.c.Store.GetClientByClientID(r.Context(), in)
	if err != nil {
		if err == core.ErrNotFound {
			return "", core.ErrNotFound
		}
		return "", err
	}
	return cl.ID, nil
}

func (h *AdminConsentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.ScopesConsents == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado por este driver", 1900)
		return
	}

	switch {
	// ───────────────────────────────────────────
	// POST /v1/admin/consents/upsert
	// body: { user_id, client_id, scopes: [] }
	// client_id: acepta UUID o client_id público
	// ───────────────────────────────────────────
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/consents/upsert":
		var body struct {
			UserID   string   `json:"user_id"`
			ClientID string   `json:"client_id"`
			Scopes   []string `json:"scopes"`
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.UserID = strings.TrimSpace(body.UserID)
		body.ClientID = strings.TrimSpace(body.ClientID)

		if body.UserID == "" || body.ClientID == "" || len(body.Scopes) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id, client_id y scopes requeridos", 2001)
			return
		}
		if _, err := uuid.Parse(body.UserID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2002)
			return
		}

		cid, err := h.resolveClientID(r, body.ClientID)
		if err != nil {
			status := http.StatusBadRequest
			code := "invalid_client_id"
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				status = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, status, code, desc, 2003)
			return
		}

		uc, err := h.c.ScopesConsents.UpsertConsent(r.Context(), body.UserID, cid, body.Scopes)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 2004)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, uc)

	// ───────────────────────────────────────────
	// GET /v1/admin/consents/by-user/{userID}?active_only=true
	// GET /v1/admin/consents/by-user?user_id={userID}&active_only=true
	// ───────────────────────────────────────────
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/by-user"):
		// preferir query param
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/by-user/") {
			// fallback: path param exacto
			raw := strings.TrimPrefix(r.URL.Path, "/v1/admin/consents/by-user/")
			// r.URL.Path NO incluye querystring, así que raw ya es solo el id
			userID = strings.TrimSpace(raw)
		}
		if userID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_user_id", "user_id requerido", 2011)
			return
		}
		if _, err := uuid.Parse(userID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2012)
			return
		}
		activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true")
		list, err := h.c.ScopesConsents.ListConsentsByUser(r.Context(), userID, activeOnly)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 2013)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, list)

	// ───────────────────────────────────────────
	// POST /v1/admin/consents/revoke
	// body: { user_id, client_id, at? (RFC3339) }
	// client_id: acepta UUID o client_id público
	// ───────────────────────────────────────────
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/consents/revoke":
		var body struct {
			UserID   string `json:"user_id"`
			ClientID string `json:"client_id"`
			At       string `json:"at,omitempty"` // opcional RFC3339
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.UserID = strings.TrimSpace(body.UserID)
		body.ClientID = strings.TrimSpace(body.ClientID)

		if body.UserID == "" || body.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id y client_id requeridos", 2021)
			return
		}
		if _, err := uuid.Parse(body.UserID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2022)
			return
		}

		cid, err := h.resolveClientID(r, body.ClientID)
		if err != nil {
			status := http.StatusBadRequest
			code := "invalid_client_id"
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				status = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, status, code, desc, 2023)
			return
		}

		var at time.Time
		if strings.TrimSpace(body.At) != "" {
			t, err := time.Parse(time.RFC3339, body.At)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_at", "at debe ser RFC3339", 2024)
				return
			}
			at = t
		} else {
			at = time.Now()
		}

		if err := h.c.ScopesConsents.RevokeConsent(r.Context(), body.UserID, cid, at); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "revoke_failed", err.Error(), 2025)
			return
		}
		// Revocación efectiva de refresh tokens (best-effort)
		type revAll interface {
			RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
		}
		if rAll, ok := h.c.Store.(revAll); ok {
			_ = rAll.RevokeAllRefreshTokens(r.Context(), body.UserID, cid)
		}
		w.WriteHeader(http.StatusNoContent)

	// ───────────────────────────────────────────
	// GET /v1/admin/consents?user_id=&client_id=&active_only=true
	// Si user_id && client_id => 0..1 (Get + wrap)
	// Si solo user_id => ListConsentsByUser
	// Si ninguno => 400
	// ───────────────────────────────────────────
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/consents":
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
		activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true")

		// Normalizar clientID (acepta UUID o client_id público)
		if clientID != "" {
			if cid, err := h.resolveClientID(r, clientID); err == nil {
				clientID = cid
			} else if err == core.ErrNotFound {
				// pidieron un par inexistente => lista vacía
				httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
				return
			} else {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "client_id invalido", 21001)
				return
			}
		}

		switch {
		case userID != "" && clientID != "":
			if _, err := uuid.Parse(userID); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21002)
				return
			}
			uc, err := h.c.ScopesConsents.GetConsent(r.Context(), userID, clientID)
			if err != nil {
				if err == core.ErrNotFound {
					httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
					return
				}
				httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 21003)
				return
			}
			if activeOnly && uc.RevokedAt != nil {
				httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
				return
			}
			httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{uc})
			return

		case userID != "":
			if _, err := uuid.Parse(userID); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21004)
				return
			}
			list, err := h.c.ScopesConsents.ListConsentsByUser(r.Context(), userID, activeOnly)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 21005)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, list)
			return

		default:
			httpx.WriteError(w, http.StatusBadRequest, "missing_filters", "requerido al menos user_id o user_id+client_id", 21006)
			return
		}

	// ───────────────────────────────────────────
	// DELETE /v1/admin/consents/{user_id}/{client_id}
	// Implementado como revocación (soft) en 'now()'.
	// client_id acepta UUID o client_id público.
	// ───────────────────────────────────────────
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/"):
		tail := strings.TrimPrefix(r.URL.Path, "/v1/admin/consents/")
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) != 2 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_path", "se espera /v1/admin/consents/{user_id}/{client_id}", 21011)
			return
		}
		userID := strings.TrimSpace(parts[0])
		clientID := strings.TrimSpace(parts[1])
		if _, err := uuid.Parse(userID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21012)
			return
		}
		cid, err := h.resolveClientID(r, clientID)
		if err != nil {
			code := http.StatusBadRequest
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				code = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, code, "invalid_client_id", desc, 21013)
			return
		}
		if err := h.c.ScopesConsents.RevokeConsent(r.Context(), userID, cid, time.Now()); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "revoke_failed", err.Error(), 21014)
			return
		}
		// Revocación efectiva de refresh tokens (best-effort)
		type revAll interface {
			RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
		}
		if rAll, ok := h.c.Store.(revAll); ok {
			_ = rAll.RevokeAllRefreshTokens(r.Context(), userID, cid)
		}
		w.WriteHeader(http.StatusNoContent)
		return

	default:
		http.NotFound(w, r)
	}
}
