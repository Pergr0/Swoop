// Package core wires identity, discovery and transport into a single engine.
// It is the one entry point consumed by any UI: the Wails desktop app today
// and mobile (via gomobile or native clients speaking the same protocol) later.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"swoop/core/chat"
	"swoop/core/discovery"
	"swoop/core/i18n"
	"swoop/core/identity"
	"swoop/core/invite"
	"swoop/core/nat"
	"swoop/core/netif"
	"swoop/core/overlay"
	"swoop/core/pairing"
	"swoop/core/paths"
	"swoop/core/rendezvous"
	"swoop/core/protocol"
	"swoop/core/transfer"
	"swoop/core/transport"
	"swoop/core/webpresence"
)

// Config configures the engine.
type Config struct {
	// DataDir is where identity material is stored.
	DataDir string
	// DeviceName is the human-friendly name shown to peers. Defaults to hostname.
	DeviceName string
	// DownloadsDir is where received files are written.
	DownloadsDir string
}

// Engine is the orchestrator that owns the device's networking lifecycle.
type Engine struct {
	id           *identity.Identity
	disco        *discovery.Discoverer
	paired       *pairing.Registry
	webPresence  *webpresence.Registry
	server       *transport.Server
	mgr          *transfer.Manager
	downloadsDir string
	logger       *log.Logger
	logPath      string
	logFile      *os.File

	boundIP string         // advertised IPv4 (chosen interface, or auto)
	iface   *net.Interface // chosen interface; nil = auto (all interfaces)
	started bool
	runCancel context.CancelFunc

	chat    *chat.Store
	chatLim *rateLimiter

	readMu     sync.Mutex
	readByPeer map[string]int64 // peerID -> max ts of our out-msgs they've read

	webOutMu      sync.Mutex
	webOutbox     map[string][]protocol.ChatMessage // pending desktop→browser
	webReadMu     sync.Mutex
	webReadNotify map[string]int64 // read receipts queued for browser poll

	onPeers          func([]protocol.DeviceInfo)
	onChat           func(chat.Message)
	onRead           func(peerID string, upToTs int64)
	onTransferState  func(transfer.State)
	onTransferOffer  func(transfer.Offer)

	inviteHostMu     sync.Mutex
	inviteHostCancel context.CancelFunc
	inviteNATRelease func()
	invitePunchConn  *net.UDPConn
	inviteOverlay    *overlay.Link
	inviteJoinerID   string
	inviteSessionID  string
	invitePunchPort  int
	inviteReach       *invite.Reach
	inviteQuicRelease func()

	overlayMu sync.Mutex
	overlays  map[string]*overlay.Link
	punchMu   sync.Mutex
	punchConn map[string]*net.UDPConn // joiner punch sockets for overlay upgrade

	closeOnce sync.Once
}

// New constructs an engine without starting any network activity.
func New(cfg Config) (*Engine, error) {
	name := cfg.DeviceName
	if name == "" {
		if host, err := os.Hostname(); err == nil {
			name = host
		} else {
			name = "Swoop device"
		}
	}
	id, err := identity.LoadOrCreate(cfg.DataDir, name)
	if err != nil {
		return nil, err
	}
	dl := cfg.DownloadsDir
	if dl == "" {
		dl = paths.Downloads()
	}
	lg, logPath, logFile := newLogger(cfg.DataDir)
	e := &Engine{id: id, downloadsDir: dl, logger: lg, logPath: logPath, logFile: logFile}
	e.mgr = transfer.NewManager(e.Self, dl)
	e.mgr.SetLogf(e.logf)
	e.mgr.SetOverlayFor(func(peerID string) transfer.OverlayTunnel {
		return e.overlayForPeer(peerID)
	})

	e.chatLim = newRateLimiter(30, 10*time.Second)
	e.readByPeer = make(map[string]int64)
	e.webOutbox = make(map[string][]protocol.ChatMessage)
	e.webReadNotify = make(map[string]int64)
	chatPath := filepath.Join(binaryDir(), fmt.Sprintf("swoop-chat-%d.tmp", os.Getpid()))
	if st, cerr := chat.NewStore(chatPath); cerr == nil {
		e.chat = st
		e.logf("chat log at %s", chatPath)
	} else {
		e.logf("chat store init failed: %v", cerr)
	}

	e.logf("engine ready: id=%s name=%q downloads=%q", id.DeviceID, id.Name, dl)
	return e, nil
}

// Close stops networking and releases engine resources. It removes the
// temporary chat log so no message history survives the app. Safe to call
// more than once (e.g. OnBeforeClose and OnShutdown).
func (e *Engine) Close() {
	e.closeOnce.Do(e.shutdown)
}

func (e *Engine) shutdown() {
	e.logf("engine shutting down")
	e.sendPairedGoodbyes()
	e.stopInviteHost()
	e.mgr.Shutdown()
	if e.disco != nil {
		e.disco.Goodbye()
	}
	if e.runCancel != nil {
		e.runCancel()
		e.runCancel = nil
	}
	e.started = false
	if e.chat != nil {
		if err := e.chat.Close(); err != nil {
			e.logf("chat store close: %v", err)
		}
	}
	if e.logFile != nil {
		if err := e.logFile.Close(); err != nil {
			e.logf("log file close: %v", err)
		}
		e.logFile = nil
	}
	e.logf("engine shutdown complete")
}

// LogPath returns the primary log file path (empty if logging fell back to stderr).
func (e *Engine) LogPath() string { return e.logPath }

// binaryDir returns the directory of the running executable (falling back to
// the working directory), used for files that should sit next to the binary.
func binaryDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// newLogger writes swoop.log under dataDir (always writable on packaged apps).
// When the install directory is writable it also mirrors to swoop.log next to
// the binary for local dev builds.
func newLogger(dataDir string) (*log.Logger, string, *os.File) {
	_ = os.MkdirAll(dataDir, 0o755)
	primary := filepath.Join(dataDir, "swoop.log")

	var writers []io.Writer
	var primaryFile *os.File
	if f, err := os.OpenFile(primary, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		primaryFile = f
		writers = append(writers, f)
	}

	binPath := filepath.Join(binaryDir(), "swoop.log")
	if binPath != primary {
		if f, err := os.OpenFile(binPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			writers = append(writers, f)
		}
	}

	logPath := primary
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
		logPath = ""
	}

	w := io.MultiWriter(writers...)
	lg := log.New(w, "", log.LstdFlags|log.Lmicroseconds)
	if logPath != "" {
		lg.Printf("logging to %s (binary dir: %s)", logPath, binaryDir())
	} else {
		lg.Printf("logging to stderr only (could not open %s)", primary)
	}
	return lg, logPath, primaryFile
}

