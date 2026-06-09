package server

import "testing"

func TestAllowOverlay(t *testing.T) {
	s := NewStore(nil)
	s.RegisterHost("sess1", "host-id", "Host", "10.0.0.1", "", "203.0.113.1", 53317, 60000, 0)

	if !s.AllowOverlay("sess1", "host", "host-id") {
		t.Fatal("host role should match registered host peer")
	}
	if s.AllowOverlay("sess1", "host", "other-id") {
		t.Fatal("wrong host peer should be rejected")
	}
	if !s.AllowOverlay("sess1", "joiner", "joiner-id") {
		t.Fatal("joiner role should be allowed for active session")
	}
	if s.AllowOverlay("sess1", "joiner", "host-id") {
		t.Fatal("host peer cannot join as joiner")
	}
	if s.AllowOverlay("missing", "joiner", "joiner-id") {
		t.Fatal("unknown session should be rejected")
	}
}
