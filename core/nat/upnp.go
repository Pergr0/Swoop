// Package nat discovers a public endpoint via local UPnP/NAT-PMP (no remote servers).
package nat

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Mapping holds router-mapped external endpoints for P2P pairing.
type Mapping struct {
	ExternalIP        string
	ExternalControl   int
	ExternalData      int
	ExternalPunch     int
	release           func()
}

// Release removes port mappings from the router.
func (m *Mapping) Release() {
	if m != nil && m.release != nil {
		m.release()
		m.release = nil
	}
}

// TryMapPorts attempts UPnP IGD port forwarding for control (TCP), data (TCP), and punch (UDP).
// Returns ok=false when the router does not support UPnP or mapping fails (LAN-only invite still works).
func TryMapPorts(ctx context.Context, controlPort, dataPort, punchPort int, logf func(string, ...any)) (*Mapping, bool) {
	if controlPort <= 0 || dataPort <= 0 || punchPort <= 0 {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	m, err := mapUPnP(ctx, controlPort, dataPort, punchPort)
	if err != nil {
		if logf != nil {
			logf("UPnP map skipped: %v", err)
		}
		return nil, false
	}
	if logf != nil {
		logf("UPnP mapped external %s control=%d data=%d punch=%d",
			m.ExternalIP, m.ExternalControl, m.ExternalData, m.ExternalPunch)
	}
	return m, true
}

func isUsableIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil && !parsed.IsLoopback() && !parsed.IsUnspecified()
}

func mappingError(msg string) error {
	return fmt.Errorf("%s", msg)
}
