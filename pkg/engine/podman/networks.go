package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/daemonless/fjord/pkg/engine"
)

// libpodNetwork is the subset of the libpod /networks/json response we need.
type libpodNetwork struct {
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Subnets []struct {
		Subnet  string `json:"subnet"`
		Gateway string `json:"gateway"`
	} `json:"subnets"`
}

// Networks lists macvlan networks over the libpod socket. Only macvlan is
// returned: those give a container its own routable IP on the LAN, which is
// what lets a stack bind :80/:443 without colliding on the host's ports.
// Bridge/default networks don't solve that, so they're omitted.
func (b *Backend) Networks(ctx context.Context) ([]engine.Network, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v4.0.0/libpod/networks/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("libpod networks/json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libpod networks/json: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseNetworks(data)
}

// parseNetworks filters the libpod network list to attachable macvlan networks.
// Split out from the HTTP call so it can be unit-tested without a socket.
func parseNetworks(data []byte) ([]engine.Network, error) {
	var raw []libpodNetwork
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode libpod networks: %w", err)
	}
	out := make([]engine.Network, 0, len(raw))
	for _, n := range raw {
		if n.Driver != "macvlan" {
			continue
		}
		net := engine.Network{Name: n.Name, Driver: n.Driver}
		if len(n.Subnets) > 0 {
			net.Subnet = n.Subnets[0].Subnet
			net.Gateway = n.Subnets[0].Gateway
		}
		out = append(out, net)
	}
	return out, nil
}
