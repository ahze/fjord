// Package manifest parses x-fjord catalog manifests (a compose file plus an
// x-fjord metadata block) and resolves a user's wizard input into the .env
// values and host directories needed to render a runnable stack.
package manifest

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Var is a wizard variable declared under x-fjord.variables.
type Var struct {
	Name     string
	Type     string // port | string | secret | path | zfs_dataset
	Default  string
	Optional bool
	Uid, Gid int    // host_permissions for zfs_dataset (0 -> default 1000)
	Mode     string // octal, e.g. "755"
}

// Manifest is a parsed x-fjord manifest: the compose half (x-fjord stripped,
// ${VAR} placeholders intact) plus the variable declarations. WebPort/WebHTTPS
// carry the catalog's web-endpoint hint (from the image's cit config) -- may be
// a literal port or a "${VAR}" reference resolved from the stack's .env.
type Manifest struct {
	compose   string
	appjail   *AppjailBundle
	Variables []Var
	WebPort   string
	WebHTTPS  bool
}

// AppjailBundle is the dbuild-rendered AppJail deploy bundle carried inline in
// the manifest under x-fjord.appjail. The appjail engine runs these verbatim
// via appjail-director instead of translating the compose itself; fjord fills
// the placeholder .env (WEB_PORT + one var per volume device) at install.
type AppjailBundle struct {
	Director     string `yaml:"director"`
	Makejail     string `yaml:"makejail"`
	TemplateConf string `yaml:"template_conf"`
	EnvDefaults  string `yaml:"env_defaults"`
	// Extras are further bundle files (a sidecar's own jail template),
	// written next to director.yml under their own names.
	Extras map[string]string `yaml:"extras"`
}

// Compose returns the compose YAML with the x-fjord block removed and the
// ${VAR} placeholders left in place (podman-compose substitutes them from .env).
func (m *Manifest) Compose() string { return m.compose }

// Appjail returns the AppJail director bundle, or nil when the app declares no
// appjail support (appjail: false, or no bundle was rendered at catalog time).
func (m *Manifest) Appjail() *AppjailBundle { return m.appjail }

// xfVar mirrors the on-disk x-fjord variable shape for decoding.
type xfVar struct {
	Name            string `yaml:"name"`
	Type            string `yaml:"type"`
	Default         string `yaml:"default"`
	Optional        bool   `yaml:"optional"`
	HostPermissions *struct {
		Uid  int    `yaml:"uid"`
		Gid  int    `yaml:"gid"`
		Mode string `yaml:"mode"`
	} `yaml:"host_permissions"`
}

// Parse splits a manifest into its compose half and its variables.
func Parse(manifestYAML string) (*Manifest, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(manifestYAML), &doc); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("manifest is not a YAML mapping")
	}
	root := doc.Content[0]

	// Split x-fjord out of the top-level mapping; keep everything else as compose.
	var xfNode *yaml.Node
	kept := make([]*yaml.Node, 0, len(root.Content))
	for i := 0; i+1 < len(root.Content); i += 2 {
		k, v := root.Content[i], root.Content[i+1]
		if k.Value == "x-fjord" {
			xfNode = v
			continue
		}
		// Drop the top-level compose project name (the manifest's app id); the
		// stack dir name becomes the project, so status filters + container
		// names track the stack, not the app id.
		if k.Value == "name" {
			continue
		}
		kept = append(kept, k, v)
	}
	if xfNode == nil {
		return nil, fmt.Errorf("manifest has no x-fjord section")
	}
	root.Content = kept

	// Catalog manifests hardcode container_name (e.g. "radarr"), which forces a
	// fixed name -- colliding with any existing container of that name and
	// preventing a second instance. Strip it so each install is self-contained;
	// podman-compose then names the container <stack>_<service>_1. Service-name
	// network aliases are unaffected.
	stripContainerNames(root)

	var xf struct {
		Info struct {
			WebPort  string `yaml:"web_port"`
			WebHTTPS bool   `yaml:"web_https"`
		} `yaml:"info"`
		Variables []xfVar        `yaml:"variables"`
		Appjail   *AppjailBundle `yaml:"appjail"`
	}
	if err := xfNode.Decode(&xf); err != nil {
		return nil, fmt.Errorf("decode x-fjord: %w", err)
	}
	vars := make([]Var, 0, len(xf.Variables))
	for _, v := range xf.Variables {
		nv := Var{Name: v.Name, Type: v.Type, Default: v.Default, Optional: v.Optional}
		if v.HostPermissions != nil {
			nv.Uid, nv.Gid, nv.Mode = v.HostPermissions.Uid, v.HostPermissions.Gid, v.HostPermissions.Mode
		}
		vars = append(vars, nv)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	enc.Close()

	return &Manifest{compose: buf.String(), appjail: xf.Appjail, Variables: vars, WebPort: xf.Info.WebPort, WebHTTPS: xf.Info.WebHTTPS}, nil
}

// stripContainerNames removes container_name from every service mapping.
func stripContainerNames(root *yaml.Node) {
	services := childByKey(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 1; i < len(services.Content); i += 2 {
		if svc := services.Content[i]; svc.Kind == yaml.MappingNode {
			removeKey(svc, "container_name")
		}
	}
}

func childByKey(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func removeKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}
