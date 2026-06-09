package nat

import (
	"context"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
)

// TryMapUDPPort forwards one UDP port via UPnP (overlay QUIC). Returns a release
// callback when mapping succeeds.
func TryMapUDPPort(ctx context.Context, port int, logf func(string, ...any)) (func(), bool) {
	if port <= 0 {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	clients, _, err := internetgateway2.NewWANIPConnection1Clients()
	if err != nil || len(clients) == 0 {
		clients2, _, err2 := internetgateway2.NewWANIPConnection2Clients()
		if err2 != nil || len(clients2) == 0 {
			if logf != nil {
				logf("UPnP QUIC map skipped: no IGD")
			}
			return nil, false
		}
		return mapUDPPort2(ctx, clients2[0], port, logf)
	}
	return mapUDPPort1(ctx, clients[0], port, logf)
}

func mapUDPPort1(ctx context.Context, client *internetgateway2.WANIPConnection1, port int, logf func(string, ...any)) (func(), bool) {
	extIP, err := client.GetExternalIPAddress()
	if err != nil || !isUsableIPv4(extIP) {
		return nil, false
	}
	if err := ctx.Err(); err != nil {
		return nil, false
	}
	if err := client.AddPortMapping("", uint16(port), "UDP", uint16(port), extIP, true, "Swoop QUIC", uint32(mappingLifetime.Seconds())); err != nil {
		if logf != nil {
			logf("UPnP QUIC map failed: %v", err)
		}
		return nil, false
	}
	if logf != nil {
		logf("UPnP mapped QUIC UDP %d on %s", port, extIP)
	}
	return func() { _ = client.DeletePortMapping("", uint16(port), "UDP") }, true
}

func mapUDPPort2(ctx context.Context, client *internetgateway2.WANIPConnection2, port int, logf func(string, ...any)) (func(), bool) {
	extIP, err := client.GetExternalIPAddress()
	if err != nil || !isUsableIPv4(extIP) {
		return nil, false
	}
	if err := ctx.Err(); err != nil {
		return nil, false
	}
	if err := client.AddPortMapping("", uint16(port), "UDP", uint16(port), extIP, true, "Swoop QUIC", uint32(mappingLifetime.Seconds())); err != nil {
		if logf != nil {
			logf("UPnP QUIC map failed: %v", err)
		}
		return nil, false
	}
	if logf != nil {
		logf("UPnP mapped QUIC UDP %d on %s", port, extIP)
	}
	return func() { _ = client.DeletePortMapping("", uint16(port), "UDP") }, true
}

