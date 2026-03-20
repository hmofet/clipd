package main

import "embed"

// Embed the frontend HTML file into the binary.
// The go:embed directive tells the Go compiler to include index.html
// in the compiled binary, so we can ship a single file.
//
//go:embed index.html
var staticFiles embed.FS
