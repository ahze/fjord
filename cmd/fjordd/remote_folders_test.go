package main

import "testing"

func TestParseRemote(t *testing.T) {
	cases := map[string]remoteFolder{
		"nfs://mars/mnt/tide":          {Kind: "nfs", Server: "mars", Path: "/mnt/tide"},
		"smb://alice@nas/media":        {Kind: "smb", Server: "nas", Path: "media", User: "alice"},
		"smb://nas.lan/Movies FR/":     {Kind: "smb", Server: "nas.lan", Path: "Movies FR"},
		" nfs://10.0.0.5/export/x/y/ ": {Kind: "nfs", Server: "10.0.0.5", Path: "/export/x/y"},
	}
	for in, want := range cases {
		got, ok := parseRemote(in)
		if !ok || got != want {
			t.Errorf("%q: got %+v ok=%v, want %+v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"/mnt/media", "{{appdata}}/{{stack}}/data", "nfs://", "smb://nas/", "http://x/y", ""} {
		if _, ok := parseRemote(bad); ok {
			t.Errorf("%q parsed as remote", bad)
		}
	}
	if n := remoteVolumeName(remoteFolder{Kind: "nfs", Server: "mars", Path: "/mnt/tide"}); n != "fjord-nfs-mars-mnt-tide" {
		t.Errorf("volume name = %q", n)
	}
}
