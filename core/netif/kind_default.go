//go:build !windows

package netif

func kindFromPlatform(string) (string, bool) { return "", false }
