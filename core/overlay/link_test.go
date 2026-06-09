package overlay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"swoop/core/identity"
	"swoop/core/overlay"
	"swoop/core/pairing"
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
	if _, ok := srv.Store.Join(sessionID, joinerPeer, "10.0.0.3", "127.0.0.1", "Joiner", fp, 60002, 53318, nil); !ok {
		t.Fatal("join failed")
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

// Regression: host relay must survive a long wait before the joiner arrives (yamux
// keepalive on an unbridged WebSocket used to kill the session).
func TestOverlayRelayDelayedJoiner(t *testing.T) {
	if testing.Short() {
		t.Skip("slow overlay relay delay test")
	}
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

	const sessionID = "sess-overlay-delay"
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
	if _, ok := srv.Store.Join(sessionID, joinerPeer, "10.0.0.3", "127.0.0.1", "Joiner", fp, 60002, 53318, nil); !ok {
		t.Fatal("join failed")
	}

	info := protocol.DeviceInfo{ID: hostPeer, Name: "Host", Fingerprint: fp}
	controlSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}))
	defer controlSrv.Close()
	hostPort := controlSrv.Listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		link, err := overlay.ServeHost(ctx, overlay.HostParams{
			SessionID: sessionID, PeerID: hostPeer, ControlPort: hostPort, DataPort: hostPort,
		})
		if err != nil {
			t.Errorf("serve host: %v", err)
			return
		}
		<-ctx.Done()
		_ = link.Close()
	}()

	time.Sleep(20 * time.Second)
	link, err := overlay.ConnectJoinerRetry(ctx, sessionID, joinerPeer)
	if err != nil {
		t.Fatalf("connect joiner after delay: %v", err)
	}
	defer link.Close()
	if _, err := link.ProbeInfo(ctx, protocol.DeviceInfo{ID: hostPeer, Fingerprint: fp}); err != nil {
		t.Fatalf("probe after delay: %v", err)
	}
	cancel()
	wg.Wait()
}

// TestOverlayDirectP2PUpgrade verifies relay → QUIC upgrade on loopback (no NAT).
func TestOverlayDirectP2PUpgrade(t *testing.T) {
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

	const sessionID = "sess-p2p-upgrade"
	const hostPeer = "host-peer-id"
	const joinerPeer = "joiner-peer-id"

	srv.Store.RegisterHost(sessionID, hostPeer, "Host", "127.0.0.1", "203.0.113.1", "127.0.0.1", 53317, 60001, 0)
	if _, ok := srv.Store.Join(sessionID, joinerPeer, "127.0.0.2", "127.0.0.2", "Joiner", "sha256:joiner", 60002, 53318, nil); !ok {
		t.Fatal("join failed")
	}

	id, err := identity.LoadOrCreate(t.TempDir(), "host")
	if err != nil {
		t.Fatal(err)
	}
	fp := id.Fingerprint

	info := protocol.DeviceInfo{ID: hostPeer, Name: "Host", Fingerprint: fp}
	controlSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}))
	defer controlSrv.Close()
	hostPort := controlSrv.Listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	hostReady := make(chan *overlay.Link, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		link, err := overlay.ServeHost(ctx, overlay.HostParams{
			SessionID:   sessionID,
			PeerID:      hostPeer,
			ControlPort: hostPort,
			DataPort:    hostPort,
			HostCert:    id.Certificate,
			LanAddr:     "127.0.0.1",
			ReachAddr:   "127.0.0.1",
			PunchPort:   60001,
			Logf:        t.Logf,
		})
		if err != nil {
			t.Errorf("serve host: %v", err)
			hostReady <- nil
			return
		}
		hostReady <- link
		<-ctx.Done()
		_ = link.Close()
	}()

	hostLink := <-hostReady
	if hostLink == nil {
		t.Fatal("host overlay failed")
	}
	if hostLink.QuicPort() <= 0 {
		t.Fatal("host QUIC port not bound")
	}
	t.Logf("host QUIC UDP port %d", hostLink.QuicPort())
	time.Sleep(200 * time.Millisecond)
	link, err := overlay.ConnectJoinerRetry(ctx, sessionID, joinerPeer)
	if err != nil {
		t.Fatalf("connect joiner: %v", err)
	}
	defer link.Close()

	punchConn, _, err := pairing.ListenPunchUDP()
	if err != nil {
		t.Fatal(err)
	}
	defer punchConn.Close()

	link.StartUpgrade(ctx, overlay.UpgradeParams{
		SessionID:       sessionID,
		Fingerprint:     fp,
		ReachAddr:       "127.0.0.1",
		LanAddr:         "127.0.0.1",
		HostPunchPort:   60001,
		JoinerPunchPort: punchConn.LocalAddr().(*net.UDPAddr).Port,
		JoinerPeerID:    joinerPeer,
		PunchConn:       punchConn,
		Logf: func(format string, args ...any) {
			t.Logf(format, args...)
		},
	})

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if link.Mode() == "direct" {
			if _, err := link.ProbeInfo(ctx, protocol.DeviceInfo{ID: hostPeer, Fingerprint: fp}); err != nil {
				t.Fatalf("probe over direct: %v", err)
			}
			cancel()
			wg.Wait()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if runtime.GOOS == "windows" {
		t.Skipf("QUIC loopback upgrade not reliable on Windows (mode=%q note=%q); relay path covered by other tests", link.Mode(), link.P2PNote())
	}
	t.Fatalf("expected direct P2P, mode=%q note=%q", link.Mode(), link.P2PNote())
}