func (e *Engine) logf(format string, args ...any) {
	if e.logger != nil {
		e.logger.Printf(format, args...)
	}
}

// DownloadsDir returns the directory where received files are saved.
func (e *Engine) DownloadsDir() string { return e.downloadsDir }

// Interfaces lists the selectable network interfaces.
func (e *Engine) Interfaces() []netif.NetInterface { return netif.List() }

// OnPeersChanged registers a callback for peer-set updates.
func (e *Engine) OnPeersChanged(fn func([]protocol.DeviceInfo)) { e.onPeers = fn }

// OnTransferOffer registers a callback for incoming upload offers.
func (e *Engine) OnTransferOffer(fn func(transfer.Offer)) {
	e.onTransferOffer = fn
	e.mgr.SetOnOffer(e.emitTransferOffer)
}

// OnTransferProgress registers a callback for transfer progress updates.
func (e *Engine) OnTransferProgress(fn func(transfer.Progress)) { e.mgr.SetOnProgress(fn) }

// OnTransferState registers a callback for transfer lifecycle updates.
func (e *Engine) OnTransferState(fn func(transfer.State)) {
	e.onTransferState = fn
	e.mgr.SetOnState(e.emitTransferState)
}

func (e *Engine) emitTransferOffer(o transfer.Offer) {
	if o.Sender.ID != "" {
		e.touchInternetActivity(o.Sender.ID)
	}
	if e.onTransferOffer != nil {
		e.onTransferOffer(o)
	}
}

func (e *Engine) emitTransferState(s transfer.State) {
	switch s.State {
	case "completed", "declined", "failed", "canceled":
		if id := e.peerIDForName(s.Peer); id != "" {
			e.touchInternetActivity(id)
		}
	}
	if e.onTransferState != nil {
		e.onTransferState(s)
	}
}

// SendTo starts an outgoing transfer to the peer with the given id.
func (e *Engine) SendTo(deviceID string, items []protocol.SendItem) error {
	if deviceID == e.id.DeviceID {
		return i18n.ErrSendToSelf()
	}
	peer, ok := e.findPeer(deviceID)
	if !ok {
		return i18n.ErrDeviceNotFound(deviceID)
	}
	if peer.Platform != protocol.PlatformWeb && (peer.Address == "" || peer.ControlPort == 0) {
		if e.overlayForPeer(deviceID) == nil {
			return i18n.ErrPeerNoAddress(peer.Name)
		}
	}
	e.touchInternetActivity(deviceID)
	return e.mgr.Send(peer, items)
}

// CancelOutgoing cancels the current outgoing transfer.
func (e *Engine) CancelOutgoing() { e.mgr.CancelOutgoing() }

// CancelIncoming aborts the active incoming transfer.
func (e *Engine) CancelIncoming() { e.mgr.CancelIncoming() }

// RespondIncoming accepts or declines the pending incoming offer.
func (e *Engine) RespondIncoming(accept bool) { e.mgr.RespondIncoming(accept) }

// OnChatMessage registers a callback for stored chat messages (both directions).
func (e *Engine) OnChatMessage(fn func(chat.Message)) { e.onChat = fn }

func (e *Engine) emitChat(m chat.Message) {
	if e.onChat == nil {
		return
	}
	defer func() { _ = recover() }()
	e.onChat(m)
}

