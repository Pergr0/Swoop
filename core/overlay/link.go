package overlay

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/quic-go/quic-go"

	"swoop/core/pairing"
	"swoop/core/protocol"
	"swoop/core/rendezvous"
)

const (
	connectTimeout = 12 * time.Second
	dataTokenLen   = 32
)

var relayServer = rendezvous.DefaultServer

// SetRelayServer overrides rendezvous address for overlay dials (tests).
func SetRelayServer(fn func() string) { relayServer = fn }

type upgradeRequest struct {
	JoinerPunchPort int    `json:"joinerPunchPort"`
	JoinerReflexive string `json:"joinerReflexive,omitempty"`
}

type upgradeOffer struct {
	QuicPort  int    `json:"quicPort"`
	ReachAddr string `json:"reachAddr,omitempty"`
	LanAddr   string `json:"lanAddr,omitempty"`
	PunchPort int    `json:"punchPort"`
}

// Link is a muxed invite-scoped tunnel; prefers direct QUIC over relay when upgraded.
type Link struct {
	mu        sync.RWMutex
	relayMux  *yamux.Session
	directMux *yamux.Session
	quicConn  quic.Connection
	quicUDP   *net.UDPConn

	hostCert      tls.Certificate
	quicPort      int
	reach         string
	lan           string
	punchPort     int
	punchConn     *net.UDPConn
	sessionID     string
	controlPort   int
	dataPort      int
	directServing bool

	closed       chan struct{}
	once         sync.Once
	logf         func(string, ...any)
	p2pNote      string
	onModeChange func()
	relayLost    chan struct{}
	relayLostOnce sync.Once
}

func (l *Link) activeMux() *yamux.Session {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.directMux != nil {
		return l.directMux
	}
	return l.relayMux
}

func (l *Link) relayMuxSession() *yamux.Session {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.relayMux
}

// Mode returns "direct", "relay", or "".
func (l *Link) Mode() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.directMux != nil {
		return "direct"
	}
	if l.relayMux != nil {
		return "relay"
	}
	return ""
}

func (l *Link) promoteDirect(mux *yamux.Session, qconn quic.Connection) {
	l.mu.Lock()
	oldRelay := l.relayMux
	l.directMux = mux
	l.quicConn = qconn
	l.relayMux = nil
	l.p2pNote = ""
	l.mu.Unlock()
	if oldRelay != nil {
		_ = oldRelay.Close()
	}
	l.notifyModeChange()
}

// ConnectJoiner opens relay WebSocket + yamux client.
func ConnectJoiner(ctx context.Context, sessionID, peerID string) (*Link, error) {
	ws, err := dialRelay(ctx, sessionID, "joiner", peerID)
	if err != nil {
		return nil, err
	}
	mux, err := yamux.Client(newWSConn(ws), relayYamuxConfig())
	if err != nil {
		_ = ws.Close()
		return nil, err
	}
	return &Link{relayMux: mux, closed: make(chan struct{}), relayLost: make(chan struct{})}, nil
}

