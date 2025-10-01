package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/security/totp"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

// (Removido) Antes usábamos un interface local con uuid.UUID; ahora el core.Repository ya expone los métodos con userID string.

// ENV opcionales:
// MFA_TOTP_ISSUER (string), MFA_TOTP_WINDOW (int, default 1), MFA_REMEMBER_TTL (e.g. 30d)
func mfaconfigWindow() int {
	if s := strings.TrimSpace(os.Getenv("MFA_TOTP_WINDOW")); s != "" {
		if n, err := parseInt(s); err == nil && n >= 0 && n <= 3 {
			return n
		}
	}
	return 1
}
func parseInt(s string) (int, error) { var n int; _, err := fmt.Sscanf(s, "%d", &n); return n, err }

func mfaconfigIssuer() string {
	if s := strings.TrimSpace(os.Getenv("MFA_TOTP_ISSUER")); s != "" {
		return s
	}
	return "HelloJohn"
}

func mfaconfigRememberTTL() time.Duration {
	if s := strings.TrimSpace(os.Getenv("MFA_REMEMBER_TTL")); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return 30 * 24 * time.Hour
}

// --- cifrado GCM (reusa esquema GCMV1) ---
func aesgcmEncrypt(plain []byte) (string, error) {
	k := []byte(os.Getenv("SIGNING_MASTER_KEY"))
	if len(k) < 32 {
		return "", errors.New("missing or short SIGNING_MASTER_KEY (need 32 bytes)")
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
	ct := gcm.Seal(nil, nonce, plain, nil)
	out := append(nonce, ct...)
	return "GCMV1-MFA:" + hex.EncodeToString(out), nil
}
func aesgcmDecrypt(enc string) ([]byte, error) {
	const pfx = "GCMV1-MFA:"
	if !strings.HasPrefix(enc, pfx) {
		return nil, errors.New("bad prefix")
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(enc, pfx))
	if err != nil {
		return nil, err
	}
	k := []byte(os.Getenv("SIGNING_MASTER_KEY"))
	if len(k) < 32 {
		return nil, errors.New("missing or short SIGNING_MASTER_KEY (need 32 bytes)")
	}
	block, err := aes.NewCipher(k[:32])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("short")
	}
	nonce := raw[:ns]
	ct := raw[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}

type mfaHandler struct {
	c          *app.Container
	refreshTTL time.Duration
	// Rate limit config (parsed durations)
	rl struct {
		enroll, verify, challenge, disable helpers.MFARateConfig
	}
}

func NewMFAHandler(c *app.Container, cfg *config.Config, refreshTTL time.Duration) *mfaHandler {
	// Parse MFA windows from config; defaults already applied in config loader
	parseWin := func(s string, def time.Duration) time.Duration {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
		return def
	}

	h := &mfaHandler{c: c, refreshTTL: refreshTTL}
	h.rl.enroll = helpers.MFARateConfig{Limit: cfg.Rate.MFA.Enroll.Limit, Window: parseWin(cfg.Rate.MFA.Enroll.Window, 10*time.Minute)}
	h.rl.verify = helpers.MFARateConfig{Limit: cfg.Rate.MFA.Verify.Limit, Window: parseWin(cfg.Rate.MFA.Verify.Window, time.Minute)}
	h.rl.challenge = helpers.MFARateConfig{Limit: cfg.Rate.MFA.Challenge.Limit, Window: parseWin(cfg.Rate.MFA.Challenge.Window, time.Minute)}
	h.rl.disable = helpers.MFARateConfig{Limit: cfg.Rate.MFA.Disable.Limit, Window: parseWin(cfg.Rate.MFA.Disable.Window, 10*time.Minute)}
	return h
}

func (h *mfaHandler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Post("/v1/mfa/totp/enroll", h.enroll)
		r.Post("/v1/mfa/totp/verify", h.verify)
		r.Post("/v1/mfa/totp/challenge", h.challenge)
		r.Post("/v1/mfa/totp/disable", h.disable)
		r.Post("/v1/mfa/recovery/rotate", h.rotateRecovery)
	})
}

// Exponer handlers para ServeMux
func (h *mfaHandler) HTTPEnroll() http.Handler         { return http.HandlerFunc(h.enroll) }
func (h *mfaHandler) HTTPVerify() http.Handler         { return http.HandlerFunc(h.verify) }
func (h *mfaHandler) HTTPChallenge() http.Handler      { return http.HandlerFunc(h.challenge) }
func (h *mfaHandler) HTTPDisable() http.Handler        { return http.HandlerFunc(h.disable) }
func (h *mfaHandler) HTTPRecoveryRotate() http.Handler { return http.HandlerFunc(h.rotateRecovery) }

