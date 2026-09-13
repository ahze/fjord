package compose

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// NamedVolumes returns the stack's top-level named volumes (the ones its
// services can mount), e.g. {"media", "backups"}.
func NamedVolumes(composeYAML string) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	vols := mapGet(root, "volumes")
	if vols == nil || vols.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i < len(vols.Content); i += 2 {
		out = append(out, vols.Content[i].Value)
	}
	return out
}

// AttachVolume mounts an existing named volume into the first service at
// containerPath: adds "<name>:<path>[:ro]" to the service's volumes and
// declares the volume external at the top level (fjord volumes are created via
// the engine, not by compose).
func AttachVolume(composeYAML, volName, containerPath string, readOnly bool) (string, error) {
	if volName == "" || !strings.HasPrefix(containerPath, "/") {
		return "", fmt.Errorf("volume name and an absolute container path are required")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parse compose: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("compose is not a YAML mapping")
	}
	root := doc.Content[0]
	services := mapGet(root, "services")
	if services == nil || services.Kind != yaml.MappingNode || len(services.Content) < 2 {
		return "", fmt.Errorf("compose has no services")
	}
	svc := services.Content[1]
	if svc.Kind != yaml.MappingNode {
		return "", fmt.Errorf("first service is not a mapping")
	}

	entry := volName + ":" + containerPath
	if readOnly {
		entry += ":ro"
	}
	svcVols := mapGet(svc, "volumes")
	if svcVols == nil {
		svc.Content = append(svc.Content, scalar("volumes"),
			&yaml.Node{Kind: yaml.SequenceNode})
		svcVols = svc.Content[len(svc.Content)-1]
	}
	if svcVols.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("service volumes is not a list")
	}
	for _, it := range svcVols.Content {
		if it.Kind == yaml.ScalarNode && strings.HasPrefix(it.Value, volName+":") {
			return "", fmt.Errorf("volume %q is already attached", volName)
		}
	}
	svcVols.Content = append(svcVols.Content, quotedScalar(entry))

	// Top-level: volumes: <name>: {external: true}
	topVols := mapGet(root, "volumes")
	if topVols == nil {
		root.Content = append(root.Content, scalar("volumes"),
			&yaml.Node{Kind: yaml.MappingNode})
		topVols = root.Content[len(root.Content)-1]
	}
	if topVols.Kind != yaml.MappingNode {
		return "", fmt.Errorf("top-level volumes is not a mapping")
	}
	if mapGet(topVols, volName) == nil {
		ext := &yaml.Node{Kind: yaml.MappingNode}
		ext.Content = append(ext.Content, scalar("external"),
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
		topVols.Content = append(topVols.Content, scalar(volName), ext)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return "", err
	}
	enc.Close()
	return buf.String(), nil
}

// AttachBindMount mounts a host path into the first service at containerPath as
// a short-form "host:container[:ro]" entry. Unlike AttachVolume it declares no
// top-level volume -- a bind references the host path directly.
func AttachBindMount(composeYAML, hostPath, containerPath string, readOnly bool) (string, error) {
	if !strings.HasPrefix(hostPath, "/") || !strings.HasPrefix(containerPath, "/") {
		return "", fmt.Errorf("both host and container paths must be absolute")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parse compose: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("compose is not a YAML mapping")
	}
	root := doc.Content[0]
	services := mapGet(root, "services")
	if services == nil || services.Kind != yaml.MappingNode || len(services.Content) < 2 {
		return "", fmt.Errorf("compose has no services")
	}
	svc := services.Content[1]
	if svc.Kind != yaml.MappingNode {
		return "", fmt.Errorf("first service is not a mapping")
	}
	svcVols := mapGet(svc, "volumes")
	if svcVols == nil {
		svc.Content = append(svc.Content, scalar("volumes"), &yaml.Node{Kind: yaml.SequenceNode})
		svcVols = svc.Content[len(svc.Content)-1]
	}
	if svcVols.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("service volumes is not a list")
	}
	for _, it := range svcVols.Content {
		if it.Kind == yaml.ScalarNode && mountDest(it.Value) == containerPath {
			return "", fmt.Errorf("%q is already mounted", containerPath)
		}
	}
	entry := hostPath + ":" + containerPath
	if readOnly {
		entry += ":ro"
	}
	svcVols.Content = append(svcVols.Content, quotedScalar(entry))
	return encodeRoot(root)
}

