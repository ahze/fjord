package main

import (
	"encoding/json"
	"net/http"
)

// Provider plugins layer opinions on the generic core; each is a toggle in
// Settings → Extensions. Engines are the other plugin kind (engines.go). The
// core never depends on a plugin being on: with "homelab" off, folder sets
// are still there, just without the presets and their automatic pick-up.
type pluginInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

var providerPlugins = []pluginInfo{
	{
		Name:  "homelab",
		Label: "Homelab",
		Description: "Preset folder sets for a media homelab — Movies, TV, Music, Downloads, Books, Photos — " +
			"that apps pick up automatically at install.",
	},
}

// pluginEnabled reports whether a provider plugin is on. Plugins are on
// unless the operator turned them off (settings.disabledPlugins).
func (s *server) pluginEnabled(name string) bool {
	for _, d := range loadSettings(s.fjordRoot).DisabledPlugins {
		if d == name {
			return false
		}
	}
	return true
}

// handlePlugins lists provider plugins with their state (GET) or toggles one
// (POST {name, enabled}).
func (s *server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out := make([]pluginInfo, 0, len(providerPlugins))
		for _, p := range providerPlugins {
			p.Enabled = s.pluginEnabled(p.Name)
			out = append(out, p)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"plugins": out})

	case http.MethodPost:
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", 400)
			return
		}
		known := false
		for _, p := range providerPlugins {
			known = known || p.Name == req.Name
		}
		if !known {
			http.Error(w, "unknown plugin: "+req.Name, 400)
			return
		}
		if err := updateSettings(s.fjordRoot, func(st *savedSettings) {
			st.DisabledPlugins = toggleInList(st.DisabledPlugins, req.Name, !req.Enabled)
		}); err != nil {
			http.Error(w, "persist: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"name": req.Name, "enabled": req.Enabled})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