// ConnectJoinerRetry waits for host relay and retries.
func ConnectJoinerRetry(ctx context.Context, sessionID, peerID string) (*Link, error) {
	var last error
	for attempt := 0; attempt < 20; attempt++ {
		link, err := ConnectJoiner(ctx, sessionID, peerID)
		if err == nil {
			return link, nil
		}
		last = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if last == nil {
		last = errors.New("overlay connect failed")
	}
	return nil, last
}

// ServeHost opens relay and prepares QUIC listener for direct upgrade.
func ServeHost(ctx context.Context, p HostParams) (*Link, error) {
	ws, err := dialRelay(ctx, p.SessionID, "host", p.PeerID)
	if err != nil {
		return nil, err
	}
	mux, err := yamux.Server(newWSConn(ws), relayYamuxConfig())
	if err != nil {
		_ = ws.Close()
		return nil, err
	}
	link := &Link{
		relayMux:    mux,
		closed:      make(chan struct{}),
		relayLost:   make(chan struct{}),
		hostCert:    p.HostCert,
		reach:       p.ReachAddr,
		lan:         p.LanAddr,
		punchPort:   p.PunchPort,
		punchConn:   p.PunchConn,
		sessionID:   p.SessionID,
		controlPort: p.ControlPort,
		dataPort:    p.DataPort,
		logf:        p.Logf,
	}
	if len(p.HostCert.Certificate) > 0 {
		quicUDP, port, err := bindQUICUDP()
		if err != nil {
			_ = mux.Close()
			_ = ws.Close()
			return nil, err
		}
		link.quicUDP = quicUDP
		link.quicPort = port
		if p.OnQuicBound != nil {
			p.OnQuicBound(port)
		}
		go link.runQUICListener(ctx)
	}
	link.startRelayInbound(ctx)
	return link, nil
}

func (l *Link) startRelayInbound(ctx context.Context) {
	if l.relayLost == nil {
		l.relayLost = make(chan struct{})
	}
	go func() {
		l.serveRelayInbound(ctx)
		l.signalRelayLost()
	}()
}

func (l *Link) signalRelayLost() {
	l.relayLostOnce.Do(func() {
		if l.relayLost != nil {
			close(l.relayLost)
		}
	})
}

// RelayAlive reports whether the overlay tunnel can carry traffic.
func (l *Link) RelayAlive() bool {
	if l == nil {
		return false
	}
	select {
	case <-l.relayLost:
		l.mu.RLock()
		alive := l.directMux != nil
		l.mu.RUnlock()
		return alive
	default:
	}
	l.mu.RLock()
	alive := l.relayMux != nil || l.directMux != nil
	l.mu.RUnlock()
	return alive
}

// WaitRelayDown blocks until the relay WebSocket session ends or ctx is canceled.
func (l *Link) WaitRelayDown(ctx context.Context) error {
	if l == nil || l.relayLost == nil {
		return errors.New("no relay")
	}
	select {
	case <-l.relayLost:
		return errors.New("relay down")
	case <-ctx.Done():
		return ctx.Err()
	case <-l.closed:
		return errors.New("closed")
	}
}

// StartJoinerRelay accepts inbound overlay streams from the invite host and proxies
// them to the local control/data plane (host → joiner over rendezvous relay).
func (l *Link) StartJoinerRelay(ctx context.Context, controlPort, dataPort int) {
	l.mu.Lock()
	l.controlPort = controlPort
	l.dataPort = dataPort
	l.mu.Unlock()
	l.startRelayInbound(ctx)
}

func (l *Link) runQUICListener(ctx context.Context) {
	ln, err := listenQUIC(l.quicUDP, l.hostCert)
	if err != nil {
		if l.logf != nil {
			l.logf("overlay quic listen: %v", err)
		}
		return
	}
	for {
		qconn, err := ln.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil || l.isClosed() {
				return
			}
			continue
		}
		mux, err := yamuxOverQUIC(ctx, qconn, true)
		if err != nil {
			_ = qconn.CloseWithError(0, "yamux")
			continue
		}
		l.promoteDirect(mux, qconn)
		if l.logf != nil {
			l.logf("overlay: direct QUIC accepted from %s", qconn.RemoteAddr())
		}
		if !l.directServing {
			l.directServing = true
			go l.serveMuxHost(ctx, mux)
		}
		return
	}
}

// StartUpgrade attempts relay → direct QUIC in the background (joiner).
func (l *Link) StartUpgrade(ctx context.Context, p UpgradeParams) {
	if l == nil || p.Fingerprint == "" {
		return
	}
	if p.PunchConn == nil {
		l.setP2PNote(P2PNoteNoPunch)
		if p.Logf != nil {
			p.Logf("overlay direct upgrade: no punch socket (keeping relay)")
		}
		return
	}
	l.setP2PNote(P2PNoteUpgrading)
	go func() {
		upCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var offer upgradeOffer
		err := l.upgradeToDirect(upCtx, p, &offer)
		if err != nil {
			note := classifyUpgradeFail(err, offer)
			l.setP2PNote(note)
			if p.Logf != nil {
				p.Logf("overlay direct upgrade failed (%s): %v (keeping relay)", note, err)
			}
			return
		}
		if p.Logf != nil {
			p.Logf("overlay: upgraded to direct QUIC P2P")
		}
	}()
}