// RemoveMount drops every short-form service mount whose container path equals
// containerPath (across all services). The top-level named-volume declaration,
// if any, is left in place -- harmless when unreferenced.
func RemoveMount(composeYAML, containerPath string) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parse compose: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("compose is not a YAML mapping")
	}
	root := doc.Content[0]
	services := mapGet(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return "", fmt.Errorf("compose has no services")
	}
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		vols := mapGet(svc, "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		kept := vols.Content[:0]
		for _, it := range vols.Content {
			if it.Kind == yaml.ScalarNode && mountDest(it.Value) == containerPath {
				continue // drop it
			}
			kept = append(kept, it)
		}
		vols.Content = kept
	}
	return encodeRoot(root)
}

// FolderMount is one host folder to mount under a variable's mount point, at
// the sub-folder Sub (callers make Subs unique -- see SubfolderName).
type FolderMount struct {
	Host string
	Sub  string
}

// SubfolderName is the default sub-folder for a host path: its basename,
// lowercased and made path-safe ("/mars/sea/Movies-FR" -> "movies-fr").
func SubfolderName(p string) string { return pathBase(strings.TrimRight(p, "/")) }

// ExpandFolderMounts replaces the single service volume that references
// ${varName} (e.g. "${MOVIES_PATH}:/movies") with one bind per host path, mounted
// as "<host>:<dest>/<basename>[:opts]" -- N folders into one app mount point
// (the wizard's multi-folder lists / folder sets). If nothing references the
// var, the compose is returned unchanged. Basenames that collide are the
// caller's problem; use ExpandFolderMountsNamed with unique Subs.
func ExpandFolderMounts(composeYAML, varName string, hosts []string, readOnly bool) (string, error) {
	mounts := make([]FolderMount, 0, len(hosts))
	for _, h := range hosts {
		mounts = append(mounts, FolderMount{Host: h, Sub: SubfolderName(h)})
	}
	return ExpandFolderMountsNamed(composeYAML, varName, mounts, readOnly)
}

// ExpandFolderMountsNamed is ExpandFolderMounts with explicit sub-folder names.
func ExpandFolderMountsNamed(composeYAML, varName string, mounts []FolderMount, readOnly bool) (string, error) {
	if len(mounts) == 0 {
		return composeYAML, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parse compose: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("compose is not a YAML mapping")
	}
	services := mapGet(doc.Content[0], "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return composeYAML, nil
	}
	ref := "${" + varName + "}"
	for i := 1; i < len(services.Content); i += 2 {
		vols := mapGet(services.Content[i], "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		// Fresh slice, NOT vols.Content[:0]: this GROWS (1 entry -> N), so aliasing
		// the backing array would overwrite unread entries mid-iteration.
		out := make([]*yaml.Node, 0, len(vols.Content)+len(mounts))
		for _, it := range vols.Content {
			if it.Kind != yaml.ScalarNode || !strings.Contains(it.Value, ref) {
				out = append(out, it)
				continue
			}
			parts := strings.SplitN(it.Value, ":", 3)
			dest := ""
			if len(parts) >= 2 {
				dest = parts[1]
			}
			opts := ""
			if len(parts) == 3 {
				opts = parts[2]
			}
			if readOnly && !strings.Contains(opts, "ro") {
				opts = strings.TrimPrefix(opts+",ro", ",")
			}
			for _, m := range mounts {
				entry := strings.TrimRight(m.Host, "/") + ":" + strings.TrimRight(dest, "/") + "/" + m.Sub
				if opts != "" {
					entry += ":" + opts
				}
				out = append(out, quotedScalar(entry))
			}
		}
		vols.Content = out
	}
	return encodeRoot(doc.Content[0])
}

// pathBase returns the last path component (basename), lowercased and safe as a
// container sub-directory: "/mars/sea/Movies-FR" -> "movies-fr".
func pathBase(p string) string {
	i := strings.LastIndexByte(p, '/')
	base := p[i+1:]
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '-'
		}
	}, base)
}

