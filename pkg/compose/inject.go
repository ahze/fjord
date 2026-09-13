// Package compose provides minimal, structure-preserving edits to a stack's
// compose.yaml -- enough for fjord to wire up host provisioning (networks
// today) without taking over authorship of the file.
package compose

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// InjectNetwork attaches an existing external container network to a stack's
// compose so its service gets its own routable IP instead of publishing on the
// host's ports. It adds a top-level `networks: {<network>: {external: true}}`
// and, on each service, a `networks:` entry.
//
// If ip is non-empty it sets ipv4_address (a predictable IP), which requires
// exactly one service since an IP can't be shared. If ip is empty the network
// is attached in list form and the host-local IPAM auto-assigns an IP.
//
// The yaml.Node round-trip preserves comments and key order. A service that
// already declares `networks` is left for the user rather than guessing a
// merge -- InjectNetwork returns an error naming it.
func InjectNetwork(composeYAML, network, ip string) (string, error) {
	if network == "" {
		return "", fmt.Errorf("network name is required")
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
		return "", fmt.Errorf("compose has no services")
	}

	// Collect (name, node) for each service; service values sit at odd indices.
	type svc struct {
		name string
		node *yaml.Node
	}
	var svcs []svc
	for i := 0; i+1 < len(services.Content); i += 2 {
		svcs = append(svcs, svc{services.Content[i].Value, services.Content[i+1]})
	}
	if len(svcs) == 0 {
		return "", fmt.Errorf("compose has no services")
	}
	if ip != "" && len(svcs) != 1 {
		return "", fmt.Errorf("a predictable IP requires exactly one service, found %d", len(svcs))
	}

	// Top-level networks: {<network>: {external: true}} (idempotent).
	networks := mapEnsure(root, "networks")
	if mapGet(networks, network) == nil {
		mapSet(networks, network, externalNetworkNode())
	}

	for _, s := range svcs {
		if s.node.Kind != yaml.MappingNode {
			return "", fmt.Errorf("service %q is not a mapping", s.name)
		}
		if mapGet(s.node, "networks") != nil {
			return "", fmt.Errorf("service %q already declares networks; remove it to auto-attach", s.name)
		}
		mapSet(s.node, "networks", serviceNetworkNode(network, ip))
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

// --- yaml.Node helpers ---

func mapGet(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mapSet(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content, scalar(key), val)
}

// mapEnsure returns the mapping node at key, creating an empty one if absent.
func mapEnsure(m *yaml.Node, key string) *yaml.Node {
	if v := mapGet(m, key); v != nil {
		return v
	}
	nv := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, scalar(key), nv)
	return nv
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// externalNetworkNode builds {external: true}.
func externalNetworkNode() *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	n.Content = append(n.Content, scalar("external"),
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	return n
}

// serviceNetworkNode builds the per-service networks value, using the two forms
// verified against podman-compose on FreeBSD: list form for auto-assign,
// mapping form with ipv4_address for a predictable IP.
func serviceNetworkNode(network, ip string) *yaml.Node {
	if ip == "" {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		seq.Content = append(seq.Content, scalar(network))
		return seq
	}
	inner := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	inner.Content = append(inner.Content, scalar("ipv4_address"), scalar(ip))
	outer := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	outer.Content = append(outer.Content, scalar(network), inner)
	return outer
}
