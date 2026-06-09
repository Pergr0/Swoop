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
	if s.AllowOverlay("sess1", "joiner", "joiner-id") {
		t.Fatal("joiner must call Join before overlay attach")
	}
	_, ok := s.Join("sess1", "joiner-id", "10.0.0.2", "198.51.100.2", "Joiner", "sha256:abc", 55001, 53317, nil)
	if !ok {
		t.Fatal("join failed")
	}
	if !s.AllowOverlay("sess1", "joiner", "joiner-id") {
		t.Fatal("registered joiner should be allowed")
	}
	if s.AllowOverlay("sess1", "joiner", "other-joiner") {
		t.Fatal("unregistered joiner peer should be rejected")
	}
	if s.AllowOverlay("sess1", "joiner", "host-id") {
		t.Fatal("host peer cannot join as joiner")
	}
	if s.AllowOverlay("missing", "joiner", "joiner-id") {
		t.Fatal("unknown session should be rejected")
	}
}
