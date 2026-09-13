package compose

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DropVolumesReferencing removes each service volume mount whose source uses one
// of the given variables. It's used when an optional path variable resolves to
// empty: leaving "${MOVIES_PATH}:/movies" would render ":/movies" and break the
// compose, so the whole mount is dropped instead.
// DropTopLevelKey removes one top-level mapping key (e.g. "name") and
// returns the YAML; unparseable or key-less input comes back unchanged.
func DropTopLevelKey(composeYAML, key string) string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return composeYAML
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			out, err := encodeRoot(root)
			if err != nil {
				return composeYAML
			}
			return out
		}
	}
	return composeYAML
}

// DropPortsReferencing removes every service `ports:` entry that references
// one of vars -- an optional port the user left blank ("${PORT_443}:443"),
// which would otherwise make podman-compose choke on an empty host port.
func DropPortsReferencing(composeYAML string, vars []string) (string, error) {
	return dropSeqEntriesReferencing(composeYAML, "ports", vars)
}

// dropSeqEntriesReferencing removes scalar entries of the per-service sequence
// `key` (volumes, ports) that reference any of vars.
func dropSeqEntriesReferencing(composeYAML, key string, vars []string) (string, error) {
	if len(vars) == 0 {
		return composeYAML, nil
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
	if services == nil || services.Kind != yaml.MappingNode {
		return composeYAML, nil
	}
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		seq := mapGet(svc, key)
		if seq == nil || seq.Kind != yaml.SequenceNode {
			continue
		}
		kept := seq.Content[:0:0]
		for _, item := range seq.Content {
			if item.Kind == yaml.ScalarNode && referencesAny(item.Value, vars) {
				continue
			}
			kept = append(kept, item)
		}
		seq.Content = kept
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

func DropVolumesReferencing(composeYAML string, vars []string) (string, error) {
	if len(vars) == 0 {
		return composeYAML, nil
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
	if services == nil || services.Kind != yaml.MappingNode {
		return composeYAML, nil
	}
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		vols := mapGet(svc, "volumes")
		if vols == nil || vols.Kind != yaml.SequenceNode {
			continue
		}
		kept := vols.Content[:0:0]
		for _, item := range vols.Content {
			if item.Kind == yaml.ScalarNode && referencesAny(item.Value, vars) {
				continue
			}
			kept = append(kept, item)
		}
		vols.Content = kept
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

// referencesAny reports whether s uses any of vars as ${VAR} or $VAR.
func referencesAny(s string, vars []string) bool {
	for _, v := range vars {
		if strings.Contains(s, "${"+v+"}") || strings.Contains(s, "$"+v) {
			return true
		}
	}
	return false
}
