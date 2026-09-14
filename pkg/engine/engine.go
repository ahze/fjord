// Package engine defines the runtime abstraction fjord drives stacks through.
//
// The contract: a stack's definition format is compose YAML (stack.Stack) --
// that is fjord's stack language, not a runtime detail. Each Backend interprets
// it for its runtime (podman today; AppJail/Docker later would translate
// compose services into their own units). Everything above this interface --
// HTTP handlers, the WebSocket exec bridge, the UI -- must stay runtime-
// agnostic: no runtime command strings, no runtime naming conventions, no
// platform mount syntax. Runtime knowledge lives only in the backend packages,
// and anything the UI shows about the runtime (container names, the commands
// being run, volume flavors) is *reported by* the backend, never assumed.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/daemonless/fjord/pkg/stack"
)

// ErrInUse marks a resource (volume, network) that can't be removed because
// something is using it. Backends wrap it around their runtime's specific
// error; the generic handler maps it to HTTP 409 so the UI can offer a force
// option instead of leaking the runtime's raw response.
var ErrInUse = errors.New("resource in use")

// Port is one published port mapping of a running container.
type Port struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// ContainerStatus describes a single container observed via the runtime's
// status API. Ports are the container's ACTUAL published ports -- the source
// of truth for "where is this app reachable", which the saved compose is not
// (edits only apply on recreate).
type ContainerStatus struct {
	Name  string `json:"name"`
	State string `json:"state"` // running | stopped | starting | crashed | ...
	Ports []Port `json:"ports,omitempty"`
	// Detail explains a non-running state in one line (e.g. "crash-looping:
	// see Logs") -- shown as a hint, never parsed.
	Detail string `json:"detail,omitempty"`
}

// StackStatus is the aggregate lifecycle state of a stack's containers.
// State is one of "running" (all containers up), "partial" (some up),
// "stopped" (none up, but known), or "unknown" (status API unreachable).
type StackStatus struct {
	State      string            `json:"state"`
	Containers []ContainerStatus `json:"containers"`
}

// Network describes a container network a stack can attach to so it gets its
// own routable IP (avoiding host port collisions on busy hosts).
type Network struct {
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

// Volume is a runtime-managed named volume. Kind is the backend's
// classification ("local", "nfs", ...); Anonymous marks runtime-generated
// volumes (e.g. podman's 64-hex container volumes) the UI hides by default.
// Options carries raw driver detail for display.
type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Kind       string            `json:"kind,omitempty"`
	Anonymous  bool              `json:"anonymous,omitempty"`
	Mountpoint string            `json:"mountpoint"`
	Options    map[string]string `json:"options,omitempty"`
	CreatedAt  string            `json:"createdAt,omitempty"`
}

