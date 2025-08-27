package http

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorCode        int    `json:"error_code,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, code, desc string, errCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Si ya hay X-Request-ID (lo setea el middleware), lo incluimos
	rid := w.Header().Get("X-Request-ID")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{
		Error:            code,
		ErrorDescription: desc,
		ErrorCode:        errCode,
		RequestID:        rid,
	})
}
