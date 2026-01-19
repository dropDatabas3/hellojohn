package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type apiError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorCode        int    `json:"error_code,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, code, desc string, errCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	rid := w.Header().Get("X-Request-ID")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{
		Error:            code,
		ErrorDescription: desc,
		ErrorCode:        errCode,
		RequestID:        rid,
	})
}

// WriteJSON: respuesta JSON estándar
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ReadJSON: decodifica JSON de forma tolerante (NO falla por campos desconocidos).
// Valida Content-Type y limita el tamaño del body a 1MB.
func ReadJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.Contains(ct, "application/json") {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Content-Type debe ser application/json", 1102)
		return false
	}
	// máx 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	// NOTA: NO usamos DisallowUnknownFields para no romper por campos extra (p.ej. tenant_id).
	if err := dec.Decode(v); err != nil && err != io.EOF {
		WriteError(w, http.StatusBadRequest, "invalid_json", "json inválido", 1102)
		return false
	}
	return true
}

// ─────────────────────── Custom error helpers ───────────────────────

const (
	ErrCodeTenantDBMissing      = 2601
	ErrCodeTenantDBError        = 2602
	ErrCodeTenantDBPingError    = 2603 // optional, reserved
	ErrCodeTenantDBMigrateError = 2604 // optional, reserved
)

// WriteTenantDBMissing writes a 501 Not Implemented with a clear message that the tenant has no configured DB.
func WriteTenantDBMissing(w http.ResponseWriter) {
	WriteError(w, http.StatusNotImplemented, "tenant_db_missing", "Este tenant no tiene DB configurada o validada.", ErrCodeTenantDBMissing)
}

// WriteTenantDBError writes a 500 when opening the tenant DB or running migrations failed.
func WriteTenantDBError(w http.ResponseWriter, detail string) {
	msg := "Fallo abriendo pool/migraciones del tenant."
	if strings.TrimSpace(detail) != "" {
		msg = msg + " " + detail
	}
	WriteError(w, http.StatusInternalServerError, "tenant_db_error", msg, ErrCodeTenantDBError)
}
