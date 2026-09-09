package referral

import "time"

// Publisher represents a partner who refers schools to EduPlexo.
type Publisher struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	ReferralToken string     `json:"referral_token"`
	ReferralURL   string     `json:"referral_url,omitempty"`
	Status        string     `json:"status"` // active, suspended, deleted
	ReferredCount int        `json:"referred_schools_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// ReferredSchool represents a school referred by a publisher.
type ReferredSchool struct {
	ID            string    `json:"id"`
	SchoolID      string    `json:"school_id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	ContactEmail  string    `json:"contact_email"`
	ContactPhone  string    `json:"contact_phone"`
	AdminName     string    `json:"admin_name,omitempty"`
	AdminEmail    string    `json:"admin_email,omitempty"`
	LoginPassword string    `json:"login_password,omitempty"`
	LoginURL      string    `json:"login_url,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreatePublisherRequest defines payload for creating a new publisher by super admin.
type CreatePublisherRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Status   string `json:"status,omitempty"`
}

// UpdatePublisherRequest defines payload for updating a publisher.
type UpdatePublisherRequest struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}
