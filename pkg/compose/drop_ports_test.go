package compose

import (
	"strings"
	"testing"
)

// An optional port left blank is dropped from ports:, nothing else moves.
func TestDropPortsReferencing(t *testing.T) {
	in := "services:\n  haproxy:\n    ports:\n      - \"${WEB_PORT}:80\"\n      - \"${PORT_443}:443\"\n      - \"${PORT_8404}:8404\"\n    volumes:\n      - \"${CONFIG_DATA}:/config\"\n"
	out, err := DropPortsReferencing(in, []string{"PORT_443", "PORT_8404"})
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"PORT_443", "PORT_8404"} {
		if strings.Contains(out, gone) {
			t.Errorf("%s still published:\n%s", gone, out)
		}
	}
	for _, kept := range []string{"${WEB_PORT}:80", "${CONFIG_DATA}:/config"} {
		if !strings.Contains(out, kept) {
			t.Errorf("%s lost:\n%s", kept, out)
		}
	}
	if same, _ := DropPortsReferencing(in, nil); same != in {
		t.Error("no vars must be a no-op")
	}
}
