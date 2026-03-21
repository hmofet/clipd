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
// In Go, grouping handlers on a struct is a clean way to share dependencies.
type Server struct {
	store *Store
}

// NewServer creates a Server with the given Store.
func NewServer(store *Store) *Server {
	return &Server{store: store}
}

// RegisterRoutes sets up all API routes on the given mux.
// Go 1.22+ supports "METHOD /path" patterns in http.ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tabs", s.handleListTabs)
	mux.HandleFunc("POST /api/tabs", s.handleCreateTab)
	mux.HandleFunc("PUT /api/tabs/reorder", s.handleReorderTabs)
	mux.HandleFunc("GET /api/tabs/{id}", s.handleGetTab)
	mux.HandleFunc("PUT /api/tabs/{id}", s.handleUpdateTab)
	mux.HandleFunc("DELETE /api/tabs/{id}", s.handleDeleteTab)
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
	tabs, err := s.store.ListTabs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Return empty array instead of null when there are no tabs.
	if tabs == nil {
		tabs = []Tab{}
	}
	writeJSON(w, http.StatusOK, tabs)
}

func (s *Server) handleGetTab(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid tab id", http.StatusBadRequest)
		return
	}
	tab, err := s.store.GetTab(id)
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
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Name = ""
	}
	if req.Name == "" {
		// Generate a default name based on current tab count.
		tabs, _ := s.store.ListTabs()
		req.Name = fmt.Sprintf("Tab %d", len(tabs)+1)
	}

	tab, err := s.store.CreateTab(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, tab)
}

func (s *Server) handleUpdateTab(w http.ResponseWriter, r *http.Request) {
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

	tab, err := s.store.UpdateTab(id, req.Name, req.Content)
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
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid tab id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteTab(id); err != nil {
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
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := s.store.ReorderTabs(req.IDs); err != nil {
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
