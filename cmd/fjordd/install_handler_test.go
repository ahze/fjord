package main

import (
	"reflect"
	"testing"
)

func TestApplyPathLists(t *testing.T) {
	values := map[string]string{"MOVIES_PATH": "/containers/radarr/movies", "TZ": "UTC"}
	paths := map[string][]string{
		"MOVIES_PATH":    {" /mnt/home/alice ", "", "/mnt/home"},
		"DOWNLOADS_PATH": {"/downloads"},
		"EMPTY_PATH":     {"", "  "},
	}
	exp := applyPathLists(values, paths)

	want := map[string]string{"MOVIES_PATH": "/mnt/home/alice", "DOWNLOADS_PATH": "/downloads", "TZ": "UTC"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %v, want %v", values, want)
	}
	if len(exp) != 1 || exp[0].Var != "MOVIES_PATH" || !reflect.DeepEqual(exp[0].Hosts, []string{"/mnt/home/alice", "/mnt/home"}) {
		t.Fatalf("expansions = %+v", exp)
	}
	if _, ok := values["EMPTY_PATH"]; ok {
		t.Fatal("an all-blank list must not set a value")
	}
}

func TestChooseAppData(t *testing.T) {
	locs := []string{"/ssd/apps", "/hdd/apps"}
	if got := chooseAppData("/hdd/apps", locs); got != "/hdd/apps" {
		t.Fatalf("configured choice: got %q", got)
	}
	if got := chooseAppData("/etc", locs); got != "/ssd/apps" {
		t.Fatalf("unknown path must fall back to the default: got %q", got)
	}
	if got := chooseAppData("", locs); got != "/ssd/apps" {
		t.Fatalf("empty must be the default: got %q", got)
	}
}

func TestExpandPathTemplates(t *testing.T) {
	values := map[string]string{
		"MOVIES_PATH": "{{ appdata }}/{{stack}}/movies",
		"DATA_PATH":   "/mnt/pool/{{stack}}",
		"TZ":          "{{stack}}", // not a path var: untouched
	}
	exps := []pathExpansion{{Var: "MOVIES_PATH", Hosts: []string{"{{base}}/{{stack}}/movies", "/mnt/media/movies"}}} // {{base}} = legacy alias
	dirs := expandPathTemplates(values, exps, map[string]bool{"MOVIES_PATH": true, "DATA_PATH": true}, "radarr-4k", "/containers")

	if values["MOVIES_PATH"] != "/containers/radarr-4k/movies" || values["DATA_PATH"] != "/mnt/pool/radarr-4k" || values["TZ"] != "{{stack}}" {
		t.Fatalf("values = %v", values)
	}
	if !reflect.DeepEqual(exps[0].Hosts, []string{"/containers/radarr-4k/movies", "/mnt/media/movies"}) {
		t.Fatalf("hosts = %v", exps[0].Hosts)
	}
	// Only the path under the storage base is this stack's to create, once.
	if len(dirs) != 1 || dirs[0].Path != "/containers/radarr-4k/movies" || dirs[0].Uid != 1000 {
		t.Fatalf("dirs = %+v", dirs)
	}
}

func TestUniqueSubfolders(t *testing.T) {
	got := uniqueSubfolders([][]string{
		subfolderCandidates("/mnt/home/alice"),
		subfolderCandidates("mars/mnt/sea/alice"),
		subfolderCandidates("/x/alice"),
		subfolderCandidates("/y/alice"),
		subfolderCandidates("/mnt/home"),
	})
	want := []string{"alice", "sea-alice", "x-alice", "y-alice", "home"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	got = uniqueSubfolders([][]string{{"a"}, {"a"}, {"a"}})
	if !reflect.DeepEqual(got, []string{"a", "a-2", "a-3"}) {
		t.Fatalf("numeric fallback: %v", got)
	}
}
