package main

import (
	"context"
	"net/http"
	"os"
)

type contextKey string

const userIDKey contextKey = "userID"

// authDisabled returns true when AUTH_DISABLED env var is set to any non-empty value.
func authDisabled() bool {
	return os.Getenv("AUTH_DISABLED") != ""
}

// requireAuth is middleware that validates the session cookie and injects the
// user ID into the request context. Returns 401 if not authenticated.
// When AUTH_DISABLED is set, uses the default user from the store.
func requireAuth(store *Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authDisabled() {
			ctx := context.WithValue(r.Context(), userIDKey, store.DefaultUserID)
			next(w, r.WithContext(ctx))
			return
		}

		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := store.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// getUserID extracts the authenticated user ID from the request context.
func getUserID(r *http.Request) int {
	return r.Context().Value(userIDKey).(int)
}
