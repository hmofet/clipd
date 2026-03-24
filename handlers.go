package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Server holds the store and provides HTTP handler methods.
type Server struct {
	store *Store
}

// NewServer creates a Server with the given Store.
func NewServer(store *Store) *Server {
	return &Server{store: store}
}

// RegisterRoutes sets up all API and auth routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Auth routes (no auth required for login/verify).
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("GET /auth/verify", s.handleVerify)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/me", requireAuth(s.store, s.handleMe))

	// Tab API routes (all require auth).
	mux.HandleFunc("GET /api/tabs", requireAuth(s.store, s.handleListTabs))
	mux.HandleFunc("POST /api/tabs", requireAuth(s.store, s.handleCreateTab))
	mux.HandleFunc("PUT /api/tabs/reorder", requireAuth(s.store, s.handleReorderTabs))
	mux.HandleFunc("GET /api/tabs/{id}", requireAuth(s.store, s.handleGetTab))
	mux.HandleFunc("PUT /api/tabs/{id}", requireAuth(s.store, s.handleUpdateTab))
	mux.HandleFunc("DELETE /api/tabs/{id}", requireAuth(s.store, s.handleDeleteTab))
}

// parseID extracts and validates the {id} path value as a positive integer.
func parseID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func (s *Server) handleListTabs(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	tabs, err := s.store.ListTabs(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tabs == nil {
		tabs = []Tab{}
	}
	writeJSON(w, http.StatusOK, tabs)
}

func (s *Server) handleGetTab(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid tab id", http.StatusBadRequest)
		return
	}
	tab, err := s.store.GetTab(userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "tab not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tab)
}

func (s *Server) handleCreateTab(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Name = ""
	}
	if req.Name == "" {
		tabs, _ := s.store.ListTabs(userID)
		req.Name = fmt.Sprintf("Tab %d", len(tabs)+1)
	}

	tab, err := s.store.CreateTab(userID, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, tab)
}

func (s *Server) handleUpdateTab(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid tab id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	tab, err := s.store.UpdateTab(userID, id, req.Name, req.Content)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "tab not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tab)
}

func (s *Server) handleDeleteTab(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid tab id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteTab(userID, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "tab not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReorderTabs(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := s.store.ReorderTabs(userID, req.IDs); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON sets Content-Type and encodes data as JSON.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}
