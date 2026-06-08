//go:build linux

package netif

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func kindFromPlatform(name string) (string, bool) {
	if _, err := os.Stat(filepath.Join("/sys/class/net", name, "wireless")); err == nil {
		return "wifi", true
	}
	return "", false
}

func enrichInterface(n *NetInterface) {
	if n.Kind != "wifi" {
		if k, ok := kindFromPlatform(n.Name); ok {
			n.Kind = k
		}
	}
	if n.Kind == "wifi" {
		if ssid := linuxWifiSSID(n.Name); ssid != "" {
			n.SSID = ssid
			n.DisplayName = ssid
		}
	}
}

func linuxWifiSSID(device string) string {
	out, err := exec.Command("iwgetid", "-r", device).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