// SendMessage sends a text message to the given peer over the control plane and
// records it locally on success.
func (e *Engine) SendMessage(deviceID, text string) error {
	if strings.TrimSpace(text) == "" {
		return i18n.ErrEmptyMessage()
	}
	if len(text) > protocol.MaxMessageBytes {
		return i18n.ErrMessageTooLong(protocol.MaxMessageBytes)
	}
	if !utf8.ValidString(text) {
		return i18n.ErrMessageNotUTF8()
	}
	peer, ok := e.findPeer(deviceID)
	if !ok {
		return i18n.ErrDeviceNotFoundShort()
	}
	ts := time.Now().UnixMilli()

	if peer.Platform == protocol.PlatformWeb {
		msg := protocol.ChatMessage{Sender: e.Self(), Text: text, Ts: ts}
		e.webOutMu.Lock()
		e.webOutbox[peer.ID] = append(e.webOutbox[peer.ID], msg)
		e.webOutMu.Unlock()
		rec := chat.Message{Ts: ts, PeerID: peer.ID, PeerName: peer.Name, Dir: "out", Text: text}
		if e.chat != nil {
			_ = e.chat.Append(rec)
		}
		e.emitChat(rec)
		return nil
	}

	if peer.Fingerprint == "" {
		return i18n.ErrPeerNoFingerprint(peer.Name)
	}
	if peer.Address == "" || peer.ControlPort == 0 {
		if e.overlayForPeer(deviceID) == nil {
			return i18n.ErrPeerNoAddress(peer.Name)
		}
	}

	body, _ := json.Marshal(protocol.ChatMessage{Sender: e.Self(), Text: text, Ts: ts})
	var resp *http.Response
	var err error
	if link := e.overlayForPeer(deviceID); link != nil {
		client := link.HTTPClient(peer.Fingerprint, 10*time.Second)
		url := "https://overlay/api/v1/message"
		resp, err = client.Post(url, "application/json", bytes.NewReader(body))
	} else {
		client := transport.NewPinnedClient(peer.Fingerprint, 10*time.Second)
		url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/message"
		resp, err = client.Post(url, "application/json", bytes.NewReader(body))
	}
	if err != nil {
		e.logf("chat send to %s failed: %v", peer.Name, err)
		return i18n.ErrChatSend(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		e.logf("chat send to %s rejected: HTTP %d", peer.Name, resp.StatusCode)
		return i18n.ErrChatRejected(resp.StatusCode)
	}

	rec := chat.Message{Ts: ts, PeerID: peer.ID, PeerName: peer.Name, Dir: "out", Text: text}
	if e.chat != nil {
		_ = e.chat.Append(rec)
	}
	e.touchInternetActivity(peer.ID)
	e.emitChat(rec)
	return nil
}

// ReceiveMessage implements transport.MessageHandler: it validates, rate-limits,
// persists and surfaces an incoming chat message. Text is treated strictly as
// inert data — never executed or interpreted.
func (e *Engine) ReceiveMessage(msg protocol.ChatMessage, remoteAddr, webToken string) int {
	text := msg.Text
	if text == "" || len(text) > protocol.MaxMessageBytes || !utf8.ValidString(text) {
		return http.StatusBadRequest
	}
	peerID := msg.Sender.ID
	if peerID == "" {
		peerID = remoteAddr
	}
	if msg.Sender.Platform == protocol.PlatformWeb {
		if e.webPresence == nil || !e.webPresence.Verify(peerID, remoteAddr, webToken) {
			return http.StatusForbidden
		}
	}
	if e.chatLim != nil && !e.chatLim.allow(peerID) {
		e.logf("chat message from %s (%s) rate-limited", msg.Sender.Name, remoteAddr)
		return http.StatusTooManyRequests
	}
	ts := msg.Ts
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	rec := chat.Message{Ts: ts, PeerID: peerID, PeerName: msg.Sender.Name, Dir: "in", Text: text}
	if e.chat != nil {
		if err := e.chat.Append(rec); err != nil {
			e.logf("chat append failed: %v", err)
			return http.StatusInternalServerError
		}
	}
	e.logf("chat message from %s (%s): %d bytes", msg.Sender.Name, remoteAddr, len(text))
	e.touchInternetActivity(peerID)
	e.emitChat(rec)
	return http.StatusOK
}

// ChatHistory returns up to limit recent messages exchanged with deviceID.
// Outgoing messages are flagged Read when the peer has acknowledged them.
func (e *Engine) ChatHistory(deviceID string, limit int) []chat.Message {
	if e.chat == nil {
		return nil
	}
	msgs, err := e.chat.History(deviceID, limit)
	if err != nil {
		e.logf("chat history error: %v", err)
		return nil
	}
	e.readMu.Lock()
	watermark := e.readByPeer[deviceID]
	e.readMu.Unlock()
	for i := range msgs {
		if msgs[i].Dir == "out" && msgs[i].Ts <= watermark {
			msgs[i].Read = true
		}
	}
	return msgs
}

// OnChatRead registers a callback fired when a peer acknowledges reading our
// messages up to a timestamp.
func (e *Engine) OnChatRead(fn func(peerID string, upToTs int64)) { e.onRead = fn }

func (e *Engine) emitRead(peerID string, upToTs int64) {
	if e.onRead == nil {
		return
	}
	defer func() { _ = recover() }()
	e.onRead(peerID, upToTs)
}

// MarkRead tells the peer that we have read their messages up to the newest one
// we hold. Best-effort: if the peer is unreachable the receipt is simply
// dropped (the sender still has it marked delivered).
func (e *Engine) MarkRead(deviceID string) {
	if e.chat == nil {
		return
	}
	msgs, err := e.chat.History(deviceID, 0)
	if err != nil {
		return
	}
	var upTo int64
	for _, m := range msgs {
		if m.Dir == "in" && m.Ts > upTo {
			upTo = m.Ts
		}
	}
	if upTo == 0 {
		return // nothing incoming to acknowledge
	}
	peer, ok := e.findPeer(deviceID)
	if !ok {
		return
	}
	if peer.Platform == protocol.PlatformWeb {
		e.webReadMu.Lock()
		if upTo > e.webReadNotify[deviceID] {
			e.webReadNotify[deviceID] = upTo
		}
		e.webReadMu.Unlock()
		return
	}
	if peer.Fingerprint == "" {
		return
	}
	if peer.Address == "" || peer.ControlPort == 0 {
		if e.overlayForPeer(deviceID) == nil {
			return
		}
	}
	body, _ := json.Marshal(protocol.ReadReceipt{Reader: e.Self(), UpToTs: upTo})
	var resp *http.Response
	var postErr error
	if link := e.overlayForPeer(deviceID); link != nil {
		client := link.HTTPClient(peer.Fingerprint, 5*time.Second)
		resp, postErr = client.Post("https://overlay/api/v1/read", "application/json", bytes.NewReader(body))
	} else {
		client := transport.NewPinnedClient(peer.Fingerprint, 5*time.Second)
		url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/read"
		resp, postErr = client.Post(url, "application/json", bytes.NewReader(body))
	}
	if postErr != nil {
		e.logf("read receipt to %s failed: %v", peer.Name, postErr)
		return
	}
	_ = resp.Body.Close()
}

// ReceiveGoodbye removes an invite-paired peer that is shutting down.
func (e *Engine) ReceiveGoodbye(notice protocol.GoodbyeNotice, remoteAddr, webToken string) int {
	dev := notice.Device
	if dev.ID == "" || dev.ID == e.id.DeviceID {
		return http.StatusBadRequest
	}
	if dev.Platform == protocol.PlatformWeb {
		if e.webPresence == nil || !e.webPresence.Verify(dev.ID, remoteAddr, webToken) {
			return http.StatusForbidden
		}
	}
	e.logf("goodbye from %s (%s)", dev.Name, remoteAddr)
	e.dropPairedPeer(dev.ID)
	return http.StatusOK
}

func (e *Engine) sendPairedGoodbyes() {
	if e.paired == nil {
		return
	}
	for _, peer := range e.paired.Peers() {
		e.sendGoodbyeToPeer(peer)
	}
}

func (e *Engine) sendGoodbyeToPeer(peer protocol.DeviceInfo) {
	if peer.Platform == protocol.PlatformWeb || peer.Fingerprint == "" {
		return
	}
	body, _ := json.Marshal(protocol.GoodbyeNotice{Device: e.Self()})
	var resp *http.Response
	var err error
	if link := e.overlayForPeer(peer.ID); link != nil {
		client := link.HTTPClient(peer.Fingerprint, 3*time.Second)
		resp, err = client.Post("https://overlay/api/v1/goodbye", "application/json", bytes.NewReader(body))
	} else if peer.Address != "" && peer.ControlPort > 0 {
		client := transport.NewPinnedClient(peer.Fingerprint, 3*time.Second)
		url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/goodbye"
		resp, err = client.Post(url, "application/json", bytes.NewReader(body))
	}
	if err != nil {
		e.logf("goodbye to %s failed: %v", peer.Name, err)
		return
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
}

func (e *Engine) touchInternetActivity(peerID string) {
	if peerID == "" || e.paired == nil {
		return
	}
	if !e.paired.TouchActivity(peerID) {
		return
	}
	e.refreshRendezvousSession(peerID)
}

func (e *Engine) refreshRendezvousSession(peerID string) {
	if !rendezvous.Enabled() || e.paired == nil {
		return
	}
	inv, ok := e.paired.InviteMeta(peerID)
	if !ok || inv.SessionID == "" {
		return
	}
	sessionID := inv.SessionID
	e.inviteHostMu.Lock()
	isHost := e.inviteJoinerID == peerID && e.inviteSessionID == sessionID
	punchPort := e.invitePunchPort
	reach := e.inviteReach
	e.inviteHostMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), clientTimeout())
		defer cancel()
		client := rendezvous.NewClient()
		if isHost && punchPort > 0 {
			self := e.Self()
			req := rendezvous.HostRegisterRequest{
				SessionID:   sessionID,
				PeerID:      self.ID,
				DeviceName:  self.Name,
				LanAddr:     self.Address,
				ControlPort: e.server.Port(),
				PunchPort:   punchPort,
			}
			if reach != nil {
				req.ReachAddr = reach.Addr
				req.ReachPort = reach.ControlPort
			}
			if err := client.RegisterHost(ctx, req); err != nil {
				e.logf("rendezvous refresh (host): %v", err)
			}
			return
		}
		if err := client.TouchSession(ctx, sessionID); err != nil {
			e.logf("rendezvous refresh: %v", err)
		}
	}()
}

