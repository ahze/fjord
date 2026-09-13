package compose

import "testing"

func TestExpandEnv(t *testing.T) {
	env := map[string]string{"TAG": "v2", "EMPTY": ""}
	cases := []struct{ in, want string }{
		{"repo:${TAG}", "repo:v2"},
		{"repo:$TAG", "repo:v2"},
		{"repo:${TAG:-latest}", "repo:v2"},
		{"repo:${MISSING:-latest}", "repo:latest"},
		{"repo:${EMPTY:-latest}", "repo:latest"}, // :- falls back on empty too
		{"repo:${EMPTY-latest}", "repo:"},        // - only falls back when unset
		{"repo:${MISSING-latest}", "repo:latest"},
		{"repo:${MISSING}", "repo:"},
		{"repo:latest", "repo:latest"},
	}
	for _, c := range cases {
		if got := ExpandEnv(c.in, env); got != c.want {
			t.Errorf("ExpandEnv(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
