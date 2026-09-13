package appjail

import (
	"testing"

	"github.com/daemonless/fjord/pkg/engine"
)

func TestImageRowCountsDanglingAsReclaimable(t *testing.T) {
	row := imageRow([]buildahImage{
		{ID: "a", Names: []string{"ghcr.io/daemonless/navidrome:latest"}, Size: "300 MB"},
		{ID: "b", Size: "100 MB"},
		{ID: "c", Size: "100 MB"},
	})
	if row.Total != 3 || row.Active != 1 {
		t.Fatalf("total/active = %d/%d, want 3/1", row.Total, row.Active)
	}
	if row.RawReclaimable != 200_000_000 || row.Reclaimable != "200.0MB (40%)" {
		t.Fatalf("reclaimable = %d %q", row.RawReclaimable, row.Reclaimable)
	}
	if row.Size != "500.0MB" {
		t.Fatalf("size = %q", row.Size)
	}
}

func TestParseHumanBytes(t *testing.T) {
	for in, want := range map[string]int64{"730 MB": 730_000_000, "1.2GB": 1_200_000_000, "512 B": 512, "3.5 GiB": 3_758_096_384, "": 0, "lots": 0} {
		if got := parseHumanBytes(in); got != want {
			t.Errorf("parseHumanBytes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0B", 999: "999B", 1500: "1.5kB", 2_500_000_000: "2.5GB"} {
		if got := engine.HumanBytes(in); got != want {
			t.Errorf("engine.HumanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
