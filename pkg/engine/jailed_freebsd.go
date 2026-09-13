package engine

import "syscall"

// Jailed reports whether this process is running inside a FreeBSD jail.
// Engines that manage host jails (appjail) are impossible to drive from
// inside one, so a jailed fjordd (e.g. deployed as an OCI container) must
// not offer them.
func Jailed() bool {
	v, err := syscall.SysctlUint32("security.jail.jailed")
	return err == nil && v == 1
}