// mountDest returns the container path of a short-form "src:dst[:opts]" entry.
// A leading Windows-style drive letter isn't a concern on FreeBSD/Linux hosts.
func mountDest(entry string) string {
	parts := strings.Split(entry, ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// encodeRoot serializes a compose root node back to YAML with 2-space indent.
func encodeRoot(root *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return "", err
	}
	enc.Close()
	return buf.String(), nil
}

// Mount is one resolved storage mount of a stack, classified for the UI.
type Mount struct {
	Source   string `json:"source"` // host path (bind) or volume name
	Dest     string `json:"dest"`   // container path
	ReadOnly bool   `json:"readOnly"`
	Kind     string `json:"kind"` // "bind" | "volume"
	Service  string `json:"service"`
}

// Mounts lists every service mount, classifying each as a named volume (source
// is a top-level volume) or a host-path bind, with ${VAR}s in bind sources
// resolved against env for display. Unlike ParseServices (which keeps only
// absolute-source binds for dir provisioning), this keeps named volumes too --
// a mount like "bfee:/data" must still show in the Resources table.
func Mounts(composeYAML string, env map[string]string) []Mount {
	named := map[string]bool{}
	for _, n := range NamedVolumes(composeYAML) {
		named[n] = true
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	services := mapGet(doc.Content[0], "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	var out []Mount
	for i := 0; i+1 < len(services.Content); i += 2 {
		svcName := services.Content[i].Value
		vols := mapGet(services.Content[i+1], "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		for _, it := range vols.Content {
			if it.Kind != yaml.ScalarNode {
				continue // long-form mapping entries aren't shown yet
			}
			parts := strings.SplitN(it.Value, ":", 3)
			if len(parts) < 2 {
				continue
			}
			m := Mount{Dest: parts[1], Kind: "bind", Service: svcName, Source: ExpandEnv(parts[0], env)}
			if named[parts[0]] {
				m.Kind = "volume"
				m.Source = parts[0] // referenced by name, not a path
			}
			if len(parts) == 3 && strings.Contains(parts[2], "ro") {
				m.ReadOnly = true
			}
			out = append(out, m)
		}
	}
	return out
}

// quotedScalar returns a double-quoted string scalar node.
func quotedScalar(v string) *yaml.Node {
	n := scalar(v)
	n.Style = yaml.DoubleQuotedStyle
	return n
}

// BindMount is one host-path bind mount a service declares.
type BindMount struct {
	Source string // absolute host path, ${VAR}s resolved
	Dest   string
}

// BindMounts returns every service's host-path bind mounts (short-form
// "src:dst[:opts]" entries whose source resolves to an absolute path),
// resolving ${VAR} against env. Named-volume mounts are not included.
func BindMounts(composeYAML string, env map[string]string) []BindMount {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	services := mapGet(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	resolve := func(s string) string { return ExpandEnv(s, env) }
	var out []BindMount
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		vols := mapGet(svc, "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range vols.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			parts := strings.Split(resolve(strings.Trim(item.Value, `"`)), ":")
			if len(parts) < 2 || !strings.HasPrefix(parts[0], "/") {
				continue // named volume or unresolved -- not a host bind
			}
			out = append(out, BindMount{Source: parts[0], Dest: parts[1]})
		}
	}
	return out
}

// PublishedPort is one host-side port a stack wants to publish.
type PublishedPort struct {
	Host  int
	Proto string // "tcp" or "udp"
}

// PublishedPorts returns the host ports every service publishes, resolving
// ${VAR} references against env. Long-form ({published, target}) entries and
// unresolvable values are skipped (best-effort -- this feeds pre-flight
// warnings, not enforcement).
func PublishedPorts(composeYAML string, env map[string]string) []PublishedPort {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	services := mapGet(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	resolve := func(s string) string { return ExpandEnv(s, env) }
	var out []PublishedPort
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		ports := mapGet(svc, "ports")
		if ports == nil || ports.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range ports.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			spec := resolve(strings.Trim(item.Value, `"`))
			proto := "tcp"
			if s := strings.IndexByte(spec, '/'); s >= 0 {
				proto = spec[s+1:]
				spec = spec[:s]
			}
			host, _ := SplitPortSpec(spec)
			if n, err := strconv.Atoi(host); err == nil && n > 0 {
				out = append(out, PublishedPort{Host: n, Proto: proto})
			}
		}
	}
	return out
}

// MountTargetOf returns the container path of the first service volume entry
// that references ${varName} ("${MOVIES_PATH}:/movies" -> "/movies"), or "".
func MountTargetOf(composeYAML, varName string) string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 {
		return ""
	}
	services := mapGet(doc.Content[0], "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return ""
	}
	ref := "${" + varName + "}"
	for i := 1; i < len(services.Content); i += 2 {
		vols := mapGet(services.Content[i], "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		for _, it := range vols.Content {
			if it.Kind == yaml.ScalarNode && strings.Contains(it.Value, ref) {
				return mountDest(it.Value)
			}
		}
	}
	return ""
}