func (e *Engine) onPairedPeerIdle(peer protocol.DeviceInfo) {
	e.logf("paired peer %q idle timeout — goodbye", peer.Name)
	e.sendGoodbyeToPeer(peer)
	e.removeOverlay(peer.ID)
	e.inviteHostMu.Lock()
	if e.inviteJoinerID == peer.ID {
		e.inviteJoinerID = ""
		e.inviteHostMu.Unlock()
		e.stopInviteHost()
	} else {
		e.inviteHostMu.Unlock()
	}
	e.emitPeers()
}

func (e *Engine) peerIDForName(name string) string {
	if name == "" {
		return ""
	}
	for _, p := range e.Peers() {
		if p.Name == name && p.Paired {
			return p.ID
		}
	}
	for _, p := range e.Peers() {
		if p.Name == name {
			return p.ID
		}
	}
	return ""
}

func (e *Engine) dropPairedPeer(id string) {
	if e.paired != nil {
		e.paired.Remove(id)
	}
	e.removeOverlay(id)
	e.emitPeers()
}

// ReceiveRead implements transport.MessageHandler: a peer acknowledges reading
// our messages up to rr.UpToTs. We bump the per-peer watermark and surface it.
func (e *Engine) ReceiveRead(rr protocol.ReadReceipt, remoteAddr, webToken string) int {
	readerID := rr.Reader.ID
	if readerID == "" {
		readerID = remoteAddr
	}
	if rr.Reader.Platform == protocol.PlatformWeb {
		if e.webPresence == nil || !e.webPresence.Verify(readerID, remoteAddr, webToken) {
			return http.StatusForbidden
		}
	}
	if e.chatLim != nil && !e.chatLim.allow("read:"+readerID) {
		return http.StatusTooManyRequests
	}
	if rr.UpToTs <= 0 {
		return http.StatusBadRequest
	}
	e.readMu.Lock()
	if rr.UpToTs > e.readByPeer[readerID] {
		e.readByPeer[readerID] = rr.UpToTs
	}
	e.readMu.Unlock()
	e.emitRead(readerID, rr.UpToTs)
	return http.StatusOK
}

// PollWebChat returns pending desktop→browser messages and read receipts for a
// browser client. Implements transport.WebChatHandler.
func (e *Engine) PollWebChat(clientID, remoteAddr, webToken string, since int64) (protocol.WebChatPollResponse, int) {
	if e.webPresence == nil || !e.webPresence.Verify(clientID, remoteAddr, webToken) {
		return protocol.WebChatPollResponse{}, http.StatusForbidden
	}
	var out []protocol.ChatMessage
	e.webOutMu.Lock()
	pending := e.webOutbox[clientID]
	for _, m := range pending {
		if m.Ts > since {
			out = append(out, m)
		}
	}
	if len(pending) > 0 {
		delivered := make(map[int64]struct{}, len(out))
		for _, m := range out {
			delivered[m.Ts] = struct{}{}
		}
		var remain []protocol.ChatMessage
		for _, m := range pending {
			if _, ok := delivered[m.Ts]; !ok {
				remain = append(remain, m)
			}
		}
		if len(remain) == 0 {
			delete(e.webOutbox, clientID)
		} else {
			e.webOutbox[clientID] = remain
		}
	}
	e.webOutMu.Unlock()

	e.webReadMu.Lock()
	readUpTo := e.webReadNotify[clientID]
	e.webReadMu.Unlock()

	if len(out) == 0 && readUpTo <= 0 {
		return protocol.WebChatPollResponse{}, http.StatusNoContent
	}
	return protocol.WebChatPollResponse{Messages: out, ReadUpTo: readUpTo}, http.StatusOK
}

