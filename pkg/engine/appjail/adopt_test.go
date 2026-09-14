package appjail

import (
	"strings"
	"testing"
)

// Fixture: uptime-kuma on saturn, made by a director project outside fjord.
func TestConvertJail(t *testing.T) {
	info := jailInfo{
		Name: "uptime_kuma_uptime_kuma", Image: "ghcr.io/daemonless/uptime-kuma:latest", State: "running",
		Network: "ajnet", Address: "10.0.0.3", NAT: true,
		Exposes: []exposeEntry{{"3001", "3001", "tcp"}},
		Mounts:  []mountEntry{{"/containers/uptime-kuma/config", "/config", "<pseudofs>", "rw"}},
		Env:     map[string]string{"PUID": "1000", "PGID": "1000", "TZ": "UTC", "DATA_DIR": "/config"},
		User:    "root",
		Params: []string{
			`exec.start: "/bin/sh /etc/rc"`, `exec.stop: "/bin/sh /etc/rc.shutdown jail"`, "mount.devfs", "persist", "allow.raw_sockets",
			"vnet", `vnet.interface: "eb_1568ea0a609"`,
			`exec.prestart: "appjail network plug -e \"1568ea0a609\" -n \"ajnet\""`,
			`exec.poststart: "appjail network assign -d -e \"1568ea0a609\" -j \"${name}\" -n \"ajnet\""`,
			`exec.poststop: "appjail network unplug \"ajnet\" \"1568ea0a609\""`,
			`exec.prestart+: "appjail nat on jail \"${name}\""`, `exec.poststop+: "appjail nat off jail \"${name}\""`,
			`exec.prestart+: "appjail expose on \"${name}\""`, `exec.poststop+: "appjail expose off \"${name}\""`,
		},
	}
	spec := convertJail(info)
	if spec.Service != "uptime-kuma" {
		t.Errorf("service = %q", spec.Service)
	}
	for _, want := range []string{
		"  - virtualnet: 'ajnet:<random> default address:10.0.0.3'\n", "  - nat:\n",
		"    name: uptime_kuma_uptime_kuma\n", "      - expose: '3001:3001 proto:tcp'\n",
		"      user: root\n", "        - PUID: !ENV '${PUID}'\n", "        - DATA_DIR: /config\n",
		"      - UPTIME_KUMA_CONFIG_PATH: /config\n", "    device: !ENV '${UPTIME_KUMA_CONFIG_PATH}'\n",
	} {
		if !strings.Contains(spec.Director, want) {
			t.Errorf("director missing %q:\n%s", want, spec.Director)
		}
	}
	if !strings.Contains(spec.Makejail, "OPTION from=ghcr.io/daemonless/uptime-kuma:${tag}\n") || !strings.Contains(spec.Makejail, "ARG tag=latest\n") {
		t.Errorf("makejail:\n%s", spec.Makejail)
	}
	want := "# template.conf\n\nexec.start: \"/bin/sh /etc/rc\"\nexec.stop: \"/bin/sh /etc/rc.shutdown jail\"\nmount.devfs\npersist\nallow.raw_sockets\n"
	if spec.Template != want {
		t.Errorf("template kept plumbing:\n%s", spec.Template)
	}
	for _, want := range []string{"container_name: uptime_kuma_uptime_kuma\n", "      - \"${UPTIME_KUMA_CONFIG_PATH}:/config\"\n", "      - \"3001:3001\"\n", "      - PUID=${PUID}\n"} {
		if !strings.Contains(spec.Compose, want) {
			t.Errorf("compose missing %q:\n%s", want, spec.Compose)
		}
	}
	if spec.Env != "PGID=1000\nPUID=1000\nTZ=UTC\nUPTIME_KUMA_CONFIG_PATH=/containers/uptime-kuma/config\n" {
		t.Errorf("env:\n%s", spec.Env)
	}
	if len(spec.Notes) != 0 {
		t.Errorf("notes: %v", spec.Notes)
	}
}
