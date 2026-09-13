package main

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/daemonless/fjord/pkg/doctor"
)

// handleHealthz is an unauthenticated liveness probe for reverse proxies,
// container healthchecks, and the rc "status" command. Deliberately trivial --
// no engine detection, no auth -- so it answers whenever the daemon is up.
func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
}

// handleAbout reports the daemon's identity and effective configuration --
// the read-only truth behind the Settings page. Configuration is env-driven
// (rc.conf on FreeBSD), so the UI displays it rather than edits it.
func (s *server) handleAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	host, _ := os.Hostname()
	json.NewEncoder(w).Encode(map[string]any{
		"version":   version,
		"hostname":  strings.SplitN(host, ".", 2)[0], // short name: the browser tab reads "fjord - saturn"
		"engine":    s.defaultEngine(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH, // OCI spelling (amd64, arm64); the store hides apps not built for it
		"fjordRoot": s.fjordRoot,
		"stacksDir": s.stacksDir,
		"catalogs":  len(s.cat.Sources()), // refreshable sources; 0 = local seed only (or none)
		"mode":      doctor.Mode(),        // "host" | "container": gates the file browser
		"listen":    s.listenAddr,
	})
}
