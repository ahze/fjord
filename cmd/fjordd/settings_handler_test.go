package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The old single "storageBase" key must load as the first App data location,
// and the legacy library shape must load as folder sets.
func TestLoadSettingsMigratesLegacyKeys(t *testing.T) {
	root := t.TempDir()
	legacy := `{"storageBase": "/containers", "libraries": [{"label": "Movies", "backings": [{"kind": "local", "host": "/mnt/movies"}]}]}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s := loadSettings(root)
	if len(s.AppData) != 1 || s.AppData[0] != "/containers" || s.StorageBase != "" {
		t.Fatalf("appData = %v, storageBase = %q", s.AppData, s.StorageBase)
	}
	if len(s.FolderSets) != 1 || s.FolderSets[0].Name != "Movies" || len(s.FolderSets[0].Folders) != 1 {
		t.Fatalf("folderSets = %+v", s.FolderSets)
	}
	// A save round-trips into the new keys only.
	if err := saveSettings(root, s); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "settings.json"))
	for _, old := range []string{`"storageBase"`, `"libraries"`} {
		if strings.Contains(string(b), old) {
			t.Fatalf("legacy key %s survived a save: %s", old, b)
		}
	}
}
