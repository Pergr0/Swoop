//go:build !linux

package netif

// speedMbps is unavailable without platform-specific APIs here; report unknown.
func speedMbps(string) int { return 0 }