// rateLimiter is a small per-key sliding-window limiter guarding the message
// endpoint against spam/DoS.
type rateLimiter struct {
	mu     sync.Mutex
	events map[string][]int64
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{events: make(map[string][]int64), limit: limit, window: window}
}

func (r *rateLimiter) allow(key string) bool {
	now := time.Now().UnixNano()
	cutoff := now - int64(r.window)
	r.mu.Lock()
	defer r.mu.Unlock()
	ts := r.events[key]
	keep := ts[:0]
	for _, t := range ts {
		if t >= cutoff {
			keep = append(keep, t)
		}
	}
	if len(keep) >= r.limit {
		if len(keep) > 0 {
			r.events[key] = keep
		} else {
			delete(r.events, key)
		}
		return false
	}
	keep = append(keep, now)
	r.events[key] = keep
	return true
}

func (e *Engine) findPeer(id string) (protocol.DeviceInfo, bool) {
	for _, p := range e.Peers() {
		if p.ID == id {
			return p, true
		}
	}
	return protocol.DeviceInfo{}, false
}

// Self returns this device's advertised info.
func (e *Engine) Self() protocol.DeviceInfo {
	port := 0
	if e.server != nil {
		port = e.server.Port()
	}
	addr := e.boundIP
	if addr == "" {
		addr = primaryIP()
	}
	host, _ := os.Hostname()
	return protocol.DeviceInfo{
		ID:          e.id.DeviceID,
		Name:        e.id.Name,
		Host:        host,
		Address:     addr,
		Platform:    currentPlatform(),
		ControlPort: port,
		Fingerprint: e.id.Fingerprint,
		Version:     protocol.Version,
		Capabilities: []string{
			protocol.CapTCPPush,
			protocol.CapHTTPUpload,
			protocol.CapHTTPPull,
		},
	}
}

// interfaceIPv4 resolves an interface name to its first usable IPv4 address.
func interfaceIPv4(name string) (string, *net.Interface) {
	ifi, err := net.InterfaceByName(name)
	if err != nil {
		return "", nil
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return "", ifi
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
				return ip4.String(), ifi
			}
		}
	}
	return "", ifi
}

// primaryIP returns the local IPv4 address that would be used to reach the LAN.
// It opens no actual connection; a UDP "dial" only selects the routing source.
func primaryIP() string {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if ua, ok := conn.LocalAddr().(*net.UDPAddr); ok && ua.IP != nil {
		return ua.IP.String()
	}
	return ""
}

// Start brings up the control plane, data plane and discovery until ctx is
// cancelled. ifaceName selects the network interface to advertise and discover
// on; an empty string auto-selects across all interfaces.
func (e *Engine) Start(ctx context.Context, ifaceName string) error {
	if e.started {
		return nil
	}

	if ifaceName != "" {
		ip, ifi := interfaceIPv4(ifaceName)
		if ifi != nil {
			e.iface = ifi
		}
		if ip != "" {
			e.boundIP = ip
		}
		e.logf("using interface %q (advertise IP %q)", ifaceName, ip)
	}
	if e.boundIP == "" {
		e.boundIP = primaryIP()
		e.logf("auto-selected advertise IP %q", e.boundIP)
	}

	runCtx, runCancel := context.WithCancel(ctx)
	e.mgr.SetRunContext(runCtx)

	if err := e.mgr.StartDataPlane(runCtx); err != nil {
		runCancel()
		e.logf("data plane failed to start: %v", err)
		return err
	}
	e.logf("data plane listening on :%d", e.mgr.DataPort())

	e.server = transport.NewServer(e.id, e.Self, e.mgr)
	e.server.SetLogf(e.logf)
	e.server.SetMessageHandler(e)
	e.server.SetHTTPUploadHandler(e.mgr)
	e.server.SetHTTPPullHandler(e.mgr)
	e.webPresence = webpresence.New(func() int {
		if e.server != nil {
			return e.server.Port()
		}
		return 0
	})
	e.server.SetPresenceHandler(e.webPresence)
	e.server.SetWebChatHandler(e)
	e.mgr.SetWebVerifier(e.webPresence.Verify)
	if err := e.server.Start(runCtx, protocol.DefaultControlPort); err != nil {
		runCancel()
		e.logf("control plane failed to bind :%d: %v (is another Swoop instance running?)", protocol.DefaultControlPort, err)
		return err
	}
	e.logf("control plane listening on :%d (advertise IP %s)", e.server.Port(), e.boundIP)

	e.disco = discovery.New(e.Self())
	e.paired = pairing.New()
	if e.iface != nil {
		e.disco.SetInterface(e.iface)
	}
	if e.onPeers != nil {
		e.disco.OnChange(func([]protocol.DeviceInfo) { e.emitPeers() })
		e.paired.OnChange(func([]protocol.DeviceInfo) { e.emitPeers() })
		e.webPresence.OnChange(func([]protocol.DeviceInfo) { e.emitPeers() })
	}
	e.paired.OnProbe(e.probePairedPeer)
	e.paired.OnRemove(func(id string) { e.removeOverlay(id) })
	e.paired.SetIdlePolicy(e.mgr.IsBusyWith, e.onPairedPeerIdle)
	go e.paired.Start(runCtx)
	go e.webPresence.Start(runCtx)
	if err := e.disco.Start(runCtx); err != nil {
		runCancel()
		return err
	}
	e.runCancel = runCancel
	e.started = true
	return nil
}

func (e *Engine) emitPeers() {
	if e.onPeers == nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			e.logf("panic in peers callback: %v", rec)
		}
	}()
	e.onPeers(e.Peers())
}

// Peers returns LAN-discovered peers, invite-paired peers, then browser clients.
func (e *Engine) Peers() []protocol.DeviceInfo {
	seen := make(map[string]bool)
	var out []protocol.DeviceInfo
	if e.disco != nil {
		for _, p := range e.disco.Peers() {
			out = append(out, p)
			seen[p.ID] = true
		}
	}
	if e.paired != nil {
		for _, p := range e.paired.Peers() {
			if !seen[p.ID] {
				out = append(out, p)
				seen[p.ID] = true
			}
		}
	}
	if e.webPresence != nil {
		for _, p := range e.webPresence.Peers() {
			if !seen[p.ID] {
				out = append(out, p)
			}
		}
	}
	for i := range out {
		out[i] = e.decoratePeer(out[i])
	}
	return out
}