func currentUserFromHeader(r *http.Request) (string, error) {
	uidStr := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if uidStr == "" {
		return "", errors.New("missing X-User-ID")
	}
	if _, err := uuid.Parse(uidStr); err != nil { // validación básica
		return "", errors.New("invalid X-User-ID")
	}
	return uidStr, nil
}

// POST /v1/mfa/totp/enroll
func (h *mfaHandler) enroll(w http.ResponseWriter, r *http.Request) {
	// Rate limit: enroll is low frequency
	if h.c.MultiLimiter != nil {
		// conservative defaults handled at config level; here we pass from env via headers if available later
		// Without tenant context on header, key by user+ip only
		if !helpers.EnforceMFAEnrollLimit(w, r, h.c.MultiLimiter, h.rl.enroll, r.Header.Get("X-User-ID")) {
			return
		}
	}
	uid, err := currentUserFromHeader(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "login requerido", 1701)
		return
	}

	u, err := h.c.Store.GetUserByID(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "user_not_found", "usuario inválido", 1701)
		return
	}
	email := u.Email

	_, b32, err := totp.GenerateSecret()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "mfa_init_failed", "no se pudo generar secreto", 1702)
		return
	}

	enc, err := aesgcmEncrypt([]byte(b32))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "no se pudo cifrar", 1703)
		return
	}

	if err := h.c.Store.UpsertMFATOTP(r.Context(), uid, enc); errors.Is(err, core.ErrNotImplemented) {
		httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store sin soporte MFA", 1799)
		return
	} else if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "no se pudo guardar secreto", 1704)
		return
	}

	out := map[string]any{
		"secret_base32": b32,
		"otpauth_url":   totp.OTPAuthURL(mfaconfigIssuer(), email, b32),
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// POST /v1/mfa/totp/verify {code}

func (h *mfaHandler) verify(w http.ResponseWriter, r *http.Request) {
	uid, err := currentUserFromHeader(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "login requerido", 1711)
		return
	}
	// Rate limit: verify
	if h.c.MultiLimiter != nil {
		if !helpers.EnforceMFAVerifyLimit(w, r, h.c.MultiLimiter, h.rl.verify, uid) {
			return
		}
	}
	var req struct {
		Code string `json:"code"`
	}
	if !httpx.ReadJSON(w, r, &req) {
		return
	}

	m, err := h.c.Store.GetMFATOTP(r.Context(), uid)
	if err != nil || m == nil {
		httpx.WriteError(w, http.StatusBadRequest, "mfa_not_initialized", "primero enroll", 1712)
		return
	}
	plain, err := aesgcmDecrypt(m.SecretEncrypted)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "no se pudo descifrar", 1713)
		return
	}

	win := mfaconfigWindow()
	var last *int64
	if m.LastUsedAt != nil {
		c := m.LastUsedAt.Unix() / 30
		last = &c
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(string(plain))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "secreto inválido", 1714)
		return
	}
	if ok, counter := totp.Verify(raw, req.Code, time.Now(), win, last); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido", 1715)
		return
	} else {
		_ = h.c.Store.UpdateMFAUsedAt(r.Context(), uid, time.Unix(counter*30, 0).UTC())
		// Confirm y potencial generación de recovery codes si es primera vez
		firstTime := m.ConfirmedAt == nil
		_ = h.c.Store.ConfirmMFATOTP(r.Context(), uid, time.Now().UTC())
		if firstTime {
			if recPlain, recHashes, errGen := generateRecoveryCodes(10); errGen == nil {
				if err := h.c.Store.InsertRecoveryCodes(r.Context(), uid, recHashes); err == nil {
					// Respuesta con codes one-time
					resp := map[string]any{"enabled": true, "recovery_codes": recPlain}
					httpx.WriteJSON(w, http.StatusOK, resp)
					return
				}
			}
			// Si falla generación o inserción, devolvemos enabled sin codes (no bloquea MFA)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"enabled": true})
}

// POST /v1/mfa/totp/challenge {mfa_token, code|recovery, remember_device?}

func (h *mfaHandler) challenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
		Recovery string `json:"recovery"`
		Remember bool   `json:"remember_device"`
	}
	if !httpx.ReadJSON(w, r, &req) {
		return
	}
	req.MFAToken = strings.TrimSpace(req.MFAToken)

	// 🔴 Validación primero: falta code/recovery -> 400
	if strings.TrimSpace(req.Code) == "" && strings.TrimSpace(req.Recovery) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_mfa_credential", "se requiere code o recovery", 1720)
		return
	}

	key := "mfa:token:" + req.MFAToken // Declarar al principio para que esté en scope
	payload, ok := h.c.Cache.Get(key)
	if !ok || len(payload) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "mfa_token_not_found", "token inválido o expirado", 1721)
		return
	}
	// No eliminar aquí - solo después de validación exitosa

	var ch mfaChallenge
	if err := json.Unmarshal(payload, &ch); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cache_payload_invalid", "payload inválido", 1722)
		return
	}
	uidStr := ch.UserID

	// Rate limit: challenge per user
	if h.c.MultiLimiter != nil {
		if !helpers.EnforceMFAChallengeLimit(w, r, h.c.MultiLimiter, h.rl.challenge, uidStr) {
			return
		}
	}

	// 1) Recovery
	if strings.TrimSpace(req.Recovery) != "" {
		hh := tokens.SHA256Base64URL(strings.TrimSpace(req.Recovery))
		ok, err := h.c.Store.UseRecoveryCode(r.Context(), uidStr, hh, time.Now().UTC())
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "no se pudo validar recovery", 1723)
			return
		}
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "recovery inválido", 1724)
			return
		}
	} else {
		// 2) TOTP
		m, err := h.c.Store.GetMFATOTP(r.Context(), uidStr)
		if err != nil || m == nil || m.ConfirmedAt == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_state", "MFA no habilitado", 1725)
			return
		}
		plain, err := aesgcmDecrypt(m.SecretEncrypted)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "no se pudo descifrar", 1726)
			return
		}
		raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(string(plain))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "secreto inválido", 1727)
			return
		}

		var last *int64
		if m.LastUsedAt != nil {
			c := m.LastUsedAt.Unix() / 30
			last = &c
		}
		if ok, counter := totp.Verify(raw, req.Code, time.Now(), mfaconfigWindow(), last); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido", 1728)
			return
		} else {
			_ = h.c.Store.UpdateMFAUsedAt(r.Context(), uidStr, time.Unix(counter*30, 0).UTC())
		}
	}

	// Remember device
	if req.Remember {
		devToken, _ := tokens.GenerateOpaqueToken(32)
		devHash := tokens.SHA256Base64URL(devToken)
		ttl := mfaconfigRememberTTL()
		_ = h.c.Store.AddTrustedDevice(r.Context(), uidStr, devHash, time.Now().Add(ttl).UTC())
		// Cookie debe ser Secure solo si el request es HTTPS (o detrás de proxy con X-Forwarded-Proto=https)
		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		http.SetCookie(w, &http.Cookie{
			Name:     "mfa_trust",
			Value:    devToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(ttl).UTC(),
		})
	}

	// Emitir tokens con amr=["pwd","mfa"]
	std := map[string]any{"tid": ch.TenantID, "amr": append(ch.AMRBase, "mfa"), "acr": "urn:hellojohn:loa:2"}
	custom := map[string]any{}
	std, custom = applyAccessClaimsHook(r.Context(), h.c, ch.TenantID, ch.ClientID, uidStr, ch.Scope, append(ch.AMRBase, "mfa"), std, custom)
	token, exp, err := h.c.Issuer.IssueAccess(uidStr, ch.ClientID, std, custom)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 1729)
		return
	}

	rawRT, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1730)
		return
	}
	hash := tokens.SHA256Base64URL(rawRT)
	cl, _, err := h.c.Store.GetClientByClientID(r.Context(), ch.ClientID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1731)
		return
	}

	if _, err := h.c.Store.CreateRefreshToken(r.Context(), uidStr, cl.ID, hash, time.Now().Add(h.refreshTTL), nil); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1732)
		return
	}

	// Eliminar el token MFA solo después de éxito
	h.c.Cache.Delete(key)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
		AccessToken:  token,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(exp).Seconds()),
		RefreshToken: rawRT,
	})
}

