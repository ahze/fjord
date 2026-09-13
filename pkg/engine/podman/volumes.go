package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"

	"github.com/daemonless/fjord/pkg/engine"
)

// libpodVolume is the subset of the libpod /volumes response we surface.
type libpodVolume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	Options    map[string]string `json:"Options"`
	CreatedAt  string            `json:"CreatedAt"`
}

// anonRe matches podman's auto-generated (anonymous) volume names.
var anonRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (v libpodVolume) toEngine() engine.Volume {
	kind := "local"
	switch v.Options["type"] {
	case "nfs":
		kind = "nfs"
	case "smbfs", "cifs":
		kind = "smb"
	}
	return engine.Volume{
		Name:       v.Name,
		Driver:     v.Driver,
		Kind:       kind,
		Anonymous:  anonRe.MatchString(v.Name),
		Mountpoint: v.Mountpoint,
		Options:    v.Options,
		CreatedAt:  v.CreatedAt,
	}
}

// Volumes lists podman-managed named volumes over the libpod socket.
func (b *Backend) Volumes(ctx context.Context) ([]engine.Volume, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v4.0.0/libpod/volumes/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("libpod volumes/json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libpod volumes/json: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var raw []libpodVolume
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode libpod volumes: %w", err)
	}
	out := make([]engine.Volume, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.toEngine())
	}
	return out, nil
}

// CreateVolume creates a named volume, translating the runtime-neutral spec
// into this platform's mount options. This is where the FreeBSD-vs-Linux NFS
// syntax difference lives: FreeBSD mount_nfs takes device="server:/path",
// Linux takes o="addr=server,..." with device=":/path". Raw spec.Options, when
// provided, override the translation (advanced escape hatch).
func (b *Backend) CreateVolume(ctx context.Context, spec engine.VolumeSpec) (engine.Volume, error) {
	driver := spec.Driver
	if driver == "" {
		driver = "local"
	}
	opts := spec.Options
	if opts == nil && spec.Kind == "smb" {
		mode := "rw"
		if spec.ReadOnly {
			mode = "ro"
		}
		username := spec.User
		if username == "" {
			username = "guest"
		}
		share := strings.Trim(spec.Path, "/")
		if err := ensureSMBCredentials(spec.Server, username, spec.Password); err != nil {
			return engine.Volume{}, err
		}
		if runtime.GOOS == "freebsd" {
			// mount(8) turns "-o -N,-I=host" into mount_smbfs flags. -N: never
			// prompt (password comes from /etc/nsmb.conf); -I: skip NetBIOS
			// name lookup. Ownership: the host user with uid 1000 when there is
			// one, else world-writable modes so the container's user can write
			// (the server enforces the real permissions either way).
			o := []string{"-N", "-I=" + spec.Server}
			if uname, gname := hostNames(1000, 1000); uname != "" {
				o = append(o, "-u="+uname, "-f=0664", "-d=0775")
				if gname != "" {
					o = append(o, "-g="+gname)
				}
			} else {
				o = append(o, "-f=0666", "-d=0777")
			}
			o = append(o, mode)
			opts = map[string]string{"type": "smbfs", "device": "//" + username + "@" + spec.Server + "/" + share, "o": strings.Join(o, ",")}
		} else {
			opts = map[string]string{
				"type":   "cifs",
				"device": "//" + spec.Server + "/" + share,
				"o":      "credentials=" + smbCredFile(spec.Server, username) + ",uid=1000,gid=1000,iocharset=utf8," + mode,
			}
		}
	}
	if opts == nil && spec.Kind == "nfs" {
		mode := "rw"
		if spec.ReadOnly {
			mode = "ro"
		}
		path := spec.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if runtime.GOOS == "freebsd" {
			opts = map[string]string{"type": "nfs", "device": spec.Server + ":" + path, "o": mode}
		} else {
			opts = map[string]string{"type": "nfs", "device": ":" + path, "o": "addr=" + spec.Server + "," + mode}
		}
	}
	body, err := json.Marshal(map[string]any{
		"Name":    spec.Name,
		"Driver":  driver,
		"Options": opts,
	})
	if err != nil {
		return engine.Volume{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v4.0.0/libpod/volumes/create", bytes.NewReader(body))
	if err != nil {
		return engine.Volume{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return engine.Volume{}, fmt.Errorf("libpod volumes/create: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return engine.Volume{}, fmt.Errorf("libpod volumes/create: %s: %s", resp.Status, bytes.TrimSpace(data))
	}
	var v libpodVolume
	if err := json.Unmarshal(data, &v); err != nil {
		return engine.Volume{}, fmt.Errorf("decode created volume: %w", err)
	}
	return v.toEngine(), nil
}

// RemoveVolume deletes a named volume; force removes it even if a container
// still references it.
func (b *Backend) RemoveVolume(ctx context.Context, name string, force bool) error {
	u := "http://d/v4.0.0/libpod/volumes/" + url.PathEscape(name)
	if force {
		u += "?force=true"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("libpod volumes delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		// libpod 409 = volume mounted by a running container. Surface a clean,
		// typed error (ErrInUse) so the handler returns 409 and the UI can offer
		// a force-remove -- never the raw {"cause":...} JSON.
		if resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("%w: volume %q is mounted by a running container — stop its stack, or force-remove", engine.ErrInUse, name)
		}
		return fmt.Errorf("libpod volumes delete: %s: %s", resp.Status, bytes.TrimSpace(data))
	}
	return nil
}
