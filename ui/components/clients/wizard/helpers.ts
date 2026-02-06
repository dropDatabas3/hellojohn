// ============================================================================
// CLIENT WIZARD - HELPERS
// Funciones utilitarias para el flujo de clients OAuth2
// ============================================================================

import type { ClientType } from "./types"

// ----------------------------------------------------------------------------
// SLUGIFY
// ----------------------------------------------------------------------------

/**
 * Convierte texto a slug valido para client IDs.
 * Solo permite letras minusculas, numeros y underscores.
 * Trunca a 20 caracteres.
 */
export function slugify(text: string): string {
    return text
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "_")
        .replace(/^_|_$/g, "")
        .slice(0, 20)
}

// ----------------------------------------------------------------------------
// CLIENT ID GENERATION
// ----------------------------------------------------------------------------

/**
 * Genera un client_id unico basado en tenant slug, nombre y tipo.
 * Formato: {tenantSlug}_{nameSlug}_{typeShort}_{random4}
 * Ejemplo: acme_mi_app_web_x4f2
 */
export function generateClientId(tenantSlug: string, name: string, type: ClientType): string {
    const nameSlug = slugify(name)
    const typeShort = type === "public" ? "web" : "srv"
    const rand = Math.random().toString(36).substring(2, 6)
    return `${tenantSlug}_${nameSlug}_${typeShort}_${rand}`
}

// ----------------------------------------------------------------------------
// CLIENT ID VALIDATION
// ----------------------------------------------------------------------------

const CLIENT_ID_REGEX = /^[a-z0-9\-_]+$/

/**
 * Valida un client_id manual.
 * Reglas: 3-64 chars, solo lowercase alfanumerico + guiones + underscores.
 */
export function validateClientId(clientId: string): { valid: boolean; error?: string } {
    if (!clientId || clientId.trim().length === 0) {
        return { valid: false, error: "El Client ID es obligatorio" }
    }
    if (clientId.length < 3) {
        return { valid: false, error: "Minimo 3 caracteres" }
    }
    if (clientId.length > 64) {
        return { valid: false, error: "Maximo 64 caracteres" }
    }
    if (!CLIENT_ID_REGEX.test(clientId)) {
        return { valid: false, error: "Solo letras minusculas, numeros, guiones y underscores" }
    }
    return { valid: true }
}

// ----------------------------------------------------------------------------
// URI VALIDATION
// ----------------------------------------------------------------------------

/**
 * Valida una URI de redireccion.
 * Reglas del backend:
 * - https:// para cualquier dominio
 * - http:// solo para localhost o 127.0.0.1
 */
export function validateUri(uri: string): { valid: boolean; error?: string } {
    if (!uri || uri.trim().length === 0) {
        return { valid: false, error: "La URI no puede estar vacia" }
    }

    const trimmed = uri.trim()

    // Intentar parsear como URL
    try {
        const url = new URL(trimmed)

        // Permitir http solo para localhost y 127.0.0.1
        if (url.protocol === "http:") {
            const hostname = url.hostname
            if (hostname !== "localhost" && hostname !== "127.0.0.1") {
                return {
                    valid: false,
                    error: "HTTP solo permitido para localhost o 127.0.0.1. Usa HTTPS para otros dominios.",
                }
            }
            return { valid: true }
        }

        // HTTPS siempre permitido
        if (url.protocol === "https:") {
            return { valid: true }
        }

        // Esquemas custom (deep links como myapp://)
        if (trimmed.includes("://")) {
            return { valid: true }
        }

        return { valid: false, error: "Protocolo no soportado. Usa https:// o http://localhost" }
    } catch {
        // Si no es URL valida, verificar si es deep link (scheme://path)
        if (/^[a-z][a-z0-9+.-]*:\/\//.test(trimmed)) {
            return { valid: true }
        }
        return { valid: false, error: "URL invalida. Ejemplo: https://miapp.com/callback" }
    }
}

// ----------------------------------------------------------------------------
// TIME FORMATTING
// ----------------------------------------------------------------------------

/**
 * Formatea minutos a string legible.
 * Ejemplos: 15 -> "15 min", 60 -> "1h", 1440 -> "1d", 43200 -> "30d"
 */
export function formatTTL(minutes: number): string {
    if (minutes < 60) return `${minutes} min`
    if (minutes < 1440) return `${Math.round(minutes / 60)}h`
    return `${Math.round(minutes / 1440)}d`
}

/**
 * Formatea una fecha ISO a tiempo relativo en espanol.
 * Ejemplos: "hace menos de 1 min", "hace 5 min", "hace 2h", "hace 3 dias"
 */
export function formatRelativeTime(date: string): string {
    if (!date) return "—"
    const now = Date.now()
    const ts = new Date(date).getTime()
    const diff = now - ts
    if (diff < 60 * 1000) return "hace menos de 1 min"
    if (diff < 60 * 60 * 1000) return `hace ${Math.floor(diff / 60000)} min`
    if (diff < 24 * 60 * 60 * 1000) return `hace ${Math.floor(diff / 3600000)}h`
    return `hace ${Math.floor(diff / 86400000)} dias`
}