// POST /v1/mfa/totp/disable {code or recovery}
func (h *mfaHandler) disable(w http.ResponseWriter, r *http.Request) {
	uid, err := currentUserFromHeader(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "login requerido", 1741)
		return
	}
	// Rate limit: disable
	if h.c.MultiLimiter != nil {
		if !helpers.EnforceMFADisableLimit(w, r, h.c.MultiLimiter, h.rl.disable, uid) {
			return
		}
	}
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
		Recovery string `json:"recovery"`
	}
	if !httpx.ReadJSON(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_password", "password requerido", 1742)
		return
	}
	if strings.TrimSpace(req.Recovery) == "" && strings.TrimSpace(req.Code) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "code o recovery requerido", 1743)
		return
	}

	// Validar password (obtener identity)
	user, err := h.c.Store.GetUserByID(r.Context(), uid)
	if err != nil || user == nil {
		httpx.WriteError(w, http.StatusBadRequest, "user_not_found", "usuario inválido", 1744)
		return
	}
	_, identity, err := h.c.Store.GetUserByEmail(r.Context(), user.TenantID, user.Email)
	if err != nil || identity == nil || identity.PasswordHash == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "no_password_identity", "identity password no encontrada", 1745)
		return
	}
	if ok := h.c.Store.CheckPassword(identity.PasswordHash, req.Password); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "password inválido", 1746)
		return
	}

	if strings.TrimSpace(req.Recovery) != "" {
		hh := tokens.SHA256Base64URL(strings.TrimSpace(req.Recovery))
		if ok, _ := h.c.Store.UseRecoveryCode(r.Context(), uid, hh, time.Now().UTC()); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "recovery inválido", 1747)
			return
		}
	} else {
		m, err := h.c.Store.GetMFATOTP(r.Context(), uid)
		if err != nil || m == nil || m.ConfirmedAt == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_state", "MFA no habilitado", 1748)
			return
		}
		plain, _ := aesgcmDecrypt(m.SecretEncrypted)
		raw, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(string(plain))
		if ok, _ := totp.Verify(raw, strings.TrimSpace(req.Code), time.Now(), mfaconfigWindow(), nil); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido", 1749)
			return
		}
	}
	_ = h.c.Store.DisableMFATOTP(r.Context(), uid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"disabled": true})
}

