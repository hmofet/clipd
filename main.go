package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

func main() {
	// Read configuration from environment variables.
	// Railway sets PORT and DATABASE_URL automatically.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Open the PostgreSQL connection pool and initialize the schema.
	store, err := NewStore(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	// Create server and register API routes.
	server := NewServer(store)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Serve the embedded index.html at the root path.
	// fs.Sub extracts a subtree from the embed.FS so files are served
	// from "/" instead of "/index.html".
	htmlFS, err := fs.Sub(staticFiles, ".")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(htmlFS)))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("clipd starting on http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
