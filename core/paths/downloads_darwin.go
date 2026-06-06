//go:build darwin

package paths

// Downloads returns ~/Downloads on macOS.
func Downloads() string { return homeJoin("Downloads") }
