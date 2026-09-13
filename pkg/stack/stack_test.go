package stack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateEngineRecordsOnlyMissing(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	for _, n := range []string{"old", "new"} {
		os.MkdirAll(filepath.Join(dir, n), 0o755)
		os.WriteFile(filepath.Join(dir, n, "compose.yaml"), []byte("services: {}\n"), 0o644)
	}
	m.SaveState("new", &State{SchemaVersion: stateSchemaVersion, Engine: "appjail"})
	migrated, err := m.MigrateEngine("podman")
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 1 || migrated[0] != "old" {
		t.Fatalf("migrated = %v, want [old]", migrated)
	}
	if st, _ := m.LoadState("old"); st == nil || st.Engine != "podman" {
		t.Fatalf("old: engine not recorded: %+v", st)
	}
	if st, _ := m.LoadState("new"); st.Engine != "appjail" {
		t.Fatalf("new: engine changed to %q", st.Engine)
	}
}

func TestAllocateNameSlugsAndSuffixes(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if got := m.AllocateName("Tautulli 4K", "tautulli"); got != "tautulli-4k" {
		t.Fatalf("slug = %q", got)
	}
	if got := m.AllocateName("!!!", "radarr"); got != "radarr" {
		t.Fatalf("fallback = %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "radarr"), 0o755)
	if got := m.AllocateName("Radarr", "radarr"); got != "radarr-2" {
		t.Fatalf("second radarr = %q, want radarr-2", got)
	}
	os.RemoveAll(filepath.Join(dir, "radarr"))
	if got := m.AllocateName("Radarr", "radarr"); got != "radarr" {
		t.Fatalf("after delete = %q, want radarr (name is free again)", got)
	}
}
