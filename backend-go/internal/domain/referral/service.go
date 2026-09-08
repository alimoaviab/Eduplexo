package referral

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTokenInvalid = errors.New("referral token is invalid or has already been used")
	ErrTokenExpired = errors.New("referral token has expired")
	ErrPublisher    = errors.New("publisher not found or inactive")
)

// GenerateRawToken creates a 32-byte secure random token and returns its hex string and SHA-256 hash.
func GenerateRawToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// HashToken returns the SHA-256 hash of a raw token.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// newID generates a simple prefixed ID (e.g., rtk_a1b2c3d4)
func newID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// GenerateTokenRequest holds the payload for generating a new token.
type GenerateTokenRequest struct {
	PublisherID          string
	PlanID               string
	PlanNameSnapshot     string
	MonthlyPriceSnapshot float64
	Currency             string
	BillingPeriod        string
	ExpiresAt            *time.Time
}

// GenerateToken creates a new token in the database and returns the raw link token (which should only be shown once).
func GenerateToken(ctx context.Context, pool *pgxpool.Pool, req GenerateTokenRequest) (string, *Token, error) {
	// Validate publisher
	var pubStatus string
	err := pool.QueryRow(ctx, "SELECT status FROM publishers WHERE id = $1", req.PublisherID).Scan(&pubStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrPublisher
		}
		return "", nil, err
	}
	if pubStatus != "active" {
		return "", nil, ErrPublisher
	}

	raw, hash, err := GenerateRawToken()
	if err != nil {
		return "", nil, err
	}

	token := &Token{
		ID:                   newID("rtk"),
		PublisherID:          req.PublisherID,
		TokenHash:            hash,
		Status:               "UNUSED",
		PlanID:               req.PlanID,
		PlanNameSnapshot:     req.PlanNameSnapshot,
		MonthlyPriceSnapshot: req.MonthlyPriceSnapshot,
		Currency:             req.Currency,
		BillingPeriod:        req.BillingPeriod,
		ExpiresAt:            req.ExpiresAt,
		CreatedAt:            time.Now(),
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO referral_tokens 
		(id, publisher_id, token_hash, status, plan_id, plan_name_snapshot, monthly_price_snapshot, currency, billing_period, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, token.ID, token.PublisherID, token.TokenHash, token.Status, token.PlanID, token.PlanNameSnapshot, token.MonthlyPriceSnapshot, token.Currency, token.BillingPeriod, token.ExpiresAt, token.CreatedAt)

	if err != nil {
		return "", nil, err
	}

	return raw, token, nil
}

// ValidateToken retrieves a token by its raw string, ensuring it is UNUSED and unexpired.
func ValidateToken(ctx context.Context, pool *pgxpool.Pool, rawToken string) (*Token, error) {
	hash := HashToken(strings.TrimSpace(rawToken))

	var t Token
	err := pool.QueryRow(ctx, `
		SELECT id, publisher_id, status, plan_id, plan_name_snapshot, monthly_price_snapshot, currency, billing_period, expires_at, created_at
		FROM referral_tokens
		WHERE token_hash = $1
	`, hash).Scan(
		&t.ID, &t.PublisherID, &t.Status, &t.PlanID, &t.PlanNameSnapshot,
		&t.MonthlyPriceSnapshot, &t.Currency, &t.BillingPeriod, &t.ExpiresAt, &t.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}

	if t.Status != "UNUSED" {
		return nil, ErrTokenInvalid
	}

	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return &t, nil
}
