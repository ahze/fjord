package stack

import (
	"os"
	"path/filepath"
	"testing"
)

// newStackDir makes a manager over a temp dir with one empty stack dir ready.
func newStackDir(t *testing.T, name string) *Manager {
	t.Helper()
	m := NewManager(t.TempDir())
	if err := os.MkdirAll(filepath.Join(m.StacksDir, name), 0755); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLoadStateAbsent(t *testing.T) {
	m := newStackDir(t, "demo")
	st, err := m.LoadState("demo")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st != nil {
		t.Fatalf("expected nil state for a stack with no state.json, got %+v", st)
	}
}

func TestEnsureStateThenSetDesired(t *testing.T) {
	m := newStackDir(t, "demo")

	// First save creates state.json defaulting to stopped, with InstalledAt set.
	if err := m.EnsureState("demo"); err != nil {
		t.Fatalf("EnsureState: %v", err)
	}
	st, _ := m.LoadState("demo")
	if st == nil || st.DesiredState != "stopped" {
		t.Fatalf("expected desired_state stopped, got %+v", st)
	}
	if st.SchemaVersion != stateSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", st.SchemaVersion, stateSchemaVersion)
	}
	installed := st.InstalledAt
	if installed == "" {
		t.Fatal("InstalledAt not set")
	}

	// up -> running; down -> stopped; InstalledAt must not change.
	if err := m.SetDesiredState("demo", "running"); err != nil {
		t.Fatalf("SetDesiredState running: %v", err)
	}
	st, _ = m.LoadState("demo")
	if st.DesiredState != "running" {
		t.Fatalf("desired_state = %q, want running", st.DesiredState)
	}
	if st.InstalledAt != installed {
		t.Fatalf("InstalledAt changed: %q -> %q", installed, st.InstalledAt)
	}

	if err := m.SetDesiredState("demo", "stopped"); err != nil {
		t.Fatalf("SetDesiredState stopped: %v", err)
	}
	st, _ = m.LoadState("demo")
	if st.DesiredState != "stopped" {
		t.Fatalf("desired_state = %q, want stopped", st.DesiredState)
	}
}

// SaveState must not leave a temp file behind after an atomic write.
func TestSaveStateAtomicNoTemp(t *testing.T) {
	m := newStackDir(t, "demo")
	if err := m.SetDesiredState("demo", "running"); err != nil {
		t.Fatalf("SetDesiredState: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(m.StacksDir, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover temp file after atomic write: %s", e.Name())
		}
	}
}

func TestSetDesiredStateInvalidName(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.SetDesiredState("..", "running"); err == nil {
		t.Fatal("expected error for traversal name, got nil")
	}
}

func TestGetSaveRejectInvalidName(t *testing.T) {
	m := NewManager(t.TempDir())
	for _, name := range []string{"..", "my stack", ".hidden", "a/b", ""} {
		if _, err := m.Get(name); err != ErrInvalidName {
			t.Errorf("Get(%q): want ErrInvalidName, got %v", name, err)
		}
		if err := m.Save(&Stack{Name: name, Compose: "services: {}\n"}); err != ErrInvalidName {
			t.Errorf("Save(%q): want ErrInvalidName, got %v", name, err)
		}
	}
	// Nothing may have been written outside a valid stack dir.
	if _, err := os.Stat(filepath.Join(m.StacksDir, "compose.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Save(\"..\") escaped the stacks dir: %v", err)
	}
	if err := m.Save(&Stack{Name: "ok-1", Compose: "services: {}\n"}); err != nil {
		t.Fatalf("Save(valid): %v", err)
	}
}

func TestValidDisplayName(t *testing.T) {
	for _, ok := range []string{"radarr", "Tautulli 4K", "Radarr-4k", "Fotos.privé", "x"} {
		if !ValidDisplayName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", " lead", "a/b", "a\nb", "-dash", "../x", "way too long name that goes well past the sixty four character limit set for it"} {
		if ValidDisplayName(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
