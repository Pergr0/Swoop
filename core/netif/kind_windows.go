//go:build windows

package netif

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IANA ifType not exported by x/sys/windows.
const ifTypePropVirtual = 53

var (
	adapterKindsOnce sync.Once
	adapterKinds     map[string]string
)

func kindFromPlatform(name string) (string, bool) {
	adapterKindsOnce.Do(func() {
		adapterKinds = loadWindowsAdapterKinds()
	})
	k, ok := adapterKinds[name]
	return k, ok
}

func loadWindowsAdapterKinds() map[string]string {
	m := make(map[string]string)
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
				if aa.FriendlyName != nil {
					m[windows.UTF16PtrToString(aa.FriendlyName)] = k
				}
				if aa.Description != nil {
					desc := windows.UTF16PtrToString(aa.Description)
					if _, exists := m[desc]; !exists {
						m[desc] = k
					}
				}
			}
			return m
		}
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		return m
	}
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