func (e *Engine) wireOverlayLink(link *overlay.Link) {
	if link == nil {
		return
	}
	link.SetOnModeChange(func() { e.emitPeers() })
}

func (e *Engine) decoratePeer(p protocol.DeviceInfo) protocol.DeviceInfo {
	if p.Paired {
		p.ConnectReach = protocol.ConnectInternet
		if link := e.overlayForPeer(p.ID); link != nil {
			switch link.Mode() {
			case "relay":
				p.ConnectPath = protocol.ConnectRelay
				p.P2PNote = link.P2PNote()
			case "direct":
				p.ConnectPath = protocol.ConnectP2P
				p.P2PNote = ""
			}
		} else if p.PairStatus == pairing.StatusConnected {
			p.ConnectPath = protocol.ConnectP2P
		}
	} else {
		p.ConnectReach = protocol.ConnectLocal
	}
	return p
}

// GenerateInvite creates a signed SwoopInvite blob for internet pairing.
// Starts a local UDP punch listener and, when possible, maps ports via UPnP (no remote servers).
func (e *Engine) GenerateInvite() (invite.Bundle, error) {
	if !e.started {
		return invite.Bundle{}, fmt.Errorf("%s", i18n.Pick("Сначала запустите Swoop", "Start Swoop first"))
	}
	e.stopInviteHost()

	punchConn, punchPort, err := pairing.ListenPunchUDP()
	if err != nil {
		return invite.Bundle{}, err
	}

	var reach *invite.Reach
	var natRelease func()
	if m, ok := nat.TryMapPorts(context.Background(), e.server.Port(), e.mgr.DataPort(), punchPort, e.logf); ok {
		reach = &invite.Reach{
			Addr:        m.ExternalIP,
			ControlPort: m.ExternalControl,
			PunchPort:   m.ExternalPunch,
		}
		natRelease = m.Release
	}

	bundle, err := invite.Create(e.id, e.Self(), 0, reach)
	if err != nil {
		punchConn.Close()
		if natRelease != nil {
			natRelease()
		}
		return invite.Bundle{}, err
	}

	hostCtx, cancel := context.WithCancel(context.Background())
	e.inviteHostMu.Lock()
	e.inviteHostCancel = cancel
	e.inviteNATRelease = natRelease
	e.invitePunchConn = punchConn
	e.inviteSessionID = bundle.SessionID
	e.invitePunchPort = punchPort
	e.inviteReach = reach
	e.inviteHostMu.Unlock()

	go pairing.RunPunchHost(hostCtx, punchConn, bundle.SessionID, e.logf)
	go e.inviteHostWaitExpire(bundle.ExpiresAt)
	go e.rendezvousHostLoop(hostCtx, bundle.SessionID, punchConn, punchPort, reach, bundle.ExpiresAt)

	if reach != nil {
		e.logf("internet invite: public %s:%d punch UDP %d", reach.Addr, reach.ControlPort, reach.PunchPort)
	} else {
		e.logf("internet invite: no UPnP — LAN punch only (UDP %d)", punchPort)
	}
	return bundle, nil
}

func (e *Engine) rendezvousHostLoop(ctx context.Context, sessionID string, punchConn *net.UDPConn, punchPort int, reach *invite.Reach, expiresAt int64) {
	if !rendezvous.Enabled() {
		return
	}
	self := e.Self()
	client := rendezvous.NewClient()
	regCtx, regCancel := context.WithTimeout(ctx, clientTimeout())
	defer regCancel()
	req := rendezvous.HostRegisterRequest{
		SessionID:   sessionID,
		PeerID:      self.ID,
		DeviceName:  self.Name,
		LanAddr:     self.Address,
		ControlPort: e.server.Port(),
		PunchPort:   punchPort,
	}
	if reach != nil {
		req.ReachAddr = reach.Addr
		req.ReachPort = reach.ControlPort
	}
	if err := client.RegisterHost(regCtx, req); err != nil {
		e.logf("rendezvous register: %v", err)
		return
	}
	if reach == nil {
		e.logf("rendezvous: session registered (signaling only); no UPnP reach — importer must reach your public IP on TCP %d", e.server.Port())
	} else {
		e.logf("rendezvous: session registered (signaling only)")
	}

	go func() {
		reachAddr := ""
		if reach != nil {
			reachAddr = reach.Addr
		}
		link, err := overlay.ServeHost(ctx, overlay.HostParams{
			SessionID:   sessionID,
			PeerID:      self.ID,
			ControlPort: e.server.Port(),
			DataPort:    e.mgr.DataPort(),
			HostCert:    e.id.Certificate,
			ReachAddr:   reachAddr,
			LanAddr:     self.Address,
			PunchPort:   punchPort,
			PunchConn:   punchConn,
			OnQuicBound: func(quicPort int) {
				if quicPort <= 0 {
					return
				}
				mapCtx, mapCancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer mapCancel()
				if rel, ok := nat.TryMapUDPPort(mapCtx, quicPort, e.logf); ok {
					e.inviteHostMu.Lock()
					e.inviteQuicRelease = rel
					e.inviteHostMu.Unlock()
				}
			},
			Logf: e.logf,
		})
		if err != nil {
			e.logf("overlay host: %v", err)
			return
		}
		e.wireOverlayLink(link)
		e.inviteHostMu.Lock()
		e.inviteOverlay = link
		e.inviteHostMu.Unlock()
		e.logf("overlay: host relay tunnel ready")
		<-ctx.Done()
		_ = link.Close()
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollCtx, pollCancel := context.WithTimeout(ctx, 4*time.Second)
			j, ok, err := client.PollJoiner(pollCtx, sessionID, self.ID)
			pollCancel()
			if err != nil || !ok {
				continue
			}
			e.logf("rendezvous: joiner %s at %s:%d — reverse punch", j.PeerID, j.ReflexiveAddr, j.PunchPort)
			e.pairRendezvousJoiner(sessionID, j, expiresAt)
			_ = pairing.SendPunchHello(punchConn, sessionID, j.ReflexiveAddr, j.PunchPort)
		}
	}
}

