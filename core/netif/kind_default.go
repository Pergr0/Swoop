//go:build !windows && !darwin && !linux

package netif

func kindFromPlatform(string) (string, bool) { return "", false }

func enrichInterface(*NetInterface) {}
