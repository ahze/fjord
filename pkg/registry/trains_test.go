package registry

import "testing"

// A rolling "tip" pin carries its channel before the arch suffix; it must land
// in that channel's train even when only the per-arch tag has been pushed yet
// (sylve, 2026-09-13: it showed up under "latest").
func TestDiscoverTrainsArchSuffixedPinKeepsItsChannel(t *testing.T) {
	tags := []string{
		"latest", "nightly", "0.3.0", "0.3.1", "0.3.1-nightly",
		"0.3.1-tip.20260912.dcef065-nightly-amd64", // plain tag not published yet
	}
	tr := discoverTrains(tags)
	for _, v := range tr["latest"] {
		if v.Tag == "0.3.1-tip.20260912.dcef065-nightly-amd64" {
			t.Fatalf("tip pin filed under latest: %+v", tr["latest"])
		}
	}
	found := false
	for _, v := range tr["nightly"] {
		if v.Tag == "0.3.1-tip.20260912.dcef065-nightly-amd64" && v.Version == "0.3.1-tip.20260912.dcef065" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tip pin missing from nightly: %+v", tr["nightly"])
	}
	if len(tr["latest"]) != 2 || tr["latest"][0].Version != "0.3.1" {
		t.Fatalf("latest train = %+v, want 0.3.1, 0.3.0", tr["latest"])
	}
}

// With the multi-arch tag present, the host-arch duplicate is dropped first.
func TestFilterArchTagsDropsRedundantHostArch(t *testing.T) {
	got := filterArchTags([]string{"0.3.1-nightly", "0.3.1-nightly-amd64", "0.3.1-nightly-aarch64", "latest", "latest-amd64"})
	for _, g := range got {
		if g == "0.3.1-nightly-amd64" || g == "0.3.1-nightly-aarch64" || g == "latest-amd64" {
			t.Fatalf("kept redundant/other-arch tag %q in %v", g, got)
		}
	}
}

// One rolling channel per major (redis): "8.6-pkg-latest" is a channel whose
// pins are "8.6.x-8.6-pkg-latest"; a stack on 8.6-pkg-latest must not be told
// that 8.10 (the 8.8 channel's current package) is an upgrade.
func TestDiscoverTrainsPerMajorChannels(t *testing.T) {
	tags := []string{
		"latest", "pkg", "pkg-latest",
		"8.6", "8.6-pkg-latest", "8.6.4-8.6-pkg-latest", "8.6.5-8.6-pkg-latest", "8.6.6-8.6-pkg-latest",
		"8.8", "8.8-pkg-latest", "8.8.0-8.8-pkg-latest", "8.8.1-8.8-pkg-latest", "8.10.0-8.8-pkg-latest", "8.10.1-8.8-pkg-latest",
		"8.6.3-pkg-latest", "8.8.0-pkg-latest", // older scheme, no major in the suffix
	}
	tr := discoverTrains(tags)
	want := map[string][]string{
		"8.6-pkg-latest": {"8.6.6", "8.6.5", "8.6.4"},
		"8.8-pkg-latest": {"8.10.1", "8.10.0", "8.8.1", "8.8.0"},
		"pkg-latest":     {"8.8.0", "8.6.3"},
	}
	for ch, versions := range want {
		got := tr[ch]
		if len(got) != len(versions) {
			t.Fatalf("%s: got %+v, want %v", ch, got, versions)
		}
		for i, v := range versions {
			if got[i].Version != v {
				t.Fatalf("%s[%d] = %q, want %q (%+v)", ch, i, got[i].Version, v, got)
			}
		}
	}
	for ch, versions := range tr {
		for _, v := range versions {
			if v.Tag == "8.6-pkg-latest" || v.Tag == "8.8-pkg-latest" {
				t.Fatalf("channel tag %q listed as a version of %s", v.Tag, ch)
			}
		}
	}
}
