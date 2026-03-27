package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a PostgreSQL connection pool and provides CRUD operations.
type Store struct {
	pool          *pgxpool.Pool
	DefaultUserID int // used when AUTH_DISABLED is set
}

const migrateSQL = `
CREATE TABLE IF NOT EXISTS tabs (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT 'New Tab',
    content     TEXT NOT NULL DEFAULT '',
    tab_order   INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auth_tokens (
    id         SERIAL PRIMARY KEY,
    email      TEXT NOT NULL,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE tabs ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id);
`

// NewStore opens a pgxpool connection and ensures the schema exists.
func NewStore(databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if _, err := pool.Exec(context.Background(), migrateSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	s := &Store{pool: pool}

	// When auth is disabled, ensure a default user exists.
	if os.Getenv("AUTH_DISABLED") != "" {
		id, err := s.UpsertUser("default@clipd.local")
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("creating default user: %w", err)
		}
		s.DefaultUserID = id
		log.Printf("Auth disabled — using default user (id=%d)", id)
	}

	return s, nil
}

// Close shuts down the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// generateToken creates a cryptographically random hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ── Auth storage ────────────────────────────────────────────────────────────

// CreateAuthToken stores a magic link token for the given email.
func (s *Store) CreateAuthToken(email string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	_, err = s.pool.Exec(context.Background(),
		`INSERT INTO auth_tokens (email, token, expires_at) VALUES ($1, $2, $3)`,
		email, token, expiresAt)
	return token, err
}

// ValidateAuthToken checks a token, marks it used, and returns the email.
func (s *Store) ValidateAuthToken(token string) (string, error) {
	var email string
	err := s.pool.QueryRow(context.Background(),
		`UPDATE auth_tokens SET used = TRUE
		 WHERE token = $1 AND used = FALSE AND expires_at > NOW()
		 RETURNING email`, token).Scan(&email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("token invalid or expired")
		}
		return "", err
	}
	return email, nil
}

// UpsertUser creates a user if they don't exist, returns the user ID.
func (s *Store) UpsertUser(email string) (int, error) {
	var id int
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO users (email) VALUES ($1)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`, email).Scan(&id)
	return id, err
}

// CreateSession creates a new session for a user, returns the session token.
func (s *Store) CreateSession(userID int) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, err = s.pool.Exec(context.Background(),
		`INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, expiresAt)
	return token, err
}

// ValidateSession looks up a session token, refreshes its expiry, and returns the user ID.
func (s *Store) ValidateSession(token string) (int, error) {
	var userID int
	err := s.pool.QueryRow(context.Background(),
		`UPDATE sessions SET expires_at = NOW() + INTERVAL '30 days'
		 WHERE token = $1 AND expires_at > NOW()
		 RETURNING user_id`, token).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("session invalid or expired")
		}
		return 0, err
	}
	return userID, nil
}

// DeleteSession removes a session by token.
func (s *Store) DeleteSession(token string) error {
	_, err := s.pool.Exec(context.Background(),
		`DELETE FROM sessions WHERE token = $1`, token)
	return err
}

// GetUserEmail returns the email for a user ID.
func (s *Store) GetUserEmail(userID int) (string, error) {
	var email string
	err := s.pool.QueryRow(context.Background(),
		`SELECT email FROM users WHERE id = $1`, userID).Scan(&email)
	return email, err
}

// Cleanup removes expired tokens and sessions.
func (s *Store) Cleanup() {
	ctx := context.Background()
	s.pool.Exec(ctx, "DELETE FROM auth_tokens WHERE expires_at < NOW()")
	s.pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at < NOW()")
}

// ── Tab storage (user-scoped) ───────────────────────────────────────────────

// ListTabs returns all tabs for a user sorted by tab_order then id.
func (s *Store) ListTabs(userID int) ([]Tab, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, name, content, tab_order, created_at, updated_at
		 FROM tabs
		 WHERE user_id = $1
		 ORDER BY tab_order ASC, id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tabs []Tab
	for rows.Next() {
		var t Tab
		if err := rows.Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tabs = append(tabs, t)
	}
	return tabs, rows.Err()
}

// GetTab retrieves a single tab by ID, scoped to a user.
func (s *Store) GetTab(userID, id int) (*Tab, error) {
	var t Tab
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, name, content, tab_order, created_at, updated_at
		 FROM tabs WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tab not found")
		}
		return nil, err
	}
	return &t, nil
}

// CreateTab inserts a new tab at the end of the order for a user.
func (s *Store) CreateTab(userID int, name string) (*Tab, error) {
	var maxOrder int
	err := s.pool.QueryRow(context.Background(),
		`SELECT COALESCE(MAX(tab_order), -1) FROM tabs WHERE user_id = $1`, userID).Scan(&maxOrder)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var t Tab
	err = s.pool.QueryRow(context.Background(),
		`INSERT INTO tabs (name, content, tab_order, user_id, created_at, updated_at)
		 VALUES ($1, '', $2, $3, $4, $4)
		 RETURNING id, name, content, tab_order, created_at, updated_at`,
		name, maxOrder+1, userID, now).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	return &t, err
}

// UpdateTab updates a tab's name and content, scoped to a user.
func (s *Store) UpdateTab(userID, id int, name, content string) (*Tab, error) {
	now := time.Now().UTC()
	var t Tab
	err := s.pool.QueryRow(context.Background(),
		`UPDATE tabs
		 SET name = $1, content = $2, updated_at = $3
		 WHERE id = $4 AND user_id = $5
		 RETURNING id, name, content, tab_order, created_at, updated_at`,
		name, content, now, id, userID).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tab not found")
		}
		return nil, err
	}
	return &t, nil
}

// DeleteTab removes a tab by ID, scoped to a user.
func (s *Store) DeleteTab(userID, id int) error {
	cmd, err := s.pool.Exec(context.Background(),
		`DELETE FROM tabs WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("tab not found")
	}
	return nil
}

// ReorderTabs sets tab_order for each tab based on the provided ordered list of IDs, scoped to a user.
func (s *Store) ReorderTabs(userID int, ids []int) error {
	for i, id := range ids {
		cmd, err := s.pool.Exec(context.Background(),
			`UPDATE tabs SET tab_order = $1 WHERE id = $2 AND user_id = $3`, i, id, userID)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return fmt.Errorf("tab %d not found", id)
		}
	}
	return nil
}
