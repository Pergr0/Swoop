package server

import (
	"testing"
	"time"
)

func TestStoreHostJoinPoll(t *testing.T) {
	s := NewStore(nil)
	s.RegisterHost("sess1", "host-1", "Alice", "192.168.1.1", "", "203.0.113.1", 53317, 55000, 0)

	room, ok := s.Join("sess1", "join-1", "10.0.0.2", "198.51.100.2", 55001)
	if !ok || room.host.peerID != "host-1" {
		t.Fatal("join failed")
	}

	j, ok := s.PollJoiner("sess1", "host-1")
	if !ok || j.reflexiveIP != "198.51.100.2" || j.punchPort != 55001 {
		t.Fatalf("poll: %+v", j)
	}
	if _, ok := s.PollJoiner("sess1", "host-1"); ok {
		t.Fatal("poll should be one-shot")
	}
}

func TestStoreExpiry(t *testing.T) {
	s := NewStore(nil)
	s.mu.Lock()
	s.rooms["old"] = &room{
		sessionID: "old",
		host:      hostRecord{expiresAt: time.Now().Add(-time.Minute)},
	}
	s.reapLocked()
	s.mu.Unlock()
	if len(s.rooms) != 0 {
		t.Fatal("expected reap")
	}
}
