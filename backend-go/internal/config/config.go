// Package config loads runtime configuration from environment variables.
// Mirrors the small env file in old-app/school-app/config/env.ts plus the
// few env reads scattered through the original Node services.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config is the validated runtime config for the Go backend.
type Config struct {
	// Environment controls production validation. Set APP_ENV=production for
	// deployed environments.
	Environment string

	// Port the HTTP server listens on. Default 8080 to match the docker-compose
	// expectation called out in the migration prompt.
	Port string

	// JWTSecret is HMAC-SHA256 secret used to sign and verify session tokens.
	// MUST match the secret of any environment that issued tokens we accept.
	JWTSecret string

	// AppName matches the `app` claim of issued tokens. The Node backend uses
	// "school" for school-app tokens — preserve that exact value so existing
	// frontend tokens continue to validate.
	AppName string

	// AllowedOrigins is the CORS allowlist. The frontend dev server runs on
	// http://localhost:3000.
	AllowedOrigins []string

	// CookieSecure controls the Secure flag on the session cookie. Off in dev.
	CookieSecure bool

	// DatabaseURL is the PostgreSQL DSN. Leave empty to run in pure
	// in-memory mode (development convenience only — no durability).
	DatabaseURL string

	// RedisURL is the Redis connection string (e.g. "redis://redis:6379/0").
	// Leave empty to run without caching — the cache layer degrades gracefully.
	RedisURL string

	// UseDirectPG enables direct PostgreSQL queries instead of MemStore.
	// Set to "true" to bypass MemStore entirely. Can also be controlled
	// per-entity via USE_DIRECT_PG_STUDENTS, USE_DIRECT_PG_TEACHERS, etc.
	UseDirectPG bool

	// GeminiAPIKey for the AI School Assistant (Google Gemini).
	GeminiAPIKey    string
	GeminiModel     string
	GeminiTimeoutMs int

	// AnthropicAPIKey for the SEO Engine (Claude).
	AnthropicAPIKey string

	// Brevo Transactional Email Configuration
	BrevoAPIKey        string
	BrevoSenderEmail   string
	BrevoSenderName    string
	BrevoOTPTemplateID int64
	BrevoReplyToEmail  string
	BrevoReplyToName   string

	// Email OTP Configuration
	EmailOTPLength                 int
	EmailOTPExpirySeconds          int
	EmailOTPResendCooldownSeconds  int
	EmailOTPMaxVerifyAttempts      int
	EmailOTPMaxSendAttemptsPerHour int

	// MetricsToken authenticates access to the Prometheus /metrics endpoint.
	// Sent via ?token= or Authorization: Bearer. Empty means unauthenticated
	// (development only); production validation requires it to be set.
	MetricsToken string

	// AllowWSTokenQuery permits the legacy /ws?token=... handshake. Off in
	// production: long-lived JWTs must never ride in URLs (logs, history,
	// referrers, proxies).
	AllowWSTokenQuery bool
}