// VolumeSpec describes a volume to create, in runtime-neutral terms: the
// backend translates Kind+Server+Path into its platform's mount options (e.g.
// FreeBSD mount_nfs wants device="server:/path" while Linux wants
// o="addr=server"). Options, when set, is an advanced raw-passthrough that
// overrides the translation.
type VolumeSpec struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind,omitempty"` // "" or "local" | "nfs" | "smb"
	Server   string            `json:"server,omitempty"`
	Path     string            `json:"path,omitempty"`     // nfs export path, or smb share name
	User     string            `json:"user,omitempty"`     // smb: account ("guest" when empty)
	Password string            `json:"password,omitempty"` // smb: stored by the backend, never echoed
	ReadOnly bool              `json:"readOnly,omitempty"`
	Driver   string            `json:"driver,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
}

// ExecOptions configures an interactive shell session into a container.
type ExecOptions struct {
	Container string   // the container (a stack service's) to exec into
	Cmd       []string // shell command; defaults to ["/bin/sh"] when empty
}

// ExecSession is a live bidirectional shell into a container -- deliberately
// backend-agnostic so the WebSocket transport and xterm UI never touch podman.
// A future AppJail/Docker backend implements this the same way. Read yields the
// container's output; Write sends the user's keystrokes; Resize adjusts the TTY.
type ExecSession interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
}

// Descriptor is an engine's self-description: its identity, how to detect
// whether it can run on this host, and how to build its backend. All
// engine-specific knowledge (name, description, availability checks, version
// caveats, install package) lives with the engine and is surfaced through
// this -- the generic daemon code just iterates descriptors and never names a
// specific engine. Registering an engine = adding its Descriptor to the list.
type Descriptor struct {
	Name        string
	Description string
	Package     string // OS package that provides it (for host-mode install)
	// Available reports whether the engine can run here: ok, a reason when it
	// can't, and a non-fatal warning when it can but with caveats (e.g. an old
	// version). ok=false with reason means unavailable; ok=true with warning
	// means usable-with-a-caveat.
	Available func() (ok bool, reason, warning string)
	New       func() Backend
}

// Unmanaged is a container the engine runs that no fjord stack owns -- one
// started by hand or by another tool -- with what it takes to adopt it as a
// stack: podman hands over the argv it was created from (RunArgs), appjail
// converts the jail itself (Spec). Unadoptable, when set, says why neither
// is possible. Project names a runtime-side grouping made outside fjord (a
// director project), so the handler can tell fjord's own jails apart.
type Unmanaged struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Image       string     `json:"image"`
	State       string     `json:"state"`
	Project     string     `json:"project,omitempty"`
	RunArgs     []string   `json:"-"`
	Spec        *AdoptSpec `json:"-"`
	Unadoptable string     `json:"-"`
}

// AdoptSpec is the stack an unmanaged container becomes: a compose file and
// .env always (the UI, status and the Open link read those), plus the
// director bundle for a jail on the appjail engine.
type AdoptSpec struct {
	Service  string
	Compose  string
	Env      string
	Director string // appjail-director.yml
	Makejail string
	Template string // template.conf: the jail parameters
	Notes    []string
}

// DiskRow is one category of the runtime's disk-usage report (podman system df):
// how much space it holds and how much is reclaimable. Human strings are for
// display; RawReclaimable is bytes for sizing/decisions.
type DiskRow struct {
	Type           string `json:"type"` // "Images" | "Containers" | "Local Volumes" | ...
	Total          int    `json:"total"`
	Active         int    `json:"active"`
	Size           string `json:"size"`
	Reclaimable    string `json:"reclaimable"`
	RawReclaimable int64  `json:"rawReclaimable"`
	// Note explains something Prune can't act on -- e.g. images pinned by
	// containers the runtime doesn't own (buildah builds, appjail jails) --
	// so "in use" numbers that never move don't read as a failed prune.
	Note string `json:"note,omitempty"`
}

// PruneOptions selects what a Prune should reclaim. Empty = nothing. Each
// backend honors only what its PruneCapabilities report; the rest is ignored.
type PruneOptions struct {
	Containers bool `json:"containers"` // stopped containers
	Images     bool `json:"images"`     // dangling images
	AllImages  bool `json:"allImages"`  // ALL images not used by a container (implies Images)
	Volumes    bool `json:"volumes"`    // volumes not used by any container
	Networks   bool `json:"networks"`   // networks no container is attached to
	Build      bool `json:"build"`      // build leftovers: working containers, build cache
}

// PruneCapabilities says which PruneOptions a backend can act on, so the UI
// offers only what the selected engine supports.
type PruneCapabilities struct {
	Containers bool `json:"containers"`
	Images     bool `json:"images"`
	AllImages  bool `json:"allImages"`
	Volumes    bool `json:"volumes"`
	Networks   bool `json:"networks"`
	Build      bool `json:"build"`
}

// Capabilities reports optional features a backend supports beyond the core
// lifecycle, so generic code branches on a capability instead of an engine
// name. Mirrors PruneCapabilities.
type Capabilities struct {
	// RemoteVolumes: the engine can mount nfs:// / smb:// folders as named
	// volumes (and store SMB credentials for them).
	RemoteVolumes bool `json:"remoteVolumes"`
}

// PruneReport summarizes a prune run: a human total and the raw command output.
type PruneReport struct {
	Reclaimed string `json:"reclaimed"` // e.g. "165.7GB" ("" if unknown)
	Output    string `json:"output"`
}

// Backend defines the execution engine interface.
// By coding to this interface, FJORD can easily swap out Podman for Docker or AppJail later.
type Backend interface {
	// Up spins up a stack and returns an io.ReadCloser streaming the terminal output.
	Up(ctx context.Context, s *stack.Stack) (io.ReadCloser, error)
	// Down tears down a stack and returns an io.ReadCloser streaming the terminal output.
	Down(ctx context.Context, s *stack.Stack) (io.ReadCloser, error)
	// Update pulls the latest images for a stack then recreates it, streaming both steps.
	Update(ctx context.Context, s *stack.Stack) (io.ReadCloser, error)
	// Restart restarts a stack's containers in place (no pull, no recreate).
	Restart(ctx context.Context, s *stack.Stack) (io.ReadCloser, error)
	// Logs streams container logs -- all of the stack's containers merged, or
	// just the given subset (names from Status; empty = all). With follow it
	// tails live until ctx is cancelled; tail bounds the backlog.
	Logs(ctx context.Context, s *stack.Stack, tail int, follow bool, containers []string) (io.ReadCloser, error)
	// Exec opens an interactive shell session in a container. Only session
	// creation is runtime-specific; the WebSocket bridge above is generic.
	Exec(ctx context.Context, opts ExecOptions) (ExecSession, error)
	// Status reports the current lifecycle state of a stack's containers.
	Status(ctx context.Context, s *stack.Stack) (StackStatus, error)
	// Networks lists container networks a stack can attach to for its own IP.
	Networks(ctx context.Context) ([]Network, error)
	// Volumes lists podman-managed named volumes.
	Volumes(ctx context.Context) ([]Volume, error)
	// CreateVolume creates a named volume (local driver, optional NFS opts).
	CreateVolume(ctx context.Context, spec VolumeSpec) (Volume, error)
	// RemoveVolume deletes a named volume; force removes it even if in use.
	RemoveVolume(ctx context.Context, name string, force bool) error
	// UsedPorts maps host ports already published by ANY container (running or
	// created) to the holder's container name -- pre-flight conflict checks.
	// Key format: "8080/tcp".
	UsedPorts(ctx context.Context) (map[string]string, error)
	// DiskUsage reports reclaimable space per category (the runtime's df).
	DiskUsage(ctx context.Context) ([]DiskRow, error)
	// PruneCapabilities reports which prune options this runtime supports.
	PruneCapabilities() PruneCapabilities
	// Prune reclaims space per opts, returning what was freed. A host-wide
	// maintenance op, not tied to a stack.
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
	// ImageRepoDigests returns the "repo@sha256:..." digests a locally-present
	// image is known by (empty if the image isn't pulled). For a multi-arch
	// image this includes the manifest-list/index digest, which is what a
	// registry reports for the tag -- so it's the digest to compare against for
	// update detection. Compare against RepoDigests, never the per-arch .Digest.
	ImageRepoDigests(ctx context.Context, ref string) ([]string, error)
	// Capabilities reports optional features (remote volumes, ...) so generic
	// code branches on a capability, not an engine name.
	Capabilities() Capabilities
	// StoreSMBCredentials records the password an smb:// volume will use.
	// Meaningful only when Capabilities().RemoteVolumes; other engines return
	// an error.
	StoreSMBCredentials(server, username, password string) error
}

// HumanBytes formats a byte count the way podman's df does ("35.0GB"), so
// engines that size things themselves match the podman rows.
func HumanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "kMGTPE"[exp])
}
