package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"time"

	"github.com/google/uuid"
)

type TokenKind int

const (
	TokenEmailVerify TokenKind = iota + 1
	TokenPasswordReset
)

type TokenStore struct {
	DB DBOps
}

func NewTokenStore(db DBOps) *TokenStore { return &TokenStore{DB: db} }

func generate() (plaintext string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(plaintext))
	return plaintext, h[:], nil
}

// Guardamos el hash SHA-256 crudo (BYTEA); ip/ua usan NULLIF para evitar ""
// tenantID is kept in function signature for API compatibility but not stored in DB (per-tenant DB)
func (s *TokenStore) CreateEmailVerification(ctx context.Context, tenantID, userID uuid.UUID, sentTo string, ttl time.Duration, ip, ua *string) (string, error) {
	pt, h, err := generate()
	if err != nil {
		return "", err
	}
	hashBytes := h
	exp := time.Now().Add(ttl)

	log.Printf(`{"level":"debug","msg":"db_insert_email_verif_token_try","user_id":"%s","sent_to":"%s","exp":"%s"}`, userID, sentTo, exp.Format(time.RFC3339))
	_, err = s.DB.Exec(ctx, `
		INSERT INTO email_verification_token
		    (user_id, token_hash, sent_to, ip, user_agent, expires_at)
		VALUES ($1,      $2,         $3,  NULLIF($4,'')::inet, NULLIF($5,'')::text, $6)`,
		userID, hashBytes, sentTo, ip, ua, exp,
	)
	if err != nil {
		log.Printf(`{"level":"error","msg":"db_insert_email_verif_token_err","err":"%v"}`, err)
		return "", err
	}
	log.Printf(`{"level":"debug","msg":"db_insert_email_verif_token_ok"}`)
	return pt, nil
}

// UseEmailVerification returns only userID since DB is per-tenant (no tenant_id column)
func (s *TokenStore) UseEmailVerification(ctx context.Context, plaintext string) (tenantID, userID uuid.UUID, err error) {
	h := sha256.Sum256([]byte(plaintext))
	hashBytes := h[:]

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	log.Printf(`{"level":"debug","msg":"db_use_email_verif_try"}`)
	row := tx.QueryRow(ctx, `
		UPDATE email_verification_token
		   SET used_at = now()
		 WHERE token_hash = $1
		   AND used_at IS NULL
		   AND expires_at > now()
		RETURNING user_id`, hashBytes,
	)
	if err = row.Scan(&userID); err != nil {
		log.Printf(`{"level":"warn","msg":"db_use_email_verif_not_found","err":"%v"}`, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		log.Printf(`{"level":"error","msg":"db_use_email_verif_commit_err","err":"%v"}`, err)
		return
	}
	// tenantID remains uuid.Nil, returned as part of signature but unused
	log.Printf(`{"level":"debug","msg":"db_use_email_verif_ok","user_id":"%s"}`, userID)
	return
}

func (s *TokenStore) CreatePasswordReset(ctx context.Context, tenantID, userID uuid.UUID, sentTo string, ttl time.Duration, ip, ua *string) (string, error) {
	pt, h, err := generate()
	if err != nil {
		return "", err
	}
	hashBytes := h
	exp := time.Now().Add(ttl)

	log.Printf(`{"level":"debug","msg":"db_insert_pwd_reset_try","tenant_id":"%s","user_id":"%s","sent_to":"%s","exp":"%s"}`, tenantID, userID, sentTo, exp.Format(time.RFC3339))
	_, err = s.DB.Exec(ctx, `
		INSERT INTO password_reset_token
		    (user_id, token_hash, sent_to, ip, user_agent, expires_at)
		VALUES ($1,      $2,         $3,  NULLIF($4,'')::inet, NULLIF($5,'')::text, $6)`,
		userID, hashBytes, sentTo, ip, ua, exp,
	)
	if err != nil {
		log.Printf(`{"level":"error","msg":"db_insert_pwd_reset_err","err":"%v"}`, err)
		return "", err
	}
	log.Printf(`{"level":"debug","msg":"db_insert_pwd_reset_ok"}`)
	return pt, nil
}

func (s *TokenStore) UsePasswordReset(ctx context.Context, plaintext string) (tenantID, userID uuid.UUID, err error) {
	h := sha256.Sum256([]byte(plaintext))
	hashBytes := h[:]

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	log.Printf(`{"level":"debug","msg":"db_use_pwd_reset_try"}`)
	row := tx.QueryRow(ctx, `
		UPDATE password_reset_token
		   SET used_at = now()
		 WHERE token_hash = $1
		   AND used_at IS NULL
		   AND expires_at > now()
		RETURNING user_id`, hashBytes,
	)
	if err = row.Scan(&userID); err != nil {
		log.Printf(`{"level":"warn","msg":"db_use_pwd_reset_not_found","err":"%v"}`, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		log.Printf(`{"level":"error","msg":"db_use_pwd_reset_commit_err","err":"%v"}`, err)
		return
	}
	// tenantID remains nil, callers should know context or lookup tenant via other means if needed
	log.Printf(`{"level":"debug","msg":"db_use_pwd_reset_ok","user_id":"%s"}`, userID)
	return
}
