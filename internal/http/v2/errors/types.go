package errors

import (
	"fmt"
	"net/http"
)

// AppError define la estructura estándar para errores de la aplicación v2
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	HTTPStatus int    `json:"-"` // No se serializa, usado para el header
	Err        error  `json:"-"` // Error original (causa), útil para logs, no se expone al cliente por defecto
}

// Error implementa la interfaz error
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap permite acceder al error original
func (e *AppError) Unwrap() error {
	return e.Err
}

// New crea un nuevo AppError
func New(status int, code, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

// Wrap crea un AppError envolviendo un error existente
func Wrap(err error, status int, code, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Err:        err,
	}
}

// FromError intenta convertir un error genérico en un AppError.
// Si no es un AppError, devuelve un error interno genérico conservando el error original.
// Esto cumple el requerimiento de manejar errores de otras capas.
func FromError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return ErrInternalServerError.WithCause(err)
}

// WithDetail agrega detalles adicionales al error (útil para validaciones)
// Devuelve una COPIA del error para no mutar las variables globales base
func (e *AppError) WithDetail(detail string) *AppError {
	newErr := *e
	newErr.Detail = detail
	return &newErr
}

// WithCause agrega el error original (causa)
// Devuelve una COPIA del error
func (e *AppError) WithCause(err error) *AppError {
	newErr := *e
	newErr.Err = err
	return &newErr
}

// =================================================================================
// LISTA DE ERRORES PREDEFINIDOS
// =================================================================================

// ---------------------------------------------------------------------------------
// 400 Bad Request - Errores de Cliente / Validación
// ---------------------------------------------------------------------------------