func (l *Link) upgradeToDirect(ctx context.Context, p UpgradeParams, offerOut *upgradeOffer) error {
	relay := l.relayMuxSession()
	if relay == nil {
		return errUpgradeNoRelay
	}
	stream, err := relay.Open()
	if err != nil {
		return fmt.Errorf("%w: %v", errUpgradeNegotiate, err)
	}
	defer stream.Close()
	if _, err := stream.Write([]byte{StreamUpgrade}); err != nil {
		return fmt.Errorf("%w: %v", errUpgradeNegotiate, err)
	}
	req := upgradeRequest{JoinerPunchPort: p.JoinerPunchPort, JoinerReflexive: p.JoinerReflexive}
	if err := json.NewEncoder(stream).Encode(&req); err != nil {
		return fmt.Errorf("%w: %v", errUpgradeNegotiate, err)
	}
	var offer upgradeOffer
	if err := json.NewDecoder(stream).Decode(&offer); err != nil {
		return fmt.Errorf("%w: %v", errUpgradeNegotiate, err)
	}
	if offerOut != nil {
		*offerOut = offer
	}
	if offer.QuicPort <= 0 {
		return errUpgradeNoQuic
	}
	targets := quicPunchTargets(offer.ReachAddr, offer.LanAddr, offer.QuicPort, offer.PunchPort)
	punchCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	peerID := p.JoinerPeerID
	if peerID == "" {
		peerID = "joiner"
	}
	punchErr := pairing.PunchAddrs(punchCtx, p.PunchConn, p.SessionID, peerID, targets)

	dialHost := firstAddr(offer.ReachAddr, offer.LanAddr)
	qconn, err := dialQUIC(ctx, p.PunchConn, net.JoinHostPort(dialHost, strconv.Itoa(offer.QuicPort)), p.Fingerprint)
	if err != nil {
		if punchErr != nil {
			return fmt.Errorf("%w: %v", errUpgradePunch, punchErr)
		}
		return fmt.Errorf("%w: %v", errUpgradeQuicDial, err)
	}
	mux, err := yamuxOverQUIC(ctx, qconn, false)
	if err != nil {
		_ = qconn.CloseWithError(0, "yamux")
		return fmt.Errorf("%w: %v", errUpgradeQuicDial, err)
	}
	l.promoteDirect(mux, qconn)
	return nil
}

func (l *Link) serveRelayInbound(ctx context.Context) {
	mux := l.activeMux()
	if mux == nil {
		return
	}
	local := localHTTPClient()
	for {
		stream, err := mux.Accept()
		if err != nil {
			if l.logf != nil && !l.isClosed() {
				l.logf("overlay relay accept: %v", err)
			}
			return
		}
		go l.handleHostStream(ctx, stream, local)
	}
}

func (l *Link) serveMuxHost(ctx context.Context, mux *yamux.Session) {
	local := localHTTPClient()
	for {
		stream, err := mux.Accept()
		if err != nil {
			return
		}
		go l.handleHostStream(ctx, stream, local)
	}
}

func localHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 2 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func (l *Link) handleHostStream(ctx context.Context, stream net.Conn, local *http.Client) {
	defer stream.Close()
	var kind [1]byte
	if _, err := io.ReadFull(stream, kind[:]); err != nil {
		return
	}
	switch kind[0] {
	case StreamControl:
		l.proxyControl(ctx, stream, local)
	case StreamData:
		l.proxyData(stream)
	case StreamUpgrade:
		l.handleUpgradeRequest(ctx, stream)
	}
}

func (l *Link) handleUpgradeRequest(ctx context.Context, stream net.Conn) {
	var req upgradeRequest
	if err := json.NewDecoder(stream).Decode(&req); err != nil {
		return
	}
	if req.JoinerReflexive != "" && req.JoinerPunchPort > 0 && l.punchConn != nil {
		_ = pairing.SendPunchHello(l.punchConn, l.sessionID, req.JoinerReflexive, req.JoinerPunchPort)
	}
	offer := upgradeOffer{
		QuicPort:  l.quicPort,
		ReachAddr: l.reach,
		LanAddr:   l.lan,
		PunchPort: l.punchPort,
	}
	_ = json.NewEncoder(stream).Encode(&offer)
	_ = ctx
}

func (l *Link) proxyControl(ctx context.Context, stream net.Conn, local *http.Client) {
	req, err := http.ReadRequest(bufio.NewReader(stream))
	if err != nil {
		return
	}
	req.URL.Scheme = "https"
	req.URL.Host = net.JoinHostPort("127.0.0.1", strconv.Itoa(l.controlPort))
	req.RequestURI = ""
	req.Host = req.URL.Host
	resp, err := local.Do(req.WithContext(ctx))
	if err != nil {
		_ = (&http.Response{StatusCode: http.StatusBadGateway, Body: http.NoBody}).Write(stream)
		return
	}
	defer resp.Body.Close()
	_ = resp.Write(stream)
}

func (l *Link) proxyData(stream net.Conn) {
	token := make([]byte, dataTokenLen)
	if _, err := io.ReadFull(stream, token); err != nil {
		return
	}
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(l.dataPort)))
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := conn.Write(token); err != nil {
		return
	}
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(conn, stream); errc <- err }()
	go func() { _, err := io.Copy(stream, conn); errc <- err }()
	<-errc
}

func (l *Link) ProbeInfo(ctx context.Context, device protocol.DeviceInfo) (protocol.DeviceInfo, error) {
	if device.Fingerprint == "" {
		return protocol.DeviceInfo{}, errors.New("missing fingerprint")
	}
	resp, err := l.doControl(ctx, http.MethodGet, "/api/v1/info", nil, device.Fingerprint)
	if err != nil {
		return protocol.DeviceInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return protocol.DeviceInfo{}, fmt.Errorf("info HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return protocol.DeviceInfo{}, err
	}
	var info protocol.DeviceInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return protocol.DeviceInfo{}, err
	}
	if info.ID != device.ID {
		return protocol.DeviceInfo{}, fmt.Errorf("device id mismatch")
	}
	if info.Fingerprint != device.Fingerprint {
		return protocol.DeviceInfo{}, fmt.Errorf("fingerprint mismatch")
	}
	return info, nil
}

