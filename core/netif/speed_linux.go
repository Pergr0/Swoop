//go:build linux

package netif

import (
	"os"
	"strconv"
	"strings"
)

// speedMbps reads the link speed Linux exposes via sysfs. Returns 0 when it is
// unavailable (common for Wi-Fi and virtual adapters).
func speedMbps(name string) int {
	b, err := os.ReadFile("/sys/class/net/" + name + "/speed")
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || v < 0 {
		return 0
	}
	return v
}
