package main

import "time"

// Tab represents a single clipboard tab with its content and metadata.
// ID is an int to match PostgreSQL's SERIAL PRIMARY KEY type.
type Tab struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
