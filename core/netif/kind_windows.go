//go:build windows

package netif

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IANA ifType not exported by x/sys/windows.
const ifTypePropVirtual = 53

type winAdapterMeta struct {
	kind     string
	friendly string
}

var (
	windowsMetaOnce sync.Once
	windowsMeta     map[string]winAdapterMeta
	windowsSSIDs    map[string]string
)

func kindFromPlatform(name string) (string, bool) {
	windowsMetaOnce.Do(loadWindowsMetadata)
	if meta, ok := windowsMeta[name]; ok && meta.kind != "" {
		return meta.kind, true
	}
	return "", false
}

func enrichInterface(n *NetInterface) {
	windowsMetaOnce.Do(loadWindowsMetadata)
	if meta, ok := lookupWindowsMeta(n.Name); ok {
		if meta.kind != "" {
			n.Kind = meta.kind
		}
		if meta.friendly != "" && meta.friendly != n.Name && n.DisplayName == "" {
			n.DisplayName = meta.friendly
		}
	}
	if n.Kind == "wifi" {
		for _, key := range []string{n.Name, n.DisplayName} {
			if key == "" {
				continue
			}
			if ssid, ok := windowsSSIDs[key]; ok && ssid != "" {
				n.SSID = ssid
				n.DisplayName = ssid
				return
			}
		}
	}
}

func lookupWindowsMeta(name string) (winAdapterMeta, bool) {
	if meta, ok := windowsMeta[name]; ok {
		return meta, true
	}
	return winAdapterMeta{}, false
}

func loadWindowsMetadata() {
	windowsMeta = make(map[string]winAdapterMeta)
	windowsSSIDs = loadWindowsWifiSSIDs()
	loadWindowsAdapterMeta(windowsMeta)
}

func loadWindowsAdapterMeta(m map[string]winAdapterMeta) {
	var size uint32 = 15_000
	for {
		b := make([]byte, size)
		err := windows.GetAdaptersAddresses(
			syscall.AF_UNSPEC,
			0,
			0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])),
			&size,
		)
		if err == nil {
			for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
				k := kindFromIfType(aa.IfType)
				if k == "" {
					continue
				}
				friendly := ""
				if aa.FriendlyName != nil {
					friendly = windows.UTF16PtrToString(aa.FriendlyName)
				}
				meta := winAdapterMeta{kind: k, friendly: friendly}
				if friendly != "" {
					m[friendly] = meta
				}
				if aa.Description != nil {
					desc := windows.UTF16PtrToString(aa.Description)
					if _, exists := m[desc]; !exists {
						m[desc] = meta
					}
				}
				if aa.AdapterName != nil {
					aname := windows.BytePtrToString(aa.AdapterName)
					if aname != "" {
						if _, exists := m[aname]; !exists {
							m[aname] = meta
						}
					}
				}
			}
			return
		}
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		return
	}
}

func loadWindowsWifiSSIDs() map[string]string {
	out, err := exec.Command("netsh", "wlan", "show", "interfaces").Output()
	if err != nil {
		return map[string]string{}
	}
	return parseNetshWLANInterfaces(string(out))
}

func kindFromIfType(t uint32) string {
	switch t {
	case windows.IF_TYPE_IEEE80211:
		return "wifi"
	case windows.IF_TYPE_ETHERNET_CSMACD, windows.IF_TYPE_ISO88025_TOKENRING:
		return "ethernet"
	case windows.IF_TYPE_PPP, windows.IF_TYPE_TUNNEL:
		return "tunnel"
	case ifTypePropVirtual:
		return "virtual"
	default:
		return ""
	}
}