func clientTimeout() time.Duration { return 8 * time.Second }

func (e *Engine) stopInviteHost() {
	e.inviteHostMu.Lock()
	cancel := e.inviteHostCancel
	conn := e.invitePunchConn
	release := e.inviteNATRelease
	ol := e.inviteOverlay
	e.inviteHostCancel = nil
	e.invitePunchConn = nil
	e.inviteNATRelease = nil
	e.inviteOverlay = nil
	e.inviteJoinerID = ""
	e.inviteSessionID = ""
	e.invitePunchPort = 0
	e.inviteReach = nil
	quicRelease := e.inviteQuicRelease
	e.inviteQuicRelease = nil
	e.inviteHostMu.Unlock()
	if quicRelease != nil {
		quicRelease()
	}
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if release != nil {
		release()
	}
	if ol != nil {
		_ = ol.Close()
	}
}

func (e *Engine) setOverlay(peerID string, link *overlay.Link, punch *net.UDPConn) {
	e.overlayMu.Lock()
	if e.overlays == nil {
		e.overlays = make(map[string]*overlay.Link)
	}
	if old := e.overlays[peerID]; old != nil {
		_ = old.Close()
	}
	e.overlays[peerID] = link
	e.overlayMu.Unlock()
	if punch != nil {
		e.punchMu.Lock()
		if e.punchConn == nil {
			e.punchConn = make(map[string]*net.UDPConn)
		}
		if old := e.punchConn[peerID]; old != nil && old != punch {
			_ = old.Close()
		}
		e.punchConn[peerID] = punch
		e.punchMu.Unlock()
	}
}

func (e *Engine) overlayForPeer(peerID string) *overlay.Link {
	e.overlayMu.Lock()
	link := e.overlays[peerID]
	e.overlayMu.Unlock()
	if link != nil {
		return link
	}
	e.inviteHostMu.Lock()
	defer e.inviteHostMu.Unlock()
	if e.inviteOverlay != nil && peerID == e.inviteJoinerID {
		return e.inviteOverlay
	}
	return nil
}

func (e *Engine) removeOverlay(peerID string) {
	e.overlayMu.Lock()
	link := e.overlays[peerID]
	delete(e.overlays, peerID)
	e.overlayMu.Unlock()
	if link != nil {
		_ = link.Close()
	}
	e.punchMu.Lock()
	if pc := e.punchConn[peerID]; pc != nil {
		_ = pc.Close()
		delete(e.punchConn, peerID)
	}
	e.punchMu.Unlock()
}

