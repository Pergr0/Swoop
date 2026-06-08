//go:build darwin

package netif

import (
	"os/exec"
	"strings"
	"sync"
)

var (
	darwinPortsOnce sync.Once
	darwinPorts     map[string]darwinPort
	darwinKinds     map[string]string
)

type darwinPort struct {
	hardwarePort string
	device       string
	kind         string
}

func kindFromPlatform(name string) (string, bool) {
	darwinPortsOnce.Do(loadDarwinPorts)
	k, ok := darwinKinds[name]
	return k, ok
}

func enrichInterface(n *NetInterface) {
	darwinPortsOnce.Do(loadDarwinPorts)
	if p, ok := darwinPorts[n.Name]; ok {
		if p.kind != "" {
			n.Kind = p.kind
		}
		if p.hardwarePort != "" && n.DisplayName == "" {
			n.DisplayName = p.hardwarePort
		}
	}
	if n.Kind == "wifi" {
		if ssid := darwinWifiSSID(n.Name); ssid != "" {
			n.SSID = ssid
			n.DisplayName = ssid
		}
	}
}

func loadDarwinPorts() {
	darwinPorts = make(map[string]darwinPort)
	darwinKinds = make(map[string]string)
	out, err := exec.Command("/usr/sbin/networksetup", "-listallhardwareports").Output()
	if err != nil {
		return
	}
	var current darwinPort
	flush := func() {
		if current.hardwarePort == "" || current.device == "" {
			return
		}
		kind := hardwarePortKindFromLabel(current.hardwarePort)
		current.kind = kind
		darwinPorts[current.device] = current
		if kind != "" {
			darwinKinds[current.device] = kind
		}
		current = darwinPort{}
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Hardware Port:"):
			flush()
			current.hardwarePort = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		case strings.HasPrefix(line, "Device:"):
			current.device = strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
		}
	}
	flush()
}

func darwinWifiSSID(device string) string {
	out, err := exec.Command("/usr/sbin/networksetup", "-getairportnetwork", device).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	lower := strings.ToLower(s)
	if strings.Contains(lower, "not associated") || strings.Contains(lower, "не подключ") {
		return ""
	}
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		return strings.TrimSpace(s[idx+1:])
	}
	return ""
}
