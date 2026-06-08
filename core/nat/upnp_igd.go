package nat

import (
	"context"
	"fmt"
	"time"

	"github.com/huin/goupnp"
	"github.com/huin/goupnp/dcps/internetgateway2"
)

const mappingLifetime = 20 * time.Minute

func mapUPnP(ctx context.Context, controlPort, dataPort, punchPort int) (*Mapping, error) {
	clients, _, err := internetgateway2.NewWANIPConnection1Clients()
	if err != nil || len(clients) == 0 {
		clients2, _, err2 := internetgateway2.NewWANIPConnection2Clients()
		if err2 != nil || len(clients2) == 0 {
			return nil, mappingError("no UPnP IGD found")
		}
		return mapWANIPConnection2(ctx, clients2[0], controlPort, dataPort, punchPort)
	}
	return mapWANIPConnection1(ctx, clients[0], controlPort, dataPort, punchPort)
}

func mapWANIPConnection1(ctx context.Context, client *internetgateway2.WANIPConnection1, controlPort, dataPort, punchPort int) (*Mapping, error) {
	extIP, err := client.GetExternalIPAddress()
	if err != nil || !isUsableIPv4(extIP) {
		return nil, fmt.Errorf("external IP: %w", err)
	}
	if err := addMapping(ctx, func(proto string, extPort, intPort int, desc string) error {
		return client.AddPortMapping("", uint16(extPort), proto, uint16(intPort), extIP, true, desc, uint32(mappingLifetime.Seconds()))
	}, controlPort, dataPort, punchPort); err != nil {
		return nil, err
	}
	return &Mapping{
		ExternalIP:      extIP,
		ExternalControl: controlPort,
		ExternalData:    dataPort,
		ExternalPunch:   punchPort,
		release: func() {
			_ = client.DeletePortMapping("", uint16(controlPort), "TCP")
			_ = client.DeletePortMapping("", uint16(dataPort), "TCP")
			_ = client.DeletePortMapping("", uint16(punchPort), "UDP")
		},
	}, nil
}

func mapWANIPConnection2(ctx context.Context, client *internetgateway2.WANIPConnection2, controlPort, dataPort, punchPort int) (*Mapping, error) {
	extIP, err := client.GetExternalIPAddress()
	if err != nil || !isUsableIPv4(extIP) {
		return nil, fmt.Errorf("external IP: %w", err)
	}
	if err := addMapping(ctx, func(proto string, extPort, intPort int, desc string) error {
		return client.AddPortMapping("", uint16(extPort), proto, uint16(intPort), extIP, true, desc, uint32(mappingLifetime.Seconds()))
	}, controlPort, dataPort, punchPort); err != nil {
		return nil, err
	}
	return &Mapping{
		ExternalIP:      extIP,
		ExternalControl: controlPort,
		ExternalData:    dataPort,
		ExternalPunch:   punchPort,
		release: func() {
			_ = client.DeletePortMapping("", uint16(controlPort), "TCP")
			_ = client.DeletePortMapping("", uint16(dataPort), "TCP")
			_ = client.DeletePortMapping("", uint16(punchPort), "UDP")
		},
	}, nil
}

type addFn func(proto string, extPort, intPort int, desc string) error

func addMapping(ctx context.Context, add addFn, controlPort, dataPort, punchPort int) error {
	type step struct {
		proto, desc string
		port        int
	}
	steps := []step{
		{"TCP", "Swoop control", controlPort},
		{"TCP", "Swoop data", dataPort},
		{"UDP", "Swoop punch", punchPort},
	}
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := add(s.proto, s.port, s.port, s.desc); err != nil {
			return fmt.Errorf("map %s %d: %w", s.proto, s.port, err)
		}
	}
	return nil
}

// DiscoverGateways is exported for tests; lists UPnP IGD devices on the LAN.
func DiscoverGateways() ([]string, error) {
	devs, err := goupnp.DiscoverDevices(internetgateway2.URN_WANIPConnection_1)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(devs))
	for _, d := range devs {
		if d.Location != nil {
			out = append(out, d.Location.String())
		}
	}
	return out, nil
}
