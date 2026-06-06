package discovery

import (
	"testing"

	"swoop/core/protocol"
)

func TestPeerOrderFirstSeen(t *testing.T) {
	d := New(protocol.DeviceInfo{ID: "self"})
	d.upsert(protocol.DeviceInfo{ID: "b", Name: "B"})
	d.upsert(protocol.DeviceInfo{ID: "a", Name: "A"})
	d.upsert(protocol.DeviceInfo{ID: "b", Name: "B-updated"})

	peers := d.Peers()
	if len(peers) != 2 {
		t.Fatalf("len=%d", len(peers))
	}
	if peers[0].ID != "b" || peers[1].ID != "a" {
		t.Fatalf("order=%v,%v", peers[0].ID, peers[1].ID)
	}
	if peers[0].Name != "B-updated" {
		t.Fatalf("name=%q", peers[0].Name)
	}
}

func TestPeerRejoinAppended(t *testing.T) {
	d := New(protocol.DeviceInfo{ID: "self"})
	d.upsert(protocol.DeviceInfo{ID: "a", Name: "A"})
	d.upsert(protocol.DeviceInfo{ID: "b", Name: "B"})

	d.mu.Lock()
	delete(d.peers, "b")
	d.order = removeID(d.order, "b")
	d.mu.Unlock()

	d.upsert(protocol.DeviceInfo{ID: "b", Name: "B-again"})

	peers := d.Peers()
	if len(peers) != 2 || peers[0].ID != "a" || peers[1].ID != "b" {
		t.Fatalf("order=%v,%v", peers[0].ID, peers[1].ID)
	}
}