var (
	ErrBadRequest = &AppError{
		Code:       "BAD_REQUEST",
		Message:    "La solicitud contiene sintaxis inválida o parámetros faltantes.",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrInvalidJSON = &AppError{
		Code:       "INVALID_JSON",
		Message:    "El cuerpo de la solicitud no es un JSON válido.",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrMissingFields = &AppError{
		Code:       "MISSING_FIELDS",
		Message:    "Faltan campos requeridos en la solicitud.",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrInvalidFormat = &AppError{
		Code:       "INVALID_FORMAT",
		Message:    "El formato de uno o más campos es inválido.",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrInvalidParameter = &AppError{
		Code:       "INVALID_PARAMETER",
		Message:    "Uno de los parámetros de la URL o Query String es inválido.",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrBodyTooLarge = &AppError{
		Code:       "BODY_TOO_LARGE",
		Message:    "El cuerpo de la solicitud excede el tamaño máximo permitido.",
		HTTPStatus: http.StatusRequestEntityTooLarge,
	}
)

// ---------------------------------------------------------------------------------
// 401 Unauthorized - Errores de Autenticación
// ---------------------------------------------------------------------------------

var (
	ErrUnauthorized = &AppError{
		Code:       "UNAUTHORIZED",
		Message:    "No autorizado. Se requiere autenticación.",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrInvalidCredentials = &AppError{
		Code:       "INVALID_CREDENTIALS",
		Message:    "Las credenciales proporcionadas son inválidas.",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrTokenExpired = &AppError{
		Code:       "TOKEN_EXPIRED",
		Message:    "El token de acceso ha expirado.",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrTokenInvalid = &AppError{
		Code:       "TOKEN_INVALID",
		Message:    "El token de acceso es inválido o está malformado.",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrTokenMissing = &AppError{
		Code:       "TOKEN_MISSING",
		Message:    "No se proporcionó token de autenticación.",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrSessionExpired = &AppError{
		Code:       "SESSION_EXPIRED",
		Message:    "La sesión ha expirado, por favor inicie sesión nuevamente.",
		HTTPStatus: http.StatusUnauthorized,
	}
)

// ---------------------------------------------------------------------------------
// 403 Forbidden - Errores de Permisos
// ---------------------------------------------------------------------------------

var (
	ErrForbidden = &AppError{
		Code:       "FORBIDDEN",
		Message:    "No tiene permisos para realizar esta acción.",
		HTTPStatus: http.StatusForbidden,
	}

	ErrAccountSuspended = &AppError{
		Code:       "ACCOUNT_SUSPENDED",
		Message:    "La cuenta está suspendida y no puede realizar acciones.",
		HTTPStatus: http.StatusForbidden,
	}

	ErrAccountNotVerified = &AppError{
		Code:       "ACCOUNT_NOT_VERIFIED",
		Message:    "La cuenta debe ser verificada antes de continuar.",
		HTTPStatus: http.StatusForbidden,
	}

	ErrInsufficientScopes = &AppError{
		Code:       "INSUFFICIENT_SCOPES",
		Message:    "El token no tiene los scopes necesarios para este recurso.",
		HTTPStatus: http.StatusForbidden,
	}
)

// ---------------------------------------------------------------------------------
// 404 Not Found - Recursos no encontrados
// ---------------------------------------------------------------------------------

var (
	ErrNotFound = &AppError{
		Code:       "NOT_FOUND",
		Message:    "El recurso solicitado no fue encontrado.",
		HTTPStatus: http.StatusNotFound,
	}

	ErrUserNotFound = &AppError{
		Code:       "USER_NOT_FOUND",
		Message:    "El usuario especificado no existe.",
		HTTPStatus: http.StatusNotFound,
	}

	ErrTenantNotFound = &AppError{
		Code:       "TENANT_NOT_FOUND",
		Message:    "El tenant especificado no existe.",
		HTTPStatus: http.StatusNotFound,
	}

	ErrRouteNotFound = &AppError{
		Code:       "ROUTE_NOT_FOUND",
		Message:    "La ruta solicitada no existe.",
		HTTPStatus: http.StatusNotFound,
	}
)

// ---------------------------------------------------------------------------------
// 405 Method Not Allowed
// ---------------------------------------------------------------------------------

var (
	ErrMethodNotAllowed = &AppError{
		Code:       "METHOD_NOT_ALLOWED",
		Message:    "El método HTTP no está permitido para este recurso.",
		HTTPStatus: http.StatusMethodNotAllowed,
	}
)

// ---------------------------------------------------------------------------------
// 409 Conflict - Errores de Estado/Conflicto
// ---------------------------------------------------------------------------------

var (
	ErrConflict = &AppError{
		Code:       "CONFLICT",
		Message:    "La solicitud entra en conflicto con el estado actual del servidor.",
		HTTPStatus: http.StatusConflict,
	}

	ErrAlreadyExists = &AppError{
		Code:       "ALREADY_EXISTS",
		Message:    "El recurso ya existe.",
		HTTPStatus: http.StatusConflict,
	}

	ErrEmailAlreadyInUse = &AppError{
		Code:       "EMAIL_ALREADY_IN_USE",
		Message:    "El correo electrónico ya está registrado.",
		HTTPStatus: http.StatusConflict,
	}

	ErrUsernameTaken = &AppError{
		Code:       "USERNAME_TAKEN",
		Message:    "El nombre de usuario ya está en uso.",
		HTTPStatus: http.StatusConflict,
	}
)

// ---------------------------------------------------------------------------------
// 422 Unprocessable Entity - Errores de Lógica de Negocio
// ---------------------------------------------------------------------------------

var (
	ErrUnprocessableEntity = &AppError{
		Code:       "UNPROCESSABLE_ENTITY",
		Message:    "No se pudo procesar las instrucciones contenidas.",
		HTTPStatus: http.StatusUnprocessableEntity,
	}

	ErrPasswordTooWeak = &AppError{
		Code:       "PASSWORD_TOO_WEAK",
		Message:    "La contraseña no cumple con los requisitos de seguridad.",
		HTTPStatus: http.StatusUnprocessableEntity,
	}
)

// ---------------------------------------------------------------------------------
// 412 Precondition Failed & 428 Precondition Required - Concurrency Control
// ---------------------------------------------------------------------------------

var (
	ErrPreconditionFailed = &AppError{
		Code:       "PRECONDITION_FAILED",
		Message:    "La condición previa de solicitud falló (e.g. ETag no coincide).",
		HTTPStatus: http.StatusPreconditionFailed,
	}

	ErrPreconditionRequired = &AppError{
		Code:       "PRECONDITION_REQUIRED",
		Message:    "Se requiere una condición previa (e.g. If-Match).",
		HTTPStatus: http.StatusPreconditionRequired,
	}
)

// ---------------------------------------------------------------------------------
// 429 Too Many Requests - Rate Limiting
// ---------------------------------------------------------------------------------

var (
	ErrRateLimitExceeded = &AppError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "Ha excedido el límite de solicitudes. Intente más tarde.",
		HTTPStatus: http.StatusTooManyRequests,
	}
)

// ---------------------------------------------------------------------------------
// 500+ Server Errors - Errores Internos
// ---------------------------------------------------------------------------------

var (
	ErrInternalServerError = &AppError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    "Ocurrió un error interno en el servidor.",
		HTTPStatus: http.StatusInternalServerError,
	}

	ErrNotImplemented = &AppError{
		Code:       "NOT_IMPLEMENTED",
		Message:    "Esta funcionalidad aún no está implementada.",
		HTTPStatus: http.StatusNotImplemented,
	}

	ErrServiceUnavailable = &AppError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "El servicio no está disponible temporalmente.",
		HTTPStatus: http.StatusServiceUnavailable,
	}

	ErrGatewayTimeout = &AppError{
		Code:       "GATEWAY_TIMEOUT",
		Message:    "El servidor tardó demasiado en responder.",
		HTTPStatus: http.StatusGatewayTimeout,
	}
)
