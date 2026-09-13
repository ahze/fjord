// Mirrors pkg/compose.ExpandEnv: ${VAR}, ${VAR:-default} (unset or empty),
// ${VAR-default} (unset only). Nested defaults unsupported -- podman-compose
// can't evaluate them either, so no working stack relies on them.
export function expandVars(s: string, env: Record<string, string>): string {
  return s.replace(/\$\{([^}]+)\}/g, (_, ref: string) => {
    const iCol = ref.indexOf(':-');
    if (iCol >= 0) {
      return env[ref.slice(0, iCol)] || ref.slice(iCol + 2);
    }
    const iDash = ref.indexOf('-');
    if (iDash >= 0) {
      const name = ref.slice(0, iDash);
      return name in env ? env[name] : ref.slice(iDash + 1);
    }
    return env[ref] ?? '';
  });
}
