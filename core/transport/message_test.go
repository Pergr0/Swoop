package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"swoop/core/identity"
	"swoop/core/protocol"
)

type captureHandler struct {
	last     protocol.ChatMessage
	lastRead protocol.ReadReceipt
}

func (c *captureHandler) ReceiveMessage(m protocol.ChatMessage, _, _ string) int {
	c.last = m
	return http.StatusOK
}

func (c *captureHandler) ReceiveRead(rr protocol.ReadReceipt, _, _ string) int {
	c.lastRead = rr
	return http.StatusOK
}

func startMsgServer(t *testing.T, h MessageHandler) (*Server, *identity.Identity) {
	t.Helper()
	id, err := identity.LoadOrCreate(t.TempDir(), "recv")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	self := func() protocol.DeviceInfo { return protocol.DeviceInfo{ID: "recv", Fingerprint: id.Fingerprint} }
	srv := NewServer(id, self, nil)
	srv.SetMessageHandler(h)
	if err := srv.Start(context.Background(), 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	return srv, id
}

func TestMessageRoundTrip(t *testing.T) {
	h := &captureHandler{}
	srv, id := startMsgServer(t, h)

	tricky := "hello\nlink https://x/y \"q\" '; DROP TABLE t;-- <script>alert(1)</script>"
	body, _ := json.Marshal(protocol.ChatMessage{Sender: protocol.DeviceInfo{ID: "snd", Name: "snd"}, Text: tricky})
	client := NewPinnedClient(id.Fingerprint, 5*time.Second)
	resp, err := client.Post(fmt.Sprintf("https://127.0.0.1:%d/api/v1/message", srv.Port()), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if h.last.Text != tricky {
		t.Fatalf("text not preserved: %q", h.last.Text)
	}
}

func TestReadReceiptRoundTrip(t *testing.T) {
	h := &captureHandler{}
	srv, id := startMsgServer(t, h)

	body, _ := json.Marshal(protocol.ReadReceipt{Reader: protocol.DeviceInfo{ID: "snd"}, UpToTs: 12345})
	client := NewPinnedClient(id.Fingerprint, 5*time.Second)
	resp, err := client.Post(fmt.Sprintf("https://127.0.0.1:%d/api/v1/read", srv.Port()), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if h.lastRead.UpToTs != 12345 || h.lastRead.Reader.ID != "snd" {
		t.Fatalf("read receipt not preserved: %+v", h.lastRead)
	}
}

func TestMessageFingerprintMismatchRejected(t *testing.T) {
	srv, _ := startMsgServer(t, &captureHandler{})
	body, _ := json.Marshal(protocol.ChatMessage{Text: "hi"})
	client := NewPinnedClient("sha256:deadbeef", 5*time.Second)
	_, err := client.Post(fmt.Sprintf("https://127.0.0.1:%d/api/v1/message", srv.Port()), "application/json", bytes.NewReader(body))
	if err == nil {
		t.Fatal("expected fingerprint mismatch to fail the request")
	}
	if !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("expected fingerprint error, got: %v", err)
	}
}
