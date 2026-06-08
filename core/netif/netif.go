// Package netif enumerates local network interfaces so the user can pick which
// one Swoop uses. On machines with several adapters (Ethernet + Wi-Fi + VPN +
// VM/host-only networks) the auto-picked route is often not the one peers can
// reach, so an explicit choice avoids "devices see each other but can't
// transfer" problems.
package netif

import (
	"net"
	"sort"
	"strings"
)

// NetInterface is a user-selectable network interface. (Named NetInterface
// rather than Interface because Wails' binding generator reserves "Interface".)
type NetInterface struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName,omitempty"` // SSID or friendly hardware label
	SSID        string   `json:"ssid,omitempty"`
	Addresses   []string `json:"addresses"` // IPv4 addresses
	Kind        string   `json:"kind"`      // wifi|ethernet|tunnel|virtual|other
	Up          bool     `json:"up"`
	SpeedMbps   int      `json:"speedMbps"` // best-effort; 0 when unknown
	Recommended bool     `json:"recommended"`
}

// List returns the usable IPv4 interfaces (up, non-loopback, with an address).
func List() []NetInterface {
	all, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]NetInterface, 0, len(all))
	for _, ifi := range all {
		if ifi.Flags&net.FlagLoopback != 0 || ifi.Flags&net.FlagUp == 0 {
			continue
		}
		addrs := ipv4Addrs(ifi)
		if len(addrs) == 0 {
			continue
		}
		ni := NetInterface{
			Name:      ifi.Name,
			Addresses: addrs,
			Kind:      kindOf(ifi.Name),
			Up:        true,
			SpeedMbps: speedMbps(ifi.Name),
		}
		enrichInterface(&ni)
		out = append(out, ni)
	}
	sortInterfaces(out)
	markRecommended(out)
	return out
}

func kindPriority(kind string) int {
	switch kind {
	case "wifi":
		return 0
	case "ethernet":
		return 1
	case "tunnel":
		return 2
	case "virtual":
		return 3
	default:
		return 4
	}
}

func sortInterfaces(list []NetInterface) {
	sort.SliceStable(list, func(i, j int) bool {
		pi, pj := kindPriority(list[i].Kind), kindPriority(list[j].Kind)
		if pi != pj {
			return pi < pj
		}
		return list[i].Name < list[j].Name
	})
}

func markRecommended(list []NetInterface) {
	for i := range list {
		list[i].Recommended = false
	}
	for _, kind := range []string{"wifi", "ethernet", "tunnel"} {
		for i := range list {
			if list[i].Kind == kind {
				list[i].Recommended = true
				return
			}
		}
	}
	if len(list) > 0 {
		list[0].Recommended = true
	}
}

func ipv4Addrs(ifi net.Interface) []string {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip4 := ip.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
			out = append(out, ip4.String())
		}
	}
	return out
}

// kindOf picks an interface category for UI icons. Platform code may use OS
// adapter metadata; kindOfName is the portable name-based fallback.
func kindOf(name string) string {
	if k, ok := kindFromPlatform(name); ok {
		return k
	}
	return kindOfName(name)
}

// kindOfName guesses a category from the interface name (best-effort).
func kindOfName(name string) string {
	n := strings.ToLower(name)
	switch {
	case containsAny(n, "wi-fi", "wifi", "wlan", "wlp", "wlo", "802.11", "wireless", "беспровод"):
		return "wifi"
	case containsAny(n, "tun", "tap", "wg", "ppp", "utun", "wireguard", "vpn", "xray"):
		return "tunnel"
	case containsAny(n, "vbox", "virtualbox", "vmware", "vmnet", "docker", "veth", "br-", "virbr", "hyper-v", "vethernet", "loopback adapter"):
		return "virtual"
	case strings.HasPrefix(n, "eth") || containsAny(n, "ethernet", "eno", "ens", "enp", "lan", "realtek", "intel", "локальной сет"):
		return "ethernet"
	default:
		return "other"
	}
}

func hardwarePortKindFromLabel(port string) string {
	n := strings.ToLower(port)
	switch {
	case containsAny(n, "wi-fi", "wifi", "airport", "беспровод"):
		return "wifi"
	case containsAny(n, "vpn", "tunnel", "ppp", "ipsec"):
		return "tunnel"
	case containsAny(n, "bridge", "virtual", "vmware", "parallels"):
		return "virtual"
	case containsAny(n, "ethernet", "thunderbolt", "usb 10/", "usb 100/", "локальной сет"):
		return "ethernet"
	default:
		return ""
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// parseNetshWLANInterfaces parses "netsh wlan show interfaces" (en/ru labels).
func parseNetshWLANInterfaces(out string) map[string]string {
	result := make(map[string]string)
	var currentName string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		keyLower := strings.ToLower(key)
		switch {
		case keyLower == "name" || key == "Имя":
			currentName = val
		case keyLower == "ssid":
			if currentName != "" && val != "" && !strings.EqualFold(val, "n/a") {
				result[currentName] = val
			}
		}
	}
	return result
}
