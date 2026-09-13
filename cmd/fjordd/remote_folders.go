package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/daemonless/fjord/pkg/engine"
)

// Remote folders are written as URLs inside the same string lists as host
// paths -- in folder sets, wizard rows and the install request -- so nothing
// above the install pipeline needs a second data model:
//
//	nfs://server/export/path
//	smb://user@server/share
//
// At install each becomes a named volume (created once, reused after) that
// the engine mounts; SMB passwords are stored separately (see
// handleSMBCredentials) and never appear in the URL.
type remoteFolder struct {
	Kind, Server, Path, User string
}

// parseRemote recognises the two remote URL forms; ok=false for a host path.
func parseRemote(s string) (remoteFolder, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "nfs://") && !strings.HasPrefix(s, "smb://") {
		return remoteFolder{}, false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return remoteFolder{}, false
	}
	r := remoteFolder{Kind: u.Scheme, Server: u.Hostname(), Path: strings.Trim(u.Path, "/")}
	if u.User != nil {
		r.User = u.User.Username()
	}
	if r.Kind == "nfs" {
		r.Path = "/" + r.Path
	}
	if r.Path == "" || r.Path == "/" {
		return remoteFolder{}, false
	}
	return r, true
}

// remoteVolumeName is the stable named-volume name for a remote folder, so the
// same export used by two apps is one volume.
func remoteVolumeName(r remoteFolder) string {
	return "fjord-" + r.Kind + "-" + storageSlug(r.Server+" "+r.Path, "remote")
}

// ensureRemoteVolume returns the named volume for a remote folder, creating
// it on the given engine if it doesn't exist yet.
func ensureRemoteVolume(ctx context.Context, b engine.Backend, r remoteFolder) (string, error) {
	name := remoteVolumeName(r)
	vols, err := b.Volumes(ctx)
	if err != nil {
		return "", fmt.Errorf("list volumes: %w", err)
	}
	for _, v := range vols {
		if v.Name == name {
			return name, nil
		}
	}
	spec := engine.VolumeSpec{Name: name, Kind: r.Kind, Server: r.Server, Path: r.Path, User: r.User}
	if _, err := b.CreateVolume(ctx, spec); err != nil {
		return "", fmt.Errorf("create volume for %s://%s%s: %w", r.Kind, r.Server, r.Path, err)
	}
	return name, nil
}

// handleVolumeEnsure resolves a remote folder URL to its named volume,
// creating it if needed: POST /api/volumes/ensure {source} -> {name}.
// Used by Resources → Storage when a folder set carries remote entries.
func (s *server) handleVolumeEnsure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", 400)
		return
	}
	rf, ok := parseRemote(req.Source)
	if !ok {
		http.Error(w, "not a remote folder (expected nfs://server/export or smb://user@server/share)", 400)
		return
	}
	// ?stack= / ?engine= scope the volume to the engine that will mount it;
	// the default engine may not even support named volumes (appjail).
	name, err := ensureRemoteVolume(r.Context(), s.backendForRequest(r), rf)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"name": name})
}

// handleSMBCredentials stores the password for user@server so smb:// folders
// and volumes can mount without ever carrying it: POST {server,user,password}.
func (s *server) handleSMBCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Server   string `json:"server"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Server == "" || req.User == "" {
		http.Error(w, "server and user required", 400)
		return
	}
	be := s.remoteVolumeBackend()
	if be == nil {
		http.Error(w, "no engine on this host supports SMB volumes", 400)
		return
	}
	if err := be.StoreSMBCredentials(req.Server, req.User, req.Password); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stored"})
}
