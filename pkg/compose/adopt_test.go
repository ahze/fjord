package compose

import (
	"strings"
	"testing"
)

// jupiter's radarr, as podman recorded it (.Config.CreateCommand).
var radarrRun = strings.Fields(`podman run -d --name radarr --hostname radarr --network vlan5 --ip 192.168.5.10 --mac-address 0e:05:00:00:00:0a --annotation org.freebsd.jail.allow.mlock=true -e PUID=1000 -e PGID=1000 -v /containers/radarr:/config -v /mars/tide:/tide -v /mars/sea/TV:/sea/TV -v /downloads:/downloads --restart always ghcr.io/daemonless/radarr:latest`)

func TestFromRunArgsRadarr(t *testing.T) {
	a, err := FromRunArgs(radarrRun)
	if err != nil {
		t.Fatal(err)
	}
	if a.Service != "radarr" {
		t.Fatalf("service = %q", a.Service)
	}
	for _, want := range []string{
		"container_name: radarr", "hostname: radarr", "restart: always",
		"mac_address: 0e:05:00:00:00:0a", "ipv4_address: 192.168.5.10",
		"networks:\n  vlan5:\n    external: true",
		"org.freebsd.jail.allow.mlock: \"true\"",
		`"/containers/radarr:/config"`, `"/mars/sea/TV:/sea/TV"`,
		"- PUID=${PUID}", "- PGID=${PGID}",
	} {
		if !strings.Contains(a.Compose, want) {
			t.Errorf("compose missing %q:\n%s", want, a.Compose)
		}
	}
	if a.Env != "PGID=1000\nPUID=1000\n" {
		t.Errorf("env = %q", a.Env)
	}
	if len(a.Notes) != 0 {
		t.Errorf("unexpected notes: %v", a.Notes)
	}
	// The generated compose must round-trip through fjord's own parser.
	svcs := ParseServices(a.Compose, map[string]string{"PUID": "1000", "PGID": "1000"})
	if len(svcs) != 1 || svcs[0].Image != "ghcr.io/daemonless/radarr:latest" || len(svcs[0].Volumes) != 4 {
		t.Fatalf("parsed services: %+v", svcs)
	}
}

func TestFromRunArgsUnknownFlagIsReported(t *testing.T) {
	a, err := FromRunArgs(strings.Fields(`--name x --weird-flag value --network host -p 8080:80 ghcr.io/daemonless/caddy:latest caddy run`))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 1 || !strings.Contains(a.Notes[0], "--weird-flag value") {
		t.Fatalf("notes = %v", a.Notes)
	}
	for _, want := range []string{"network_mode: host", `"8080:80"`, `command: ["caddy", "run"]`} {
		if !strings.Contains(a.Compose, want) {
			t.Errorf("missing %q:\n%s", want, a.Compose)
		}
	}
}

func TestFromRunArgsNetworkNone(t *testing.T) {
	a, err := FromRunArgs([]string{"podman", "run", "-d", "--name", "seerr", "--network", "none", "ghcr.io/daemonless/seerr:latest"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Compose, "network_mode: none\n") {
		t.Errorf("want network_mode: none, got:\n%s", a.Compose)
	}
	if strings.Contains(a.Compose, "external: true") {
		t.Errorf("none must not become an external network:\n%s", a.Compose)
	}
}
