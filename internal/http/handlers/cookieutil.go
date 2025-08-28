package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// parseSameSite convierte el string de config a http.SameSite.
// Acepta: "", "lax", "strict", "none" (case-insensitive).
// Default: Lax.
func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		// Nota: para navegadores modernos, SameSite=None requiere Secure=true.
		// No forzamos Secure acá para no romper ambientes http://localhost,
		// pero dejamos un warning en BuildSessionCookie si viene inseguro.
		return http.SameSiteNoneMode
	default:
		log.Printf("cookie: SameSite desconocido=%q, usando Lax", s)
		return http.SameSiteLaxMode
	}
}

// BuildSessionCookie construye la cookie de sesión con flags de seguridad.
// - name/value: nombre de cookie y valor (ej. ID de sesión o token de sesión)
// - domain: Domain opcional (si vacío, no se setea)
// - sameSite: "", "lax", "strict", "none" (case-insensitive). Default Lax.
// - secure: si true, marca Secure (recomendado en prod con https)
// - ttl: duración de la sesión; se setea Expires y Max-Age acorde
func BuildSessionCookie(name, value, domain, sameSite string, secure bool, ttl time.Duration) *http.Cookie {
	ss := parseSameSite(sameSite)
	if ss == http.SameSiteNoneMode && !secure {
		// Advertimos: algunos navegadores rechazan SameSite=None sin Secure.
		log.Printf("cookie: SameSite=None sin Secure; algunos navegadores pueden rechazar la cookie (domain=%q)", domain)
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)

	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   "",
		Expires:  exp,
		MaxAge:   int(ttl.Seconds()),
		Secure:   secure,
		HttpOnly: true,
		SameSite: ss,
	}
	if domain != "" {
		c.Domain = domain
	}
	return c
}

// BuildDeletionCookie devuelve una cookie que "borra" la sesión del browser.
// Usa mismo nombre/domain/samesite/secure para que el user-agent la sobreescriba.
func BuildDeletionCookie(name, domain, sameSite string, secure bool) *http.Cookie {
	ss := parseSameSite(sameSite)
	c := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Unix(0, 0).UTC(), // pasado
		MaxAge:   -1,                    // eliminar
		Secure:   secure,
		HttpOnly: true,
		SameSite: ss,
	}
	if domain != "" {
		c.Domain = domain
	}
	return c
}
