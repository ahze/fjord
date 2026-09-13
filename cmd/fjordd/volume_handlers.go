package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	composepkg "github.com/daemonless/fjord/pkg/compose"
	"github.com/daemonless/fjord/pkg/engine"
)

// volumeWithUse is the volumes wire format: the engine volume plus which fjord
// stacks reference it (by top-level named volume in their compose).
type volumeWithUse struct {
	engine.Volume
	UsedBy []string `json:"usedBy,omitempty"`
}

// stackVolumeUses maps volume name -> stacks whose compose declares it.
func (s *server) stackVolumeUses() map[string][]string {
	uses := map[string][]string{}
	stacks, err := s.manager.List()
	if err != nil {
		return uses
	}
	for _, st := range stacks {
		full, err := s.manager.Get(st.Name)
		if err != nil {
			continue
		}
		for _, v := range composepkg.NamedVolumes(full.Compose) {
			// A compose-declared volume is created by the runtime as
			// "<project>_<name>" (project = lowercased stack name), so index
			// both the bare and prefixed forms or the usedBy chip misses
			// prefixed volumes (e.g. immich's "103_model-cache").
			uses[v] = append(uses[v], st.Name)
			uses[strings.ToLower(st.Name)+"_"+v] = append(uses[strings.ToLower(st.Name)+"_"+v], st.Name)
		}
	}
	return uses
}

// handleVolumes lists (GET) or creates (POST) named volumes. Creation takes
// the runtime-neutral engine.VolumeSpec; the backend translates it into its
// platform's mount options.
func (s *server) handleVolumes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		vols, err := s.backendForRequest(r).Volumes(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		uses := s.stackVolumeUses()
		out := make([]volumeWithUse, 0, len(vols))
		for _, v := range vols {
			out = append(out, volumeWithUse{Volume: v, UsedBy: uses[v.Name]})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	case http.MethodPost:
		var spec engine.VolumeSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil || spec.Name == "" {
			http.Error(w, "name required", 400)
			return
		}
		if spec.Kind == "nfs" && spec.Options == nil && (spec.Server == "" || spec.Path == "") {
			http.Error(w, "nfs volume requires server and path", 400)
			return
		}
		if spec.Kind == "smb" && spec.Options == nil && (spec.Server == "" || strings.Trim(spec.Path, "/") == "") {
			http.Error(w, "smb volume requires server and share", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		v, err := s.backendForRequest(r).CreateVolume(ctx, spec)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// handleVolumeDelete removes a named volume: DELETE /api/volumes/<name>[?force=true].
func (s *server) handleVolumeDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/volumes/")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	if err := s.backendForRequest(r).RemoveVolume(r.Context(), name, force); err != nil {
		// In-use is a 409 the UI can act on (offer force); anything else is a 502.
		if errors.Is(err, engine.ErrInUse) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 502)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"deleted"}`))
}