// POST /v1/mfa/recovery/rotate {password, code|recovery}
// Requiere: usuario logueado; password actual y un segundo factor (TOTP válido o recovery válido no usado)
// Respuesta: {rotated: true, recovery_codes: []} (codes solo se devuelven una vez)
func (h *mfaHandler) rotateRecovery(w http.ResponseWriter, r *http.Request) {
	uid, err := currentUserFromHeader(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "login requerido", 1751)
		return
	}
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
		Recovery string `json:"recovery"`
	}
	if !httpx.ReadJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_password", "password requerido", 1752)
		return
	}
	if strings.TrimSpace(req.Code) == "" && strings.TrimSpace(req.Recovery) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_second_factor", "code o recovery requerido", 1753)
		return
	}
	user, err := h.c.Store.GetUserByID(r.Context(), uid)
	if err != nil || user == nil {
		httpx.WriteError(w, http.StatusBadRequest, "user_not_found", "usuario inválido", 1754)
		return
	}
	_, identity, err := h.c.Store.GetUserByEmail(r.Context(), user.TenantID, user.Email)
	if err != nil || identity == nil || identity.PasswordHash == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "no_password_identity", "identity password no encontrada", 1755)
		return
	}
	if ok := h.c.Store.CheckPassword(identity.PasswordHash, req.Password); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "password inválido", 1756)
		return
	}
	if strings.TrimSpace(req.Recovery) != "" {
		hh := tokens.SHA256Base64URL(strings.TrimSpace(req.Recovery))
		if ok, _ := h.c.Store.UseRecoveryCode(r.Context(), uid, hh, time.Now().UTC()); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "recovery inválido", 1757)
			return
		}
	} else {
		m, err := h.c.Store.GetMFATOTP(r.Context(), uid)
		if err != nil || m == nil || m.ConfirmedAt == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_state", "MFA no habilitado", 1758)
			return
		}
		plain, err := aesgcmDecrypt(m.SecretEncrypted)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "no se pudo descifrar", 1759)
			return
		}
		raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(string(plain))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "crypto_failed", "secreto inválido", 1760)
			return
		}
		if ok, counter := totp.Verify(raw, strings.TrimSpace(req.Code), time.Now(), mfaconfigWindow(), nil); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido", 1761)
			return
		} else {
			_ = h.c.Store.UpdateMFAUsedAt(r.Context(), uid, time.Unix(counter*30, 0).UTC())
		}
	}
	_ = h.c.Store.DeleteRecoveryCodes(r.Context(), uid)
	recPlain, recHashes, err := generateRecoveryCodes(10)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "gen_failed", "no se pudo generar", 1762)
		return
	}
	if err := h.c.Store.InsertRecoveryCodes(r.Context(), uid, recHashes); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir", 1763)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rotated": true, "recovery_codes": recPlain})
}

// generateRecoveryCodes genera n códigos de recuperación, devolviendo lista en claro y sus hashes.
// Formato: 10 caracteres alfanuméricos (A-Z2-9 sin ILOU) para evitar confusiones.
// Se devuelven en mayúsculas; el hash se calcula sobre la versión en minúsculas.
func generateRecoveryCodes(n int) (plain []string, hashes []string, err error) {
	if n <= 0 {
		return []string{}, []string{}, nil
	}
	alphabet := []rune("ABCDEFGHJKMNPQRSTVWXYZ23456789")
	plain = make([]string, 0, n)
	hashes = make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := make([]rune, 10)
		for j := 0; j < 10; j++ {
			// crypto/rand via big.Int
			r, e := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if e != nil {
				return nil, nil, e
			}
			b[j] = alphabet[r.Int64()]
		}
		code := string(b)
		plain = append(plain, code)
		hashes = append(hashes, tokens.SHA256Base64URL(strings.ToLower(code)))
	}
	return plain, hashes, nil
}
