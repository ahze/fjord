package main

import (
	"encoding/json"
	"net/http"

	composepkg "github.com/daemonless/fjord/pkg/compose"
	"github.com/daemonless/fjord/pkg/stack"
)

// handleComposeMounts backs the Resources → Storage table. It's stateless: it
// operates on the compose TEXT the client sends (the editor's in-memory value,
// possibly unsaved) and never touches disk, so the compose stays the single
// source of truth. op=list returns the parsed+classified mounts; op=add/remove
// return the transformed compose for the editor to adopt (and Save as usual).
func (s *server) handleComposeMounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Compose  string `json:"compose"`
		Env      string `json:"env"`
		Op       string `json:"op"`   // "list" | "add" | "remove"
		Kind     string `json:"kind"` // add: "bind" | "volume"
		Source   string `json:"source"`
		Dest     string `json:"dest"`
		ReadOnly bool   `json:"readOnly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", 400)
		return
	}
	env := (&stack.Stack{Env: req.Env}).EnvMap()

	w.Header().Set("Content-Type", "application/json")
	switch req.Op {
	case "", "list":
		mounts := composepkg.Mounts(req.Compose, env)
		if mounts == nil {
			mounts = []composepkg.Mount{}
		}
		json.NewEncoder(w).Encode(map[string]any{"mounts": mounts})
	case "add":
		var out string
		var err error
		if req.Kind == "volume" {
			out, err = composepkg.AttachVolume(req.Compose, req.Source, req.Dest, req.ReadOnly)
		} else {
			out, err = composepkg.AttachBindMount(req.Compose, req.Source, req.Dest, req.ReadOnly)
		}
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"compose": out})
	case "remove":
		out, err := composepkg.RemoveMount(req.Compose, req.Dest)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"compose": out})
	default:
		http.Error(w, "unknown op: "+req.Op, 400)
	}
}
