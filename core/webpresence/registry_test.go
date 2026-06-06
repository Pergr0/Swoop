package webpresence

import (
	"testing"

	"swoop/core/protocol"
)

func TestRegistryOrder(t *testing.T) {
	port := func() int { return 53317 }
	r := New(port)
	if _, st := r.Touch(protocol.PresenceRequest{ID: "web-a", Name: "Phone"}, "Mozilla Safari", "192.168.1.10:1234"); st != 200 {
		t.Fatalf("touch web-a status=%d", st)
	}
	if _, st := r.Touch(protocol.PresenceRequest{ID: "web-b", Name: "Tablet"}, "Mozilla Chrome", "192.168.1.11:1234"); st != 200 {
		t.Fatalf("touch web-b status=%d", st)
	}

	peers := r.Peers()
	if len(peers) != 2 || peers[0].ID != "web-a" || peers[1].ID != "web-b" {
		t.Fatalf("peers=%+v", peers)
	}
	if peers[0].Platform != protocol.PlatformWeb || peers[0].Browser != "Safari" {
		t.Fatalf("peer0=%+v", peers[0])
	}
	if peers[0].Address != "192.168.1.10" || peers[0].ControlPort != 53317 {
		t.Fatalf("peer0 addr=%+v", peers[0])
	}
}
