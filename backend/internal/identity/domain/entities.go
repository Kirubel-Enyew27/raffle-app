package domain

import "time"

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	FullName     string     `json:"full_name,omitempty"`
	AvatarURL    string     `json:"avatar_url,omitempty"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	IsBanned     bool       `json:"is_banned"`
	BanReason    *string    `json:"ban_reason,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