func (l *Link) DoControl(ctx context.Context, method, path string, body io.Reader, contentType, fingerprint string) (*http.Response, error) {
	return l.doControl(ctx, method, path, body, fingerprint, contentType)
}

func (l *Link) doControl(ctx context.Context, method, path string, body io.Reader, fingerprint string, contentType ...string) (*http.Response, error) {
	mux := l.activeMux()
	if mux == nil {
		return nil, errors.New("overlay not connected")
	}
	stream, err := mux.Open()
	if err != nil {
		return nil, err
	}
	if _, err = stream.Write([]byte{StreamControl}); err != nil {
		_ = stream.Close()
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://overlay"+path, body)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	if len(contentType) > 0 && contentType[0] != "" {
		req.Header.Set("Content-Type", contentType[0])
	}
	if err := req.Write(stream); err != nil {
		_ = stream.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	return resp, nil
}

func (l *Link) DialData(ctx context.Context, token string) (net.Conn, error) {
	mux := l.activeMux()
	if mux == nil {
		return nil, errors.New("overlay not connected")
	}
	if len(token) != dataTokenLen {
		return nil, fmt.Errorf("overlay data token length %d", len(token))
	}
	stream, err := mux.Open()
	if err != nil {
		return nil, err
	}
	if _, err := stream.Write([]byte{StreamData}); err != nil {
		_ = stream.Close()
		return nil, err
	}
	if _, err := stream.Write([]byte(token)); err != nil {
		_ = stream.Close()
		return nil, err
	}
	return stream, nil
}

func (l *Link) HTTPClient(fingerprint string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if req.URL.RawQuery != "" {
				path += "?" + req.URL.RawQuery
			}
			var body io.Reader
			if req.Body != nil {
				body = req.Body
			}
			return l.doControl(req.Context(), req.Method, path, body, fingerprint, req.Header.Get("Content-Type"))
		}),
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func (l *Link) isClosed() bool {
	select {
	case <-l.closed:
		return true
	default:
		return false
	}
}

func (l *Link) Close() error {
	var err error
	l.once.Do(func() {
		close(l.closed)
		l.mu.Lock()
		if l.relayMux != nil {
			err = l.relayMux.Close()
		}
		if l.directMux != nil {
			_ = l.directMux.Close()
		}
		if l.quicConn != nil {
			_ = l.quicConn.CloseWithError(0, "closed")
		}
		if l.quicUDP != nil {
			_ = l.quicUDP.Close()
		}
		l.mu.Unlock()
	})
	return err
}

// relayYamuxConfig is for invite-scoped WebSocket relay tunnels. Keepalive is off
// because the host often waits minutes for a joiner while the rendezvous server
// is not yet bridging bytes; yamux pings would fill the socket buffer and kill
// the session before the importer arrives.
func relayYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = false
	return cfg
}

func directYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = 15 * time.Second
	return cfg
}

func dialRelay(ctx context.Context, sessionID, role, peerID string) (*websocket.Conn, error) {
	url := "ws://" + relayServer() + "/api/v1/overlay/connect?sessionId=" + sessionID +
		"&role=" + role + "&peerId=" + peerID
	d := websocket.Dialer{HandshakeTimeout: connectTimeout}
	ws, resp, err := d.DialContext(ctx, url, nil)
	if err != nil {
		if resp != nil {
			if resp.Body != nil {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
				_ = resp.Body.Close()
				if len(body) > 0 {
					return nil, fmt.Errorf("overlay dial HTTP %d: %s", resp.StatusCode, string(body))
				}
			}
			return nil, fmt.Errorf("overlay dial HTTP %d: %w", resp.StatusCode, err)
		}
		return nil, err
	}
	return ws, nil
}

func quicPunchTargets(hostReach, hostLAN string, quicPort, hostPunchPort int) []*net.UDPAddr {
	var out []*net.UDPAddr
	add := func(host string, port int) {
		if host == "" || port <= 0 {
			return
		}
		if a, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, strconv.Itoa(port))); err == nil {
			out = append(out, a)
		}
	}
	add(hostReach, quicPort)
	add(hostLAN, quicPort)
	add(hostReach, hostPunchPort)
	add(hostLAN, hostPunchPort)
	return out
}

func firstAddr(reach, lan string) string {
	if reach != "" {
		return reach
	}
	return lan
}
