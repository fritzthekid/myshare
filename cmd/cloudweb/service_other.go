//go:build !windows

package main

import (
	"log"
	"net/http"
)

// runService starts the HTTP server, blocking. On Linux/macOS, cloudweb
// runs as a regular systemd service (see deploy/cloudweb.service) - no
// Service Control Manager handshake and no folder watcher needed, the
// CUPS backend (cmd/backend) handles that there via a push to /push.
func runService(st *state, mux http.Handler, addr string) {
	log.Printf("cloudweb listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
