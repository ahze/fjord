package compose

import (
	"os"
	"strings"
)

// ExpandEnv resolves compose-style ${VAR} references against env, including
// the ${VAR:-default} (unset or empty) and ${VAR-default} (unset only)
// fallback forms. Nested defaults (${A:-${B:-x}}) are not supported --
// podman-compose can't evaluate them either, so no working stack relies on
// them. Unset vars without a default expand to "".
func ExpandEnv(s string, env map[string]string) string {
	return os.Expand(s, func(ref string) string {
		if i := strings.Index(ref, ":-"); i >= 0 {
			if v := env[ref[:i]]; v != "" {
				return v
			}
			return ref[i+2:]
		}
		if i := strings.Index(ref, "-"); i >= 0 {
			if v, ok := env[ref[:i]]; ok {
				return v
			}
			return ref[i+1:]
		}
		return env[ref]
	})
}
