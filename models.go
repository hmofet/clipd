package main

import "time"

// Tab represents a single clipboard tab with its content and metadata.
type Tab struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Order     int       `json:"order"`
	UserID    int       `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a registered user identified by email.
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthToken is a short-lived, one-time-use magic link token.
type AuthToken struct {
	ID        int
	Email     string
	Token     string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

// Session is a long-lived session tied to a user.
type Session struct {
	ID        int
	UserID    int
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}
