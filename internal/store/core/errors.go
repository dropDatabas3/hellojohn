package core

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrInvalid        = errors.New("invalid")
	ErrNotImplemented = errors.New("not implemented")
)
