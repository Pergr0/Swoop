package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"swoop/core/identity"
	"swoop/core/protocol"
	"swoop/core/transport"
)

// TestLoopbackTransfer exercises the full control+data plane on loopback:
// sender -> receiver prepare-upload (TLS) -> accept -> data streams -> file
// written. It is the regression guard for the end-to-end transfer path.
func TestLoopbackTransfer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recvDir := t.TempDir()
	dlDir := filepath.Join(recvDir, "downloads")

	id, err := identity.LoadOrCreate(filepath.Join(recvDir, "id"), "receiver")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}

	var recvSelf protocol.DeviceInfo
	mgrR := NewManager(func() protocol.DeviceInfo { return recvSelf }, dlDir)
	stop := make(chan struct{})
	defer close(stop)
	if err := mgrR.StartDataPlane(stop); err != nil {
		t.Fatalf("data plane: %v", err)
	}
	mgrR.SetOnOffer(func(Offer) { go mgrR.RespondIncoming(true) })

	srv := transport.NewServer(id, func() protocol.DeviceInfo { return recvSelf }, mgrR)
	if err := srv.Start(ctx, 0); err != nil {
		t.Fatalf("server start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	recvSelf = protocol.DeviceInfo{
		ID:          id.DeviceID,
		Name:        "receiver",
		Host:        "receiver",
		Address:     "127.0.0.1",
		Platform:    protocol.PlatformLinux,
		ControlPort: srv.Port(),
		Fingerprint: id.Fingerprint,
		Version:     protocol.Version,
	}

	senderSelf := protocol.DeviceInfo{ID: "sender", Name: "sender", Address: "127.0.0.1"}
	mgrS := NewManager(func() protocol.DeviceInfo { return senderSelf }, t.TempDir())

	done := make(chan State, 8)
	mgrS.SetOnState(func(s State) {
		if s.Direction == DirSend {
			done <- s
		}
	})

	srcPath := filepath.Join(t.TempDir(), "hello.txt")
	want := []byte("the quick brown fox jumps over the lazy dog\n")
	if err := os.WriteFile(srcPath, want, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := mgrS.Send(recvSelf, []protocol.SendItem{{Path: srcPath, RelPath: "hello.txt"}}); err != nil {
		t.Fatalf("send: %v", err)
	}

	deadline := time.After(15 * time.Second)
	for {
		select {
		case s := <-done:
			switch s.State {
			case "completed":
				got, err := os.ReadFile(filepath.Join(dlDir, "hello.txt"))
				if err != nil {
					t.Fatalf("read received: %v", err)
				}
				if string(got) != string(want) {
					t.Fatalf("content mismatch: got %q", got)
				}
				return
			case "failed", "declined", "canceled":
				t.Fatalf("transfer ended: state=%s msg=%s", s.State, s.Message)
			}
		case <-deadline:
			t.Fatal("timeout waiting for transfer to complete")
		}
	}
}
