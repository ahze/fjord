package compose

import (
	"strings"
	"testing"
)

const volBase = `services:
  app:
    image: example.org/app:latest
    ports:
      - "8080:8080"
`

func TestAttachVolume(t *testing.T) {
	out, err := AttachVolume(volBase, "media", "/data", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"media:/data:ro"`, "external: true"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if got := NamedVolumes(out); len(got) != 1 || got[0] != "media" {
		t.Fatalf("NamedVolumes = %v, want [media]", got)
	}
	// attaching the same volume again is rejected
	if _, err := AttachVolume(out, "media", "/other", false); err == nil {
		t.Fatal("expected duplicate-attach error")
	}
}

func TestAttachVolumeValidation(t *testing.T) {
	if _, err := AttachVolume(volBase, "", "/data", false); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := AttachVolume(volBase, "media", "data", false); err == nil {
		t.Fatal("expected error for relative path")
	}
}
