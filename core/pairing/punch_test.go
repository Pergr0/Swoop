package pairing

import (
	"context"
	"testing"
	"time"

	"swoop/core/invite"
	"swoop/core/protocol"
)

func TestPunchRoundTrip(t *testing.T) {
	hostConn, hostPort, err := ListenPunchUDP()
	if err != nil {
		t.Fatal(err)
	}
	defer hostConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunPunchHost(ctx, hostConn, "sess-test", nil)

	parsed := invite.Parsed{
		SessionID: "sess-test",
		PunchPort: hostPort,
		Device: protocol.DeviceInfo{
			Address: "127.0.0.1",
		},
	}
	punchCtx, punchCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer punchCancel()
	if err := ClientPunch(punchCtx, parsed, "importer-1", nil); err != nil {
		t.Fatal(err)
	}
}

func TestPunchTargetsReach(t *testing.T) {
	addrs := punchTargets(invite.Parsed{
		ReachAddr: "203.0.113.1",
		ReachPort: 53317,
		PunchPort: 55000,
		Device:    protocol.DeviceInfo{Address: "192.168.1.2"},
	})
	if len(addrs) < 2 {
		t.Fatalf("expected reach + LAN targets, got %d", len(addrs))
	}
}
