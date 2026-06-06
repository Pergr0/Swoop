//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd

package paths

// Downloads is a generic fallback for platforms without a native rule here
// (e.g. mobile, where the host app typically supplies the real path).
func Downloads() string { return homeJoin("Downloads") }
