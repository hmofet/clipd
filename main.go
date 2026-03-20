package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

func main() {
	// Parse command-line flags — the flag package is Go's standard way
	// to handle CLI arguments.
	port := flag.Int("port", 8181, "port to listen on")
	dbPath := flag.String("db", "clipd.db", "path to BoltDB database file")
	flag.Parse()

	// Open the BoltDB store
	store, err := NewStore(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	// Create server and register API routes
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

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("clipd starting on http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
