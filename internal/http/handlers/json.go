package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

const maxJSONBody = 64 << 10 // 64KB

func readStrictJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.Contains(ct, "application/json") {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "se requiere Content-Type: application/json", 1101)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		msg := "json inválido"
		switch err {
		case io.EOF:
			msg = "body vacío"
		default:
			// caemos en mensaje genérico; podemos refinar si querés
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", msg, 1102)
		return false
	}

	// No debe haber datos extra
	if dec.More() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "sobran datos en el body", 1103)
		return false
	}

	return true
}
