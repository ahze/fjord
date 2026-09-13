//go:build !freebsd

package engine

// Jailed is FreeBSD-only; no other platform has the jail concept.
func Jailed() bool { return false }
