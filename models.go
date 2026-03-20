package main

import "time"

// Tab represents a single clipboard tab with its content and metadata.
// This struct is serialized to JSON and stored in BoltDB.
type Tab struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
