package overlay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"swoop/core/overlay"
	"swoop/core/protocol"
	"swoop/core/rendezvous"
	rvserver "swoop/core/rendezvous/server"
)

func TestOverlayRelayProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := rvserver.New(addr)
	go func() { _ = srv.ListenAndServe() }()
	time.Sleep(150 * time.Millisecond)

	overlay.SetRelayServer(func() string { return addr })

	const sessionID = "sess-overlay-test"
	const hostPeer = "host-peer-id"
	const joinerPeer = "joiner-peer-id"
	const fp = "sha256:deadbeef"

	reg, _ := json.Marshal(rendezvous.HostRegisterRequest{
		SessionID: sessionID, PeerID: hostPeer, DeviceName: "Host",
		LanAddr: "10.0.0.2", ControlPort: 0, PunchPort: 60001,
	})
	resp, err := http.Post("http://"+addr+"/api/v1/rendezvous/host", "application/json", bytes.NewReader(reg))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register HTTP %d", resp.StatusCode)
	}

	info := protocol.DeviceInfo{
		ID: hostPeer, Name: "Host", Fingerprint: fp,
	}
	controlSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}))
	defer controlSrv.Close()

	dataLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dataLn.Close()

	hostPort := controlSrv.Listener.Addr().(*net.TCPAddr).Port
	dataPort := dataLn.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		link, err := overlay.ServeHost(ctx, overlay.HostParams{
			SessionID:   sessionID,
			PeerID:      hostPeer,
			ControlPort: hostPort,
			DataPort:    dataPort,
			Logf:        nil,
		})
		if err != nil {
			t.Errorf("serve host: %v", err)
			return
		}
		<-ctx.Done()
		_ = link.Close()
	}()

	time.Sleep(300 * time.Millisecond)
	link, err := overlay.ConnectJoinerRetry(ctx, sessionID, joinerPeer)
	if err != nil {
		t.Fatalf("connect joiner: %v", err)
	}
	defer link.Close()

	got, err := link.ProbeInfo(ctx, protocol.DeviceInfo{ID: hostPeer, Fingerprint: fp})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got.ID != hostPeer {
		t.Fatalf("got id %q want %q", got.ID, hostPeer)
	}
	cancel()
	wg.Wait()
}
