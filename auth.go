package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// handleLogin accepts an email and sends a magic link.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		// Always return success to prevent enumeration.
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Check your email"})
		return
	}

	token, err := s.store.CreateAuthToken(email)
	if err != nil {
		log.Printf("error creating auth token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := sendMagicLink(email, token); err != nil {
		log.Printf("error sending magic link to %s: %v", email, err)
		// Still return success to prevent enumeration.
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Check your email"})
}

// handleVerify validates a magic link token and creates a session.
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	email, err := s.store.ValidateAuthToken(token)
	if err != nil {
		// Show a simple error page.
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0"><div style="text-align:center"><h2>Link expired or invalid</h2><p>Please request a new login link.</p><p><a href="/" style="color:#4a90d9">Back to clipd</a></p></div></body></html>`))
		return
	}

	userID, err := s.store.UpsertUser(email)
	if err != nil {
		log.Printf("error upserting user: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionToken, err := s.store.CreateSession(userID)
	if err != nil {
		log.Printf("error creating session: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout clears the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		s.store.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// handleMe returns the current user's email.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	email, err := s.store.GetUserEmail(userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"email": email})
}
