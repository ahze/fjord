package main

import (
	"encoding/json"
	"net/http"
)

// Install wizard disclosure level (Settings → Advanced). 1 shows only the
// primary fields with Options collapsed (the default), 2 opens Options, 3
// opens Advanced too. Stored as wizardDetail; 0 reads as 1.
const (
	wizardDetailMin = 1
	wizardDetailMax = 3
)

func wizardDetailOf(st savedSettings) int {
	if st.WizardDetail < wizardDetailMin || st.WizardDetail > wizardDetailMax {
		return wizardDetailMin
	}
	return st.WizardDetail
}

func (s *server) handleWizardSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"detail": wizardDetailOf(loadSettings(s.fjordRoot))})
	case http.MethodPost:
		var req struct {
			Detail int `json:"detail"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", 400)
			return
		}
		if req.Detail < wizardDetailMin || req.Detail > wizardDetailMax {
			http.Error(w, "detail must be 1, 2 or 3", 400)
			return
		}
		if err := updateSettings(s.fjordRoot, func(st *savedSettings) { st.WizardDetail = req.Detail }); err != nil {
			http.Error(w, "persist: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"detail": req.Detail})
	default:
		http.Error(w, "method not allowed", 405)
	}
}