// inviteHostWaitExpire stops an unpaired invite host when the invite blob expires.
// Once a joiner pairs, idle policy owns session lifetime (transfers may exceed 15 min).
func (e *Engine) inviteHostWaitExpire(expiresAt int64) {
	if expiresAt <= 0 {
		return
	}
	wait := time.Until(time.Unix(expiresAt, 0))
	if wait < 0 {
		wait = 0
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	<-t.C
	e.inviteHostMu.Lock()
	paired := e.inviteJoinerID != ""
	e.inviteHostMu.Unlock()
	if paired {
		return
	}
	e.logf("internet invite: expired with no joiner")
	e.stopInviteHost()
}

// PairFromInvite registers a verified invite peer and probes reachability.
func (e *Engine) PairFromInvite(parsed invite.Parsed) error {
	if !e.started || e.paired == nil {
		return fmt.Errorf("%s", i18n.Pick("Сначала запустите Swoop", "Start Swoop first"))
	}
	if time.Now().Unix() > parsed.ExpiresAt {
		return invite.ErrExpired
	}
	if parsed.Device.ID == e.id.DeviceID {
		return i18n.ErrPairSelf()
	}
	dev := parsed.DialDevice()
	if len(dev.Capabilities) == 0 {
		dev.Capabilities = []string{
			protocol.CapTCPPush,
			protocol.CapHTTPUpload,
			protocol.CapHTTPPull,
		}
	}
	e.paired.Add(dev, parsed)
	e.touchInternetActivity(dev.ID)
	e.emitPeers()
	return nil
}

func (e *Engine) probePairedPeer(id string) {
	if e.paired == nil {
		return
	}
	peer, ok := e.paired.Get(id)
	if !ok {
		e.removeOverlay(id)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inv, hasInv := e.paired.InviteMeta(id)
	if hasInv && rendezvous.Enabled() {
		hostSide := inv.Device.ID == e.id.DeviceID
		if !hostSide && e.overlayForPeer(id) == nil {
			var joinerReflexive string
			var punchConn *net.UDPConn
			inv, peer, punchConn, joinerReflexive = e.rendezvousJoin(ctx, inv, peer, id)
			// Connect overlay relay before punch: ClientPunch uses the same ctx and
			// can burn the full deadline, leaving no time for the WebSocket tunnel.
			link, err := overlay.ConnectJoinerRetry(ctx, inv.SessionID, e.id.DeviceID)
			if err != nil {
				if punchConn != nil {
					_ = punchConn.Close()
				}
				e.logf("paired peer %q overlay: %v", peer.Name, err)
				e.paired.SetStatus(id, pairing.StatusError)
				e.emitPeers()
				return
			}
			e.setOverlay(id, link, punchConn)
			e.wireOverlayLink(link)
			link.StartJoinerRelay(context.Background(), e.server.Port(), e.mgr.DataPort())
			e.logf("overlay: tunnel to %q ready (%s)", peer.Name, link.Mode())
			joinerPunch := 0
			if punchConn != nil {
				if a, ok := punchConn.LocalAddr().(*net.UDPAddr); ok {
					joinerPunch = a.Port
				}
			}
			reachAddr := inv.ReachAddr
			if reachAddr == "" {
				reachAddr = inv.Device.Address
			}
			link.StartUpgrade(context.Background(), overlay.UpgradeParams{
				SessionID:       inv.SessionID,
				Fingerprint:     peer.Fingerprint,
				ReachAddr:       reachAddr,
				LanAddr:         inv.Device.Address,
				HostPunchPort:   inv.PunchPort,
				JoinerPunchPort: joinerPunch,
				JoinerReflexive: joinerReflexive,
				JoinerPeerID:    e.id.DeviceID,
				PunchConn:       punchConn,
				Logf:            e.logf,
			})
		}
		if link := e.overlayForPeer(id); link != nil {
			live, err := link.ProbeInfo(ctx, peer)
			if err != nil {
				e.logf("paired peer %q unreachable: %v", peer.Name, err)
				e.dropPairedPeer(id)
				return
			}
			live.Paired = true
			e.paired.Update(id, live)
			e.touchInternetActivity(id)
			e.emitPeers()
			if link.Mode() == "direct" {
				e.logf("paired peer %q using direct QUIC P2P", peer.Name)
			}
			return
		}
		if hostSide {
			return
		}
	}

	// Fallback: direct TCP when overlay unavailable (LAN / UPnP).
	var punchConn *net.UDPConn
	if hasInv && inv.PunchPort > 0 {
		var err error
		punchConn, _, err = pairing.ListenPunchUDP()
		if err == nil {
			_ = pairing.ClientPunch(ctx, inv, e.id.DeviceID, punchConn)
		}
	}
	if punchConn != nil {
		defer punchConn.Close()
	}
	live, err := pairing.ProbeInfo(ctx, peer)
	if err != nil {
		e.logf("paired peer %q probe failed: %v", peer.Name, err)
		e.paired.SetStatus(id, pairing.StatusError)
		e.emitPeers()
		return
	}
	e.paired.Update(id, live)
	e.touchInternetActivity(id)
	e.emitPeers()
}

func (e *Engine) pairRendezvousJoiner(sessionID string, j rendezvous.JoinerInfo, expiresAt int64) {
	if e.paired == nil || j.PeerID == "" || j.PeerID == e.id.DeviceID {
		return
	}
	if _, ok := e.paired.Get(j.PeerID); ok {
		return
	}
	self := e.Self()
	caps := j.Capabilities
	if len(caps) == 0 {
		caps = []string{protocol.CapTCPPush, protocol.CapHTTPUpload, protocol.CapHTTPPull}
	}
	addr := j.LanAddr
	if addr == "" {
		addr = j.ReflexiveAddr
	}
	controlPort := j.ControlPort
	if controlPort == 0 {
		controlPort = protocol.DefaultControlPort
	}
	dev := protocol.DeviceInfo{
		ID: j.PeerID, Name: j.DeviceName, Fingerprint: j.Fingerprint,
		Address: addr, ControlPort: controlPort, Capabilities: caps,
	}
	if dev.Name == "" {
		dev.Name = j.PeerID[:8]
	}
	inv := invite.Parsed{
		SessionID: sessionID,
		ExpiresAt: expiresAt,
		Device:    self,
	}
	e.inviteHostMu.Lock()
	e.inviteJoinerID = j.PeerID
	e.inviteHostMu.Unlock()
	e.paired.Add(dev, inv)
	e.touchInternetActivity(dev.ID)
	e.logf("internet invite: joiner %q paired on host (relay)", dev.Name)
}

func (e *Engine) rendezvousJoin(ctx context.Context, inv invite.Parsed, peer protocol.DeviceInfo, id string) (invite.Parsed, protocol.DeviceInfo, *net.UDPConn, string) {
	punchConn, punchPort, err := pairing.ListenPunchUDP()
	if err != nil {
		return inv, peer, nil, ""
	}

	self := e.Self()
	client := rendezvous.NewClient()
	host, err := client.Join(ctx, rendezvous.JoinRequest{
		SessionID:    inv.SessionID,
		PeerID:       e.id.DeviceID,
		PunchPort:    punchPort,
		LanAddr:      self.Address,
		DeviceName:   self.Name,
		Fingerprint:  self.Fingerprint,
		ControlPort:  self.ControlPort,
		Capabilities: self.Capabilities,
	})
	if err != nil {
		e.logf("rendezvous join: %v", err)
		punchConn.Close()
		return inv, peer, nil, ""
	}
	e.logf("rendezvous: joined session, host reflexive %s (local punch UDP %d)", host.ReflexiveAddr, punchPort)
	if host.ReachAddr == "" && host.ReflexiveAddr == rendezvous.DefaultServerHost {
		e.logf("rendezvous: warning: host reflexive equals rendezvous server — VPN or same host? TCP to %s:%d may not reach Swoop on the inviter", host.ReflexiveAddr, host.ControlPort)
	}
	inv = rendezvous.ApplyHostInfo(inv, host)
	peer = inv.DialDevice()
	if inv.HasReach() {
		e.logf("paired peer %q: reach %s:%d punch UDP %d", peer.Name, inv.ReachAddr, inv.ReachPort, inv.PunchPort)
	} else {
		e.logf("paired peer %q: reflexive %s:%d punch UDP %d", peer.Name, peer.Address, peer.ControlPort, inv.PunchPort)
	}
	peer.Paired = true
	peer.PairStatus = pairing.StatusConnecting
	e.paired.UpdateInvite(id, inv, peer)
	e.emitPeers()
	return inv, peer, punchConn, host.JoinerReflexive
}

// ImportInviteBytes parses a .swoopinvite file or invite PNG.
func (e *Engine) ImportInviteBytes(data []byte) (invite.Parsed, error) {
	var blob string
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		b, err := invite.DecodeFromPNG(data)
		if err != nil {
			return invite.Parsed{}, err
		}
		blob = b
	} else {
		blob = invite.BlobFromFile(data)
	}
	return invite.ParseAndVerify(blob)
}

func currentPlatform() protocol.Platform {
	switch runtime.GOOS {
	case "windows":
		return protocol.PlatformWindows
	case "darwin":
		return protocol.PlatformMacOS
	case "linux":
		return protocol.PlatformLinux
	default:
		return protocol.Platform(runtime.GOOS)
	}
}
