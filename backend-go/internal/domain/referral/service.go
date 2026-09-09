package referral

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTokenInvalid       = errors.New("referral token is invalid or inactive")
	ErrPublisher          = errors.New("publisher not found or inactive")
	ErrEmailTaken         = errors.New("a publisher with this email already exists")
	ErrPublisherSuspended = errors.New("publisher account has been suspended")
	ErrSchoolNotReferred  = errors.New("school not found or not referred by this partner")
)

const tokenCharset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" // unambiguous characters (no 0, O, 1, I)

// GenerateReferralToken creates a collision-resistant, human-friendly referral token e.g. PUB_7X92KLMQ
func GenerateReferralToken() string {
	b := make([]byte, 8)
	charsetLen := big.NewInt(int64(len(tokenCharset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			b[i] = tokenCharset[i%len(tokenCharset)]
		} else {
			b[i] = tokenCharset[idx.Int64()]
		}
	}
	return "PUB_" + string(b)
}

// GeneratePublisherID generates an ID with prefix pub_
func GeneratePublisherID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("pub_%d", time.Now().UnixNano()%100000000)
	}
	return "pub_" + hex.EncodeToString(b)
}

// GetAppPublicURL returns the public base URL for school registration links.
func GetAppPublicURL() string {
	url := strings.TrimSpace(os.Getenv("APP_PUBLIC_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("VITE_PUBLIC_URL"))
	}
	if url == "" {
		url = "https://app.eduplexo.com"
	}
	return strings.TrimRight(url, "/")
}

// BuildReferralURL builds the full school registration URL with referral token.
func BuildReferralURL(token string) string {
	return fmt.Sprintf("%s/auth/register?ref=%s", GetAppPublicURL(), token)
}

// CreatePublisher provisions a new publisher account with credentials and a unique referral token.
func CreatePublisher(ctx context.Context, pool *pgxpool.Pool, req CreatePublisherRequest) (*Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if name == "" || email == "" || password == "" {
		return nil, errors.New("name, email, and password are required")
	}

	// Check email uniqueness across publishers
	var existingID string
	err := pool.QueryRow(ctx, "SELECT id FROM publishers WHERE email = $1 AND deleted_at IS NULL", email).Scan(&existingID)
	if err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Hash password
	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate collision-resistant unique referral token
	var token string
	for attempts := 0; attempts < 5; attempts++ {
		candidate := GenerateReferralToken()
		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM publishers WHERE referral_token = $1", candidate).Scan(&count)
		if count == 0 {
			token = candidate
			break
		}
	}
	if token == "" {
		token = GenerateReferralToken()
	}

	status := "active"
	if req.Status == "suspended" {
		status = "suspended"
	}

	id := GeneratePublisherID()
	now := time.Now()

	_, err = pool.Exec(ctx, `
		INSERT INTO publishers (id, name, email, password_hash, referral_token, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, name, email, string(pwHash), token, status, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert publisher: %w", err)
	}

	pub := &Publisher{
		ID:            id,
		Name:          name,
		Email:         email,
		ReferralToken: token,
		ReferralURL:   BuildReferralURL(token),
		Status:        status,
		ReferredCount: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return pub, nil
}

// GetPublisherByID returns a publisher by ID.
func GetPublisherByID(ctx context.Context, pool *pgxpool.Pool, id string) (*Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	var p Publisher
	var refCount int
	err := pool.QueryRow(ctx, `
		SELECT p.id, p.name, COALESCE(p.email::text, ''), COALESCE(p.password_hash, ''), COALESCE(p.referral_token, ''),
		       p.status, p.created_at, p.updated_at, p.deleted_at,
		       (SELECT COUNT(*) FROM schools s WHERE s.referred_by_publisher_id = p.id) as ref_count
		FROM publishers p
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`, id).Scan(
		&p.ID, &p.Name, &p.Email, &p.PasswordHash, &p.ReferralToken,
		&p.Status, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &refCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPublisher
		}
		return nil, err
	}

	p.ReferredCount = refCount
	p.ReferralURL = BuildReferralURL(p.ReferralToken)
	return &p, nil
}

// GetPublisherByEmail returns a publisher by Email.
func GetPublisherByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	var p Publisher
	var refCount int
	err := pool.QueryRow(ctx, `
		SELECT p.id, p.name, COALESCE(p.email::text, ''), COALESCE(p.password_hash, ''), COALESCE(p.referral_token, ''),
		       p.status, p.created_at, p.updated_at, p.deleted_at,
		       (SELECT COUNT(*) FROM schools s WHERE s.referred_by_publisher_id = p.id) as ref_count
		FROM publishers p
		WHERE p.email = $1 AND p.deleted_at IS NULL
	`, email).Scan(
		&p.ID, &p.Name, &p.Email, &p.PasswordHash, &p.ReferralToken,
		&p.Status, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &refCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPublisher
		}
		return nil, err
	}

	p.ReferredCount = refCount
	p.ReferralURL = BuildReferralURL(p.ReferralToken)
	return &p, nil
}

// GetPublisherByToken returns an active publisher associated with a referral token.
func GetPublisherByToken(ctx context.Context, pool *pgxpool.Pool, token string) (*Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrTokenInvalid
	}

	var p Publisher
	err := pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(email::text, ''), COALESCE(referral_token, ''), status, created_at, updated_at
		FROM publishers
		WHERE referral_token = $1 AND deleted_at IS NULL
	`, token).Scan(
		&p.ID, &p.Name, &p.Email, &p.ReferralToken, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}

	if p.Status != "active" {
		return nil, ErrPublisherSuspended
	}

	p.ReferralURL = BuildReferralURL(p.ReferralToken)
	return &p, nil
}

// ListPublishers returns all active/suspended publishers for super admin with referral counts.
func ListPublishers(ctx context.Context, pool *pgxpool.Pool, search string) ([]Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	search = strings.TrimSpace(search)
	var query string
	var args []any

	if search != "" {
		query = `
			SELECT p.id, p.name, COALESCE(p.email::text, ''), COALESCE(p.referral_token, ''),
			       p.status, p.created_at, p.updated_at,
			       (SELECT COUNT(*) FROM schools s WHERE s.referred_by_publisher_id = p.id) as ref_count
			FROM publishers p
			WHERE p.deleted_at IS NULL AND (
				p.name ILIKE $1 OR p.email::text ILIKE $1 OR p.referral_token ILIKE $1
			)
			ORDER BY p.created_at DESC
		`
		args = append(args, "%"+search+"%")
	} else {
		query = `
			SELECT p.id, p.name, COALESCE(p.email::text, ''), COALESCE(p.referral_token, ''),
			       p.status, p.created_at, p.updated_at,
			       (SELECT COUNT(*) FROM schools s WHERE s.referred_by_publisher_id = p.id) as ref_count
			FROM publishers p
			WHERE p.deleted_at IS NULL
			ORDER BY p.created_at DESC
		`
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Publisher
	for rows.Next() {
		var p Publisher
		var refCount int
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.ReferralToken, &p.Status, &p.CreatedAt, &p.UpdatedAt, &refCount); err == nil {
			p.ReferredCount = refCount
			p.ReferralURL = BuildReferralURL(p.ReferralToken)
			result = append(result, p)
		}
	}

	if result == nil {
		result = []Publisher{}
	}

	return result, nil
}

// UpdatePublisher updates publisher details (name, email, or password).
func UpdatePublisher(ctx context.Context, pool *pgxpool.Pool, id string, req UpdatePublisherRequest) (*Publisher, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	existing, err := GetPublisherByID(ctx, pool, id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = existing.Email
	} else if email != existing.Email {
		var otherID string
		err := pool.QueryRow(ctx, "SELECT id FROM publishers WHERE email = $1 AND id != $2 AND deleted_at IS NULL", email, id).Scan(&otherID)
		if err == nil {
			return nil, ErrEmailTaken
		}
	}

	pwHash := existing.PasswordHash
	if strings.TrimSpace(req.Password) != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		pwHash = string(h)
	}

	now := time.Now()
	_, err = pool.Exec(ctx, `
		UPDATE publishers 
		SET name = $1, email = $2, password_hash = $3, updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
	`, name, email, pwHash, now, id)
	if err != nil {
		return nil, err
	}

	return GetPublisherByID(ctx, pool, id)
}

