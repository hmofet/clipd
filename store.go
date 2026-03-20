package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Store wraps a BoltDB database and provides CRUD operations for tabs.
type Store struct {
	db *bolt.DB
}

// Bucket names used in BoltDB
var (
	tabsBucket = []byte("tabs")
	metaBucket = []byte("meta")
)

// NewStore opens the BoltDB file and creates buckets if they don't exist.
func NewStore(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Create buckets on first run — this is a common BoltDB pattern
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(tabsBucket); err != nil {
			return err
		}
		b, err := tx.CreateBucketIfNotExists(metaBucket)
		if err != nil {
			return err
		}
		// Initialize next_id if it doesn't exist
		if b.Get([]byte("next_id")) == nil {
			return b.Put([]byte("next_id"), []byte("1"))
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying BoltDB database.
func (s *Store) Close() error {
	return s.db.Close()
}

// nextID reads and increments the auto-increment counter in the meta bucket.
func (s *Store) nextID(tx *bolt.Tx) (string, error) {
	b := tx.Bucket(metaBucket)
	val := b.Get([]byte("next_id"))
	id, err := strconv.Atoi(string(val))
	if err != nil {
		return "", fmt.Errorf("parsing next_id: %w", err)
	}
	next := strconv.Itoa(id + 1)
	if err := b.Put([]byte("next_id"), []byte(next)); err != nil {
		return "", err
	}
	return strconv.Itoa(id), nil
}

// ListTabs returns all tabs sorted by their order field.
func (s *Store) ListTabs() ([]Tab, error) {
	var tabs []Tab

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(tabsBucket)
		return b.ForEach(func(k, v []byte) error {
			var tab Tab
			if err := json.Unmarshal(v, &tab); err != nil {
				return err
			}
			tabs = append(tabs, tab)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(tabs, func(i, j int) bool {
		return tabs[i].Order < tabs[j].Order
	})

	return tabs, nil
}

// GetTab retrieves a single tab by ID.
func (s *Store) GetTab(id string) (*Tab, error) {
	var tab Tab

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(tabsBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("tab not found")
		}
		return json.Unmarshal(v, &tab)
	})
	if err != nil {
		return nil, err
	}

	return &tab, nil
}

// CreateTab creates a new tab with the given name. It auto-assigns an ID and
// places the tab at the end of the order.
func (s *Store) CreateTab(name string) (*Tab, error) {
	var tab Tab

	err := s.db.Update(func(tx *bolt.Tx) error {
		id, err := s.nextID(tx)
		if err != nil {
			return err
		}

		// Count existing tabs to determine order
		b := tx.Bucket(tabsBucket)
		order := 0
		b.ForEach(func(k, v []byte) error {
			order++
			return nil
		})

		now := time.Now().UTC()
		tab = Tab{
			ID:        id,
			Name:      name,
			Content:   "",
			Order:     order,
			CreatedAt: now,
			UpdatedAt: now,
		}

		data, err := json.Marshal(tab)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
	if err != nil {
		return nil, err
	}

	return &tab, nil
}

// UpdateTab updates an existing tab's name and/or content.
func (s *Store) UpdateTab(id, name, content string) (*Tab, error) {
	var tab Tab

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(tabsBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("tab not found")
		}
		if err := json.Unmarshal(v, &tab); err != nil {
			return err
		}

		tab.Name = name
		tab.Content = content
		tab.UpdatedAt = time.Now().UTC()

		data, err := json.Marshal(tab)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
	if err != nil {
		return nil, err
	}

	return &tab, nil
}

// DeleteTab removes a tab by ID and reorders remaining tabs.
func (s *Store) DeleteTab(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(tabsBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("tab not found")
		}
		if err := b.Delete([]byte(id)); err != nil {
			return err
		}

		// Reorder remaining tabs to close any gaps
		var tabs []Tab
		b.ForEach(func(k, v []byte) error {
			var t Tab
			json.Unmarshal(v, &t)
			tabs = append(tabs, t)
			return nil
		})
		sort.Slice(tabs, func(i, j int) bool {
			return tabs[i].Order < tabs[j].Order
		})
		for i, t := range tabs {
			t.Order = i
			data, _ := json.Marshal(t)
			b.Put([]byte(t.ID), data)
		}

		return nil
	})
}

// ReorderTabs sets the order of tabs based on the provided list of IDs.
func (s *Store) ReorderTabs(ids []string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(tabsBucket)
		for i, id := range ids {
			v := b.Get([]byte(id))
			if v == nil {
				return fmt.Errorf("tab %s not found", id)
			}
			var tab Tab
			if err := json.Unmarshal(v, &tab); err != nil {
				return err
			}
			tab.Order = i
			data, err := json.Marshal(tab)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(id), data); err != nil {
				return err
			}
		}
		return nil
	})
}
