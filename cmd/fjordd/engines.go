package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/daemonless/fjord/pkg/doctor"
	"github.com/daemonless/fjord/pkg/engine"
)

// descriptor returns the registered descriptor for an engine name, or false.
func descriptor(name string) (engine.Descriptor, bool) {
	for _, d := range engineDescriptors {
		if d.Name == name {
			return d, true
		}
	}
	return engine.Descriptor{}, false
}

// engineInfo is the wire format for one selectable runtime engine, built
// entirely from the engine's own Descriptor.
type engineInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default"`
	Available   bool   `json:"available"`         // the runtime is installed on this host
	Enabled     bool   `json:"enabled"`           // registered + usable (available AND not disabled)
	Reason      string `json:"reason,omitempty"`  // why it can't run, when unavailable
	Warning     string `json:"warning,omitempty"` // usable-with-a-caveat note
	CanInstall  bool   `json:"canInstall,omitempty"`
}

// handleEngine reports the engines this host can run and the default for new
// installs (GET), or sets the default (POST). Every engine describes itself via
// its Descriptor, so this handler names no specific engine.
func (s *server) handleEngine(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		host := doctor.Mode() == "host"
		engines := []engineInfo{}
		for _, d := range engineDescriptors {
			_, registered := s.backend(d.Name)
			ok, reason, warning := d.Available()
			info := engineInfo{
				Name:        d.Name,
				Description: d.Description,
				Default:     d.Name == s.defaultEngine(),
				Available:   ok,
				Enabled:     registered,
				Reason:      reason,
				CanInstall:  !ok && host && d.Package != "",
			}
			if ok {
				info.Warning = warning
			}
			engines = append(engines, info)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"default": s.defaultEngine(),
			"jailed":  engine.Jailed(),
			"engines": engines,
		})

	case http.MethodPost:
		var req struct {
			Default string `json:"default"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", 400)
			return
		}
		if _, ok := s.backend(req.Default); !ok {
			http.Error(w, "engine not available: "+req.Default, 400)
			return
		}
		if err := updateSettings(s.fjordRoot, func(st *savedSettings) { st.DefaultEngine = req.Default }); err != nil {
			http.Error(w, "persist: "+err.Error(), 500)
			return
		}
		s.setDefaultEngine(req.Default)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"default": req.Default})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// stacksOnEngine returns the display labels of stacks bound to the named engine
// ("<display> (<id>)"), so a blocked disable can name what depends on it.
func (s *server) stacksOnEngine(name string) []string {
	var out []string
	stacks, err := s.manager.List()
	if err != nil {
		return out
	}
	for _, st := range stacks {
		if st.EngineName() == name {
			label := st.DisplayName
			if label == "" || label == st.Name {
				out = append(out, st.Name)
			} else {
				out = append(out, label+" ("+st.Name+")")
			}
		}
	}
	return out
}

// handleEngineToggle enables or disables an engine (Extensions tab). Disabling is
// blocked while stacks are bound to it -- they'd be stranded (every operation
// on them would fail with engine.ErrUnavailable) -- and the response lists them.
func (s *server) handleEngineToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", 400)
		return
	}
	d, ok := descriptor(req.Name)
	if !ok {
		http.Error(w, "unknown engine: "+req.Name, 400)
		return
	}

	if req.Enabled {
		if avail, reason, _ := d.Available(); !avail {
			http.Error(w, "engine not available on this host: "+reason, http.StatusConflict)
			return
		}
	} else {
		// Refuse to strand stacks that run on this engine.
		if bound := s.stacksOnEngine(req.Name); len(bound) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":  "Can't disable " + req.Name + " while stacks run on it. Move or delete them first.",
				"stacks": bound,
			})
			return
		}
		// Don't disable the last usable engine -- nothing could run.
		if _, isOn := s.backend(req.Name); isOn && s.engineCount() <= 1 {
			http.Error(w, "can't disable the only enabled engine", http.StatusConflict)
			return
		}
	}

	if err := updateSettings(s.fjordRoot, func(st *savedSettings) {
		st.DisabledEngines = toggleInList(st.DisabledEngines, req.Name, !req.Enabled)
	}); err != nil {
		http.Error(w, "persist: "+err.Error(), 500)
		return
	}
	s.rebuildBackends()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "default": s.defaultEngine()})
}

// toggleInList adds name to the list when present==true, removes it otherwise,
// keeping the list deduplicated.
func toggleInList(list []string, name string, present bool) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != name {
			out = append(out, x)
		}
	}
	if present {
		out = append(out, name)
	}
	return out
}

// handleEngineInstall installs an engine's package via pkg (host mode only;
// the package name comes from the engine's Descriptor, so this stays generic).
// fjordd builds its registry at startup, so a restart enables the new engine.
func (s *server) handleEngineInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if doctor.Mode() != "host" {
		http.Error(w, "engines can only be installed when fjordd runs on the host", http.StatusConflict)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", 400)
		return
	}
	d, ok := descriptor(req.Name)
	if !ok || d.Package == "" {
		http.Error(w, "unknown engine: "+req.Name, 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	// Package may list several (appjail + its director); one pkg invocation.
	cmd := exec.CommandContext(ctx, "pkg", append([]string{"install", "-y"}, strings.Fields(d.Package)...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, "pkg install "+d.Package+" failed: "+string(out), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "installed", "restartRequired": true})
}
