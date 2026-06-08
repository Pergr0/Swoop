package pairing

import (
	"testing"
	"time"

	"swoop/core/invite"
	"swoop/core/protocol"
)

func TestRegistryAddStatusesReap(t *testing.T) {
	r := New()
	dev := protocol.DeviceInfo{
		ID:          "peer-1",
		Name:        "Remote",
		Address:     "10.0.0.2",
		ControlPort: 53317,
		Fingerprint: "sha256:abc",
	}
	exp := time.Now().Add(time.Hour).Unix()
	inv := invite.Parsed{
		Device: dev, ExpiresAt: exp, SessionID: "sess1",
	}
	r.Add(dev, inv)

	peers := r.Peers()
	if len(peers) != 1 || peers[0].PairStatus != StatusConnecting {
		t.Fatalf("peers: %+v", peers)
	}

	r.Update("peer-1", dev)
	peers = r.Peers()
	if peers[0].PairStatus != StatusConnected {
		t.Fatalf("expected connected, got %q", peers[0].PairStatus)
	}

	r.SetStatus("peer-1", StatusError)
	if r.Peers()[0].PairStatus != StatusError {
		t.Fatalf("expected error")
	}

	inv.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	r.Add(dev, inv)
	r.reap()
	if len(r.Peers()) != 0 {
		t.Fatalf("expected expired peer removed")
	}
}
