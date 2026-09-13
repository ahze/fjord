package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/daemonless/fjord/pkg/engine"
)

// maintenanceBackend picks the engine named by the request (query or body),
// falling back to the default. Returns the resolved name for the response.
func (s *server) maintenanceBackend(name string) (engine.Backend, string) {
	if name != "" {
		if b, ok := s.backend(name); ok {
			return b, name
		}
	}
	return s.primaryBackend(), s.defaultEngine()
}

// handleDiskUsage reports reclaimable space and prune capabilities for one
// engine (GET ?engine=<name>, default engine when omitted).
func (s *server) handleDiskUsage(w http.ResponseWriter, r *http.Request) {
	b, name := s.maintenanceBackend(r.URL.Query().Get("engine"))
	rows, err := b.DiskUsage(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if rows == nil {
		rows = []engine.DiskRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"engine": name, "capabilities": b.PruneCapabilities(), "rows": rows})
}

// handlePrune reclaims space per the posted options (POST). Prune can take a
// while (removing 100s of images), so it gets a generous timeout.
func (s *server) handlePrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Engine string `json:"engine"`
		engine.PruneOptions
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", 400)
		return
	}
	b, _ := s.maintenanceBackend(req.Engine)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	report, err := b.Prune(ctx, req.PruneOptions)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}
