package podman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteNsmbSectionManagedBlock(t *testing.T) {
	dir := t.TempDir()
	nsmbConf = filepath.Join(dir, "nsmb.conf")
	defer func() { nsmbConf = "/etc/nsmb.conf" }()
	os.WriteFile(nsmbConf, []byte("# user's own config\n[default]\nworkgroup=HOME\n"), 0o600)

	if err := writeNsmbSection("mars", "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := writeNsmbSection("mars", "alice", "changed"); err != nil { // rewrite, not duplicate
		t.Fatal(err)
	}
	if err := writeNsmbSection("nas2", "bob", "pw"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(nsmbConf)
	got := string(b)
	for _, want := range []string{"[default]\nworkgroup=HOME", "[MARS]\naddr=mars", "[MARS:ALICE]", "[NAS2:BOB]", nsmbBegin, nsmbEnd} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, "[MARS:ALICE]") != 1 || strings.Count(got, nsmbBegin) != 1 {
		t.Errorf("sections duplicated:\n%s", got)
	}
	if strings.Contains(got, "password=s3cret") {
		t.Errorf("old password survived the rewrite:\n%s", got)
	}
	if !hasSMBCredentials("mars", "alice") || hasSMBCredentials("mars", "nobody") {
		t.Errorf("hasSMBCredentials wrong")
	}
	if fi, _ := os.Stat(nsmbConf); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
}
