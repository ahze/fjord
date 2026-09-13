package main

import (
	"strings"
	"testing"
)

const testDirector = `services:
  bazarr:
    name: bazarr
    volumes:
      - CONFIG: /config
      - MOVIES_PATH: /movies
      - TV_PATH: /tv
volumes:
  CONFIG:
    device: !ENV '${CONFIG}'
  MOVIES_PATH:
    device: !ENV '${MOVIES_PATH}'
  TV_PATH:
    device: !ENV '${TV_PATH}'
`

// A variable given several folders is rendered as sub-mounts in the compose;
// the director volume must expand to one per folder, not be pruned.
func TestMaterializeDirectorExpandsMultiFolder(t *testing.T) {
	container2host := map[string]string{
		"/config":       "/containers/bazarr/config",
		"/movies/alice": "/mnt/home/alice/movies",
		"/movies/other": "/mnt/other/movies",
		// /tv unresolved -> pruned
	}
	out, placeholders, allVol, err := materializeDirector(testDirector, "7", container2host)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"CONFIG":        "/containers/bazarr/config",
		"MOVIES_PATH_1": "/mnt/home/alice/movies",
		"MOVIES_PATH_2": "/mnt/other/movies",
	} {
		if placeholders[k] != want {
			t.Errorf("%s = %q, want %q", k, placeholders[k], want)
		}
	}
	if _, ok := placeholders["MOVIES_PATH"]; ok {
		t.Error("original MOVIES_PATH should be replaced by its sub-mounts")
	}
	if !allVol["MOVIES_PATH"] || !allVol["TV_PATH"] {
		t.Error("original placeholders must stay in allVol so the .env writer drops them")
	}
	for _, want := range []string{
		"name: 7_bazarr",
		"- MOVIES_PATH_1: /movies/alice",
		"- MOVIES_PATH_2: /movies/other",
		"device: !ENV '${MOVIES_PATH_1}'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("director.yml missing %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"TV_PATH", "- MOVIES_PATH: /movies"} {
		if strings.Contains(out, gone) {
			t.Errorf("director.yml still has %q:\n%s", gone, out)
		}
	}
}
