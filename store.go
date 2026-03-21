package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a PostgreSQL connection pool and provides CRUD operations for tabs.
type Store struct {
	pool *pgxpool.Pool
}

// createTableSQL bootstraps the schema on first run.
const createTableSQL = `
CREATE TABLE IF NOT EXISTS tabs (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT 'New Tab',
    content     TEXT NOT NULL DEFAULT '',
    tab_order   INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// NewStore opens a pgxpool connection and ensures the schema exists.
func NewStore(databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Verify the connection is alive.
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Create table on startup — no migration tool needed for this scope.
	if _, err := pool.Exec(context.Background(), createTableSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("creating table: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close shuts down the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// ListTabs returns all tabs sorted by tab_order then id.
func (s *Store) ListTabs() ([]Tab, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, name, content, tab_order, created_at, updated_at
		 FROM tabs
		 ORDER BY tab_order ASC, id ASC`)
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

// GetTab retrieves a single tab by ID.
func (s *Store) GetTab(id int) (*Tab, error) {
	var t Tab
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, name, content, tab_order, created_at, updated_at
		 FROM tabs WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tab not found")
		}
		return nil, err
	}
	return &t, nil
}

// CreateTab inserts a new tab at the end of the order.
func (s *Store) CreateTab(name string) (*Tab, error) {
	// Place the new tab after all existing tabs.
	var maxOrder int
	err := s.pool.QueryRow(context.Background(),
		`SELECT COALESCE(MAX(tab_order), -1) FROM tabs`).Scan(&maxOrder)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var t Tab
	err = s.pool.QueryRow(context.Background(),
		`INSERT INTO tabs (name, content, tab_order, created_at, updated_at)
		 VALUES ($1, '', $2, $3, $3)
		 RETURNING id, name, content, tab_order, created_at, updated_at`,
		name, maxOrder+1, now).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	return &t, err
}

// UpdateTab updates a tab's name and content.
func (s *Store) UpdateTab(id int, name, content string) (*Tab, error) {
	now := time.Now().UTC()
	var t Tab
	err := s.pool.QueryRow(context.Background(),
		`UPDATE tabs
		 SET name = $1, content = $2, updated_at = $3
		 WHERE id = $4
		 RETURNING id, name, content, tab_order, created_at, updated_at`,
		name, content, now, id).
		Scan(&t.ID, &t.Name, &t.Content, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tab not found")
		}
		return nil, err
	}
	return &t, nil
}

// DeleteTab removes a tab by ID.
func (s *Store) DeleteTab(id int) error {
	cmd, err := s.pool.Exec(context.Background(),
		`DELETE FROM tabs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("tab not found")
	}
	return nil
}

// ReorderTabs sets tab_order for each tab based on the provided ordered list of IDs.
func (s *Store) ReorderTabs(ids []int) error {
	for i, id := range ids {
		cmd, err := s.pool.Exec(context.Background(),
			`UPDATE tabs SET tab_order = $1 WHERE id = $2`, i, id)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return fmt.Errorf("tab %d not found", id)
		}
	}
	return nil
}
