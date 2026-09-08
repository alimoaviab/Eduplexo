package referral

import "time"

// Publisher represents a partner who refers schools to EduPlexo.
type Publisher struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Token represents a generated, one-time-use referral link.
type Token struct {
	ID                   string     `json:"id"`
	PublisherID          string     `json:"publisher_id"`
	TokenHash            string     `json:"-"`
	Status               string     `json:"status"` // UNUSED, USED, REVOKED
	PlanID               string     `json:"plan_id"`
	PlanNameSnapshot     string     `json:"plan_name_snapshot"`
	MonthlyPriceSnapshot float64    `json:"monthly_price_snapshot"`
	Currency             string     `json:"currency"`
	BillingPeriod        string     `json:"billing_period"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	UsedAt               *time.Time `json:"used_at,omitempty"`
	UsedBySchoolID       *string    `json:"used_by_school_id,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
}

// ReferralRecord logs the successful consumption of a token.
type ReferralRecord struct {
	ID                   string     `json:"id"`
	PublisherID          string     `json:"publisher_id"`
	ReferralTokenID      string     `json:"referral_token_id"`
	SchoolID             string     `json:"school_id"`
	PlanID               string     `json:"plan_id"`
	MonthlyPriceSnapshot float64    `json:"monthly_price_snapshot"`
	CommissionStatus     string     `json:"commission_status"`
	CommissionAmount     *float64   `json:"commission_amount,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}
