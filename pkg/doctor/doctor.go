// Package doctor probes the host for everything fjord's engines need --
// binaries, the runtime socket, kernel facilities -- and reports each as a
// structured pass/fail with a remediation. It exists because a mis-provisioned
// host fails cryptically at stack-up time (a missing catatonit surfaces as
// "no such file or directory" from every stack); the doctor names the real
// problem up front.
//
// Platform knowledge lives only in the platform_<os>.go leaves: each assembles
// its check list from the portable probes in checks.go. Supporting a new OS
// means adding a leaf, never touching this core.
package doctor

import "context"

type Status string

const (
	OK      Status = "ok"
	Warn    Status = "warn"    // degraded: some features won't work
	Fail    Status = "fail"    // the active engine cannot run stacks
	Unknown Status = "unknown" // host state fjordd cannot observe from here
)

// Check is one probe plus its remediation metadata. Checks are data, assembled
// per-platform, so fix commands and package names are OS-specific while the
// core and the API stay OS-agnostic.
type Check struct {
	ID       string
	Name     string
	Engine   string // run only when this engine is active; "" = always
	HostOnly bool   // probes host state invisible from inside a container/jail
	Probe    func(ctx context.Context) (Status, string)
	// Why explains what the check protects, in one or two plain sentences --
	// shown next to every check so an operator knows what a red row costs.
	Why string
	// Fix is a shell snippet, one command per line ("#" lines are comments),
	// reported when not ok. It must paste into a root shell as-is: no prose,
	// no "(optional)" asides -- those belong in Why.
	Fix string
	Pkg map[string]string // GOOS -> package name, for future auto-install
}

// Result is the wire form of one executed check.
type Result struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Why    string `json:"why,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

// Report is the full doctor output. Mode is "host", "container", or "unknown";
// anything but "host" means fjordd cannot repair the machine it manages, so
// remediations are commands for the operator, not actions fjordd can take.
// Unknown deliberately gets container semantics -- never offer a fix path we
// might apply to the wrong place.
type Report struct {
	OS         string   `json:"os"`
	Mode       string   `json:"mode"`
	CanInstall bool     `json:"canInstall"`
	Checks     []Result `json:"checks"`
}

// Config carries the runtime context checks depend on.
type Config struct {
	Engine    string // active engine name (filters engine-specific checks)
	FjordRoot string
}

// platformInfo is what each platform_<os>.go leaf provides.
type platformInfo struct {
	os         string
	mode       string // "host" | "container" | "unknown"
	canInstall bool   // a package manager is present (mode still gates use)
	checks     []Check
}

// Mode reports the deployment mode ("host", "container", "unknown") without
// running the full check suite -- for features that only apply on the host
// (e.g. the host filesystem browser).
func Mode() string {
	return platform(Config{}).mode
}

// Run executes every check applicable to this platform and the active engine.
func Run(ctx context.Context, cfg Config) Report {
	p := platform(cfg)
	results := make([]Result, 0, len(p.checks))
	for _, c := range p.checks {
		if c.Engine != "" && c.Engine != cfg.Engine {
			continue
		}
		var st Status
		var detail string
		if c.HostOnly && p.mode != "host" {
			st = Unknown
			detail = "fjordd is not running directly on the host and cannot verify this -- check manually"
		} else {
			st, detail = c.Probe(ctx)
		}
		r := Result{ID: c.ID, Name: c.Name, Status: st, Detail: detail, Why: c.Why}
		if st != OK {
			r.Fix = c.Fix
		}
		results = append(results, r)
	}
	return Report{
		OS:         p.os,
		Mode:       p.mode,
		CanInstall: p.canInstall && p.mode == "host",
		Checks:     results,
	}
}
