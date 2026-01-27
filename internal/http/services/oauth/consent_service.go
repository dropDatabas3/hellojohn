package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/oauth"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// Errors
var (
	ErrConsentMissingToken = errors.New("consent_token required")
	ErrConsentNotFound     = errors.New("invalid or expired consent_token")
	ErrConsentStoreFailed  = errors.New("failed to store consent")
	ErrConsentCodeFailed   = errors.New("failed to generate auth code")
)

// ConsentService handles consent acceptance logic.
type ConsentService interface {
	Accept(ctx context.Context, req dto.ConsentAcceptRequest) (*dto.AuthCodeRedirect, error)
}

// ConsentDeps dependencies.
type ConsentDeps struct {
	DAL   store.DataAccessLayer
	Cache CacheClient
}

type consentService struct {
	dal   store.DataAccessLayer
	cache CacheClient
}

func NewConsentService(d ConsentDeps) ConsentService {
	return &consentService{
		dal:   d.DAL,
		cache: d.Cache,
	}
}

// Accept processes the user's decision on the consent screen.
func (s *consentService) Accept(ctx context.Context, req dto.ConsentAcceptRequest) (*dto.AuthCodeRedirect, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Op("oauth.consent.accept"))

	// 1. Validate Input
	if strings.TrimSpace(req.Token) == "" {
		return nil, ErrConsentMissingToken
	}

	// 2. Consume Token (One-Shot)
	key := "consent:token:" + strings.TrimSpace(req.Token)
	raw, ok := s.cache.Get(key)
	if !ok {
		return nil, ErrConsentNotFound
	}
	s.cache.Delete(key)

	var payload dto.ConsentChallenge
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Error("consent payload corrupted", logger.Err(err))
		return nil, ErrConsentNotFound
	}

	// Double check expiry
	if time.Now().After(payload.ExpiresAt) {
		return nil, ErrConsentNotFound
	}

	// 3. Handle Rejection
	if !req.Approve {
		loc := buildRedirect(payload.RedirectURI, map[string]string{
			"error": "access_denied",
			"state": payload.State,
		})
		return &dto.AuthCodeRedirect{URL: loc}, nil
	}

	// 4. Handle Approval
	// Resolve Tenant Data Access
	// Note: payload.TenantID refers to tenant slug in V2 context (from Authorize service)
	tda, err := s.dal.ForTenant(ctx, payload.TenantID)
	if err != nil {
		log.Error("failed to resolve tenant for consent", logger.Err(err), logger.String("tid", payload.TenantID))
		return nil, ErrConsentStoreFailed
	}

	// Persist Consent using repository
	_, err = tda.Consents().Upsert(ctx, payload.TenantID, payload.UserID, payload.ClientID, payload.RequestedScopes)
	if err != nil {
		log.Error("failed to upsert consent", logger.Err(err))
		return nil, ErrConsentStoreFailed
	}

	// 5. Issue Code
	code, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		log.Error("failed to generate code", logger.Err(err))
		return nil, ErrConsentCodeFailed
	}

	authPayload := AuthCodePayload{
		UserID:          payload.UserID,
		ClientID:        payload.ClientID,
		TenantID:        payload.TenantID,
		RedirectURI:     payload.RedirectURI,
		Scope:           strings.Join(payload.RequestedScopes, " "),
		Nonce:           payload.Nonce,
		CodeChallenge:   payload.CodeChallenge,
		ChallengeMethod: payload.CodeChallengeMethod,
		AMR:             payload.AMR,
		ExpiresAt:       time.Now().Add(10 * time.Minute), // Match V2 TTL
	}

	authBytes, _ := json.Marshal(authPayload)

	// Note: Store hashed code (hardening)
	// V2 TokenService expects "code:<hash>" (or "code:<code>" fallback)
	codeHash := tokens.SHA256Base64URL(code)
	s.cache.Set("code:"+codeHash, authBytes, 10*time.Minute)

	// 6. Return Redirect
	loc := buildRedirect(payload.RedirectURI, map[string]string{
		"code":  code,
		"state": payload.State,
	})

	return &dto.AuthCodeRedirect{URL: loc}, nil
}

// buildRedirect constructs the URL safely.
func buildRedirect(base string, params map[string]string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base // fallback
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