// Load reads env vars, applies local-development defaults, and validates
// production-only requirements.
func Load() (Config, error) {
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	cfg := Config{
		Environment:                    getenv("APP_ENV", getenv("ENV", "development")),
		Port:                           getenv("PORT", "8080"),
		JWTSecret:                      os.Getenv("JWT_SECRET"),
		AppName:                        getenv("APP_NAME", "school"),
		AllowedOrigins:                 append(splitCSV(getenv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:3002,http://localhost:3003,http://localhost:3004,http://localhost:5173")), "https://app.eduplexo.com", "https://admin.eduplexo.com", "https://partners.eduplexo.com", "https://www.eduplexo.com"),
		CookieSecure:                   os.Getenv("COOKIE_SECURE") == "true",
		DatabaseURL:                    os.Getenv("DATABASE_URL"),
		RedisURL:                       os.Getenv("REDIS_URL"),
		UseDirectPG:                    os.Getenv("USE_DIRECT_PG") == "true",
		GeminiAPIKey:                   os.Getenv("GEMINI_API_KEY"),
		GeminiModel:                    getenv("GEMINI_MODEL", "gemini-2.0-flash"),
		GeminiTimeoutMs:                2500,
		AnthropicAPIKey:                os.Getenv("ANTHROPIC_API_KEY"),
		BrevoAPIKey:                    os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail:               getenv("BREVO_SENDER_EMAIL", "verify@eduplexo.com"),
		BrevoSenderName:                getenv("BREVO_SENDER_NAME", "EduPlexo"),
		BrevoOTPTemplateID:             parseInt64(os.Getenv("BREVO_OTP_TEMPLATE_ID")),
		BrevoReplyToEmail:              getenv("BREVO_REPLY_TO_EMAIL", "support@eduplexo.com"),
		BrevoReplyToName:               getenv("BREVO_REPLY_TO_NAME", "EduPlexo Support"),
		EmailOTPLength:                 parseIntDefault(os.Getenv("EMAIL_OTP_LENGTH"), 6),
		EmailOTPExpirySeconds:          parseIntDefault(os.Getenv("EMAIL_OTP_EXPIRY_SECONDS"), 300),
		EmailOTPResendCooldownSeconds:  parseIntDefault(os.Getenv("EMAIL_OTP_RESEND_COOLDOWN_SECONDS"), 60),
		EmailOTPMaxVerifyAttempts:      parseIntDefault(os.Getenv("EMAIL_OTP_MAX_VERIFY_ATTEMPTS"), 5),
		EmailOTPMaxSendAttemptsPerHour: parseIntDefault(os.Getenv("EMAIL_OTP_MAX_SEND_ATTEMPTS_PER_HOUR"), 5),
		MetricsToken:                   os.Getenv("METRICS_TOKEN"),
		AllowWSTokenQuery:              os.Getenv("ALLOW_WS_TOKEN_QUERY") == "true",
	}

	if err := cfg.Validate(allowedOriginsEnv); err != nil {
		return Config{}, err
	}

	if cfg.JWTSecret == "" {
		// Development-only fallback: a RANDOM per-boot secret. Using a
		// hard-coded compile-time string would let anyone who reads the repo
		// forge JWTs for any dev/CI instance. With a random secret, dev tokens
		// die on restart and can never be forged from code alone.
		buf := make([]byte, 48)
		if _, err := rand.Read(buf); err != nil {
			return Config{}, fmt.Errorf("generate ephemeral dev JWT secret: %w", err)
		}
		cfg.JWTSecret = base64.RawStdEncoding.EncodeToString(buf)
		log.Println("[config] WARNING: JWT_SECRET is empty; generated a random per-boot development secret (sessions do not survive restart). Set JWT_SECRET for any shared environment.")
	} else if len(cfg.JWTSecret) < 32 {
		log.Printf("[config] WARNING: JWT_SECRET is only %d characters; HS256 signing keys should be at least 32 random bytes (use `openssl rand -base64 48`).", len(cfg.JWTSecret))
	}

	return cfg, nil
}

// Validate enforces fail-closed settings for production while preserving local
// development compatibility.
func (cfg Config) Validate(allowedOriginsEnv string) error {
	if !cfg.IsProduction() {
		return nil
	}

	missing := []string{}
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if strings.TrimSpace(allowedOriginsEnv) == "" {
		missing = append(missing, "ALLOWED_ORIGINS")
	}
	if strings.TrimSpace(cfg.BrevoAPIKey) == "" {
		missing = append(missing, "BREVO_API_KEY")
	}
	if strings.TrimSpace(cfg.BrevoSenderEmail) == "" {
		missing = append(missing, "BREVO_SENDER_EMAIL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("production config missing required env vars: %s", strings.Join(missing, ", "))
	}
	if strings.TrimSpace(cfg.MetricsToken) == "" {
		missing = append(missing, "METRICS_TOKEN")
		return fmt.Errorf("production config missing required env vars: %s", strings.Join(missing, ", "))
	}
	if cfg.AllowWSTokenQuery {
		return errors.New("production config forbids ALLOW_WS_TOKEN_QUERY=true: long-lived JWTs must never ride in WebSocket URLs")
	}
	if cfg.EmailOTPExpirySeconds != 300 {
		return fmt.Errorf("production EMAIL_OTP_EXPIRY_SECONDS must be exactly 300 (got %d)", cfg.EmailOTPExpirySeconds)
	}
	if !cfg.CookieSecure {
		return errors.New("production config requires COOKIE_SECURE=true")
	}
	for _, origin := range cfg.AllowedOrigins {
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") || strings.HasPrefix(origin, "http://") {
			return fmt.Errorf("production ALLOWED_ORIGINS contains insecure origin %q", origin)
		}
	}
	return nil
}

func (cfg Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(cfg.Environment))
	return env == "production" || env == "prod"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	out := []string{}
	current := ""
	for _, r := range s {
		if r == ',' {
			if current != "" {
				out = append(out, current)
				current = ""
			}
			continue
		}
		current += string(r)
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func parseIntDefault(v string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func parseInt64(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}
