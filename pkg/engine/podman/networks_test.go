package podman

import "testing"

// Real libpod /networks/json fixture (trimmed) with a bridge, two macvlans,
// and the default podman bridge -- only the macvlans should survive.
const networksFixture = `[
  {"name":"booklore-internal","driver":"bridge","subnets":[{"subnet":"10.89.1.0/24","gateway":"10.89.1.1"}]},
  {"name":"vlan4","driver":"macvlan","subnets":[{"subnet":"192.168.4.0/24","gateway":"192.168.4.1"}]},
  {"name":"vlan5","driver":"macvlan","subnets":[{"subnet":"192.168.5.0/24","gateway":"192.168.5.1"}]},
  {"name":"podman","driver":"bridge","subnets":[{"subnet":"10.88.0.0/16","gateway":"10.88.0.1"}]}
]`

func TestParseNetworksFiltersMacvlan(t *testing.T) {
	nets, err := parseNetworks([]byte(networksFixture))
	if err != nil {
		t.Fatalf("parseNetworks: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("expected 2 macvlan networks, got %d: %+v", len(nets), nets)
	}
	if nets[0].Name != "vlan4" || nets[1].Name != "vlan5" {
		t.Fatalf("unexpected names: %+v", nets)
	}
	if nets[1].Subnet != "192.168.5.0/24" || nets[1].Gateway != "192.168.5.1" {
		t.Fatalf("subnet/gateway not extracted: %+v", nets[1])
	}
}

func TestParseNetworksEmpty(t *testing.T) {
	nets, err := parseNetworks([]byte(`[]`))
	if err != nil {
		t.Fatalf("parseNetworks: %v", err)
	}
	if len(nets) != 0 {
		t.Fatalf("expected no networks, got %+v", nets)
	}
}