// SetPublisherStatus changes publisher status to active or suspended.
func SetPublisherStatus(ctx context.Context, pool *pgxpool.Pool, id string, status string) error {
	if pool == nil {
		return errors.New("database not available")
	}

	if status != "active" && status != "suspended" {
		return errors.New("invalid status value")
	}

	cmd, err := pool.Exec(ctx, `
		UPDATE publishers 
		SET status = $1, updated_at = NOW() 
		WHERE id = $2 AND deleted_at IS NULL
	`, status, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrPublisher
	}
	return nil
}

// SoftDeletePublisher marks a publisher as deleted while preserving historical school referral records.
func SoftDeletePublisher(ctx context.Context, pool *pgxpool.Pool, id string) error {
	if pool == nil {
		return errors.New("database not available")
	}

	cmd, err := pool.Exec(ctx, `
		UPDATE publishers 
		SET status = 'deleted', deleted_at = NOW(), updated_at = NOW() 
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrPublisher
	}
	return nil
}

// ListReferredSchools returns all schools attributed to a publisher with admin login details.
func ListReferredSchools(ctx context.Context, pool *pgxpool.Pool, publisherID string) ([]ReferredSchool, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	loginURL := fmt.Sprintf("%s/auth/login", GetAppPublicURL())

	rows, err := pool.Query(ctx, `
		SELECT 
			s.id, 
			s.school_id, 
			s.name, 
			s.code, 
			COALESCE(s.contact_email::text, ''), 
			COALESCE(s.contact_phone, s.admin_phone, ''), 
			COALESCE(NULLIF(s.admin_name, ''), NULLIF(TRIM(u.profile_first || ' ' || u.profile_last), ''), ''),
			COALESCE(NULLIF(s.admin_email::text, ''), NULLIF(u.email::text, ''), NULLIF(s.contact_email::text, ''), ''),
			COALESCE(s.referral_admin_password, ''),
			s.status, 
			s.created_at
		FROM schools s
		LEFT JOIN LATERAL (
			SELECT email, profile_first, profile_last 
			FROM users 
			WHERE (school_id = s.school_id OR school_id = s.id) AND role = 'admin' 
			ORDER BY created_at ASC 
			LIMIT 1
		) u ON true
		WHERE s.referred_by_publisher_id = $1
		ORDER BY s.created_at DESC
	`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ReferredSchool
	for rows.Next() {
		var s ReferredSchool
		if err := rows.Scan(
			&s.ID, &s.SchoolID, &s.Name, &s.Code,
			&s.ContactEmail, &s.ContactPhone,
			&s.AdminName, &s.AdminEmail, &s.LoginPassword,
			&s.Status, &s.CreatedAt,
		); err == nil {
			s.LoginURL = loginURL
			result = append(result, s)
		}
	}

	if result == nil {
		result = []ReferredSchool{}
	}

	return result, nil
}

// GetReferredSchoolByID returns a single school attributed to the publisher with its admin login credentials.
// Strictly enforces publisher tenancy.
func GetReferredSchoolByID(ctx context.Context, pool *pgxpool.Pool, publisherID string, schoolID string) (*ReferredSchool, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	loginURL := fmt.Sprintf("%s/auth/login", GetAppPublicURL())

	var s ReferredSchool
	err := pool.QueryRow(ctx, `
		SELECT 
			s.id, 
			s.school_id, 
			s.name, 
			s.code, 
			COALESCE(s.contact_email::text, ''), 
			COALESCE(s.contact_phone, s.admin_phone, ''), 
			COALESCE(NULLIF(s.admin_name, ''), NULLIF(TRIM(u.profile_first || ' ' || u.profile_last), ''), ''),
			COALESCE(NULLIF(s.admin_email::text, ''), NULLIF(u.email::text, ''), NULLIF(s.contact_email::text, ''), ''),
			COALESCE(s.referral_admin_password, ''),
			s.status, 
			s.created_at
		FROM schools s
		LEFT JOIN LATERAL (
			SELECT email, profile_first, profile_last 
			FROM users 
			WHERE (school_id = s.school_id OR school_id = s.id) AND role = 'admin' 
			ORDER BY created_at ASC 
			LIMIT 1
		) u ON true
		WHERE (s.id = $1 OR s.school_id = $1) AND s.referred_by_publisher_id = $2
		LIMIT 1
	`, schoolID, publisherID).Scan(
		&s.ID, &s.SchoolID, &s.Name, &s.Code,
		&s.ContactEmail, &s.ContactPhone,
		&s.AdminName, &s.AdminEmail, &s.LoginPassword,
		&s.Status, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSchoolNotReferred
		}
		return nil, err
	}

	s.LoginURL = loginURL
	return &s, nil
}

// UpdateReferredSchoolPassword sets a new password for the school admin user.
// Strictly verifies that the school is attributed to the calling publisher.
func UpdateReferredSchoolPassword(ctx context.Context, pool *pgxpool.Pool, publisherID string, schoolID string, newPassword string, onPasswordUpdated func(schoolID string, newPassword string, pwHash string)) (*ReferredSchool, error) {
	if pool == nil {
		return nil, errors.New("database not available")
	}

	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 {
		return nil, errors.New("password must be at least 8 characters long")
	}

	// Verify ownership
	var realSchoolID, currentAdminEmail string
	err := pool.QueryRow(ctx, `
		SELECT s.school_id, COALESCE(s.admin_email::text, '')
		FROM schools s
		WHERE (s.id = $1 OR s.school_id = $1) AND s.referred_by_publisher_id = $2
	`, schoolID, publisherID).Scan(&realSchoolID, &currentAdminEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSchoolNotReferred
		}
		return nil, err
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()

	// Update schools table with referral_admin_password
	_, err = pool.Exec(ctx, `
		UPDATE schools 
		SET referral_admin_password = $1, updated_at = $2
		WHERE school_id = $3
	`, newPassword, now, realSchoolID)
	if err != nil {
		return nil, fmt.Errorf("failed to update school password: %w", err)
	}

	// Update users table admin password hash
	cmd, err := pool.Exec(ctx, `
		UPDATE users 
		SET password_hash = $1, updated_at = $2
		WHERE school_id = $3 AND role = 'admin'
	`, string(pwHash), now, realSchoolID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user password: %w", err)
	}

	if onPasswordUpdated != nil {
		onPasswordUpdated(realSchoolID, newPassword, string(pwHash))
	}

	_ = cmd

	return GetReferredSchoolByID(ctx, pool, publisherID, schoolID)
}
