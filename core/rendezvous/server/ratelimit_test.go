package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPLimiter(t *testing.T) {
	lim := newIPLimiter(3, limitWindow)
	for i := 0; i < 3; i++ {
		if !lim.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if lim.allow("1.2.3.4") {
		t.Fatal("fourth request should be rate limited")
	}
	if !lim.allow("5.6.7.8") {
		t.Fatal("different IP should not share bucket")
	}
}

func TestRateLimitHTTP(t *testing.T) {
	s := New(":0")
	lim := newIPLimiter(2, limitWindow)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rendezvous/host", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	w := httptest.NewRecorder()
	if !s.rateLimit(w, req, lim) {
		t.Fatal("first request expected pass")
	}
	w = httptest.NewRecorder()
	if !s.rateLimit(w, req, lim) {
		t.Fatal("second request expected pass")
	}
	w = httptest.NewRecorder()
	if s.rateLimit(w, req, lim) {
		t.Fatal("third request should be limited")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", w.Code)
	}
}
