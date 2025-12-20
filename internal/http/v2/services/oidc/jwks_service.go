// Package oidc contiene los services para endpoints OIDC/Discovery.
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// JWKSService define las operaciones para obtener JWKS.
type JWKSService interface {
	GetGlobalJWKS(ctx context.Context) (json.RawMessage, error)
	GetTenantJWKS(ctx context.Context, slug string) (json.RawMessage, error)
}

// jwksService implementa JWKSService.
type jwksService struct {
	cache *jwtx.JWKSCache
}

// NewJWKSService crea un nuevo servicio JWKS.
func NewJWKSService(cache *jwtx.JWKSCache) JWKSService {
	return &jwksService{cache: cache}
}

const componentJWKS = "oidc.jwks"

// slugRe valida slugs de tenant: solo a-z, 0-9, - con max 64 chars.
var slugRe = regexp.MustCompile(`^[a-z0-9\-]{1,64}$`)

// ErrInvalidSlug indica que el slug es inválido.
var ErrInvalidSlug = fmt.Errorf("invalid slug format")

func (s *jwksService) GetGlobalJWKS(ctx context.Context) (json.RawMessage, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentJWKS),
		logger.Op("GetGlobalJWKS"),
	)

	data, err := s.cache.Get("global")
	if err != nil {
		log.Error("failed to get global JWKS", logger.Err(err))
		return nil, err
	}

	return data, nil
}

func (s *jwksService) GetTenantJWKS(ctx context.Context, slug string) (json.RawMessage, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentJWKS),
		logger.Op("GetTenantJWKS"),
		logger.TenantSlug(slug),
	)

	// Validar slug
	if !slugRe.MatchString(slug) {
		return nil, ErrInvalidSlug
	}

	data, err := s.cache.Get(slug)
	if err != nil {
		log.Error("failed to get tenant JWKS", logger.Err(err))
		return nil, err
	}

	return data, nil
}
