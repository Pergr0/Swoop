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
	"swoop/core/identity"
	"swoop/core/netif"
	"swoop/core/paths"
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

	onPeers func([]protocol.DeviceInfo)
	onChat  func(chat.Message)
	onRead  func(peerID string, upToTs int64)

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
func (e *Engine) OnTransferOffer(fn func(transfer.Offer)) { e.mgr.SetOnOffer(fn) }

// OnTransferProgress registers a callback for transfer progress updates.
func (e *Engine) OnTransferProgress(fn func(transfer.Progress)) { e.mgr.SetOnProgress(fn) }

// OnTransferState registers a callback for transfer lifecycle updates.
func (e *Engine) OnTransferState(fn func(transfer.State)) { e.mgr.SetOnState(fn) }

// SendTo starts an outgoing transfer to the peer with the given id.
func (e *Engine) SendTo(deviceID string, items []protocol.SendItem) error {
	if deviceID == e.id.DeviceID {
		return fmt.Errorf("нельзя отправить файлы самому себе")
	}
	peer, ok := e.findPeer(deviceID)
	if !ok {
		return fmt.Errorf("устройство не найдено: %s", deviceID)
	}
	if peer.Platform != protocol.PlatformWeb && (peer.Address == "" || peer.ControlPort == 0) {
		return fmt.Errorf("у устройства %q нет адреса для подключения", peer.Name)
	}
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
		return fmt.Errorf("пустое сообщение")
	}
	if len(text) > protocol.MaxMessageBytes {
		return fmt.Errorf("сообщение слишком длинное (макс %d байт)", protocol.MaxMessageBytes)
	}
	if !utf8.ValidString(text) {
		return fmt.Errorf("сообщение должно быть корректным UTF-8")
	}
	peer, ok := e.findPeer(deviceID)
	if !ok {
		return fmt.Errorf("устройство не найдено")
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
		return fmt.Errorf("у устройства %q нет отпечатка TLS", peer.Name)
	}
	if peer.Address == "" || peer.ControlPort == 0 {
		return fmt.Errorf("у устройства %q нет адреса для подключения", peer.Name)
	}

	body, _ := json.Marshal(protocol.ChatMessage{Sender: e.Self(), Text: text, Ts: ts})
	client := transport.NewPinnedClient(peer.Fingerprint, 10*time.Second)
	url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/message"
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		e.logf("chat send to %s failed: %v", peer.Name, err)
		return fmt.Errorf("не удалось отправить: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		e.logf("chat send to %s rejected: HTTP %d", peer.Name, resp.StatusCode)
		return fmt.Errorf("получатель отклонил сообщение (HTTP %d)", resp.StatusCode)
	}

	rec := chat.Message{Ts: ts, PeerID: peer.ID, PeerName: peer.Name, Dir: "out", Text: text}
	if e.chat != nil {
		_ = e.chat.Append(rec)
	}
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
	if peer.Fingerprint == "" || peer.Address == "" || peer.ControlPort == 0 {
		return
	}
	body, _ := json.Marshal(protocol.ReadReceipt{Reader: e.Self(), UpToTs: upTo})
	client := transport.NewPinnedClient(peer.Fingerprint, 5*time.Second)
	url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/read"
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		e.logf("read receipt to %s failed: %v", peer.Name, err)
		return
	}
	_ = resp.Body.Close()
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
	if e.iface != nil {
		e.disco.SetInterface(e.iface)
	}
	if e.onPeers != nil {
		e.disco.OnChange(func([]protocol.DeviceInfo) { e.emitPeers() })
		e.webPresence.OnChange(func([]protocol.DeviceInfo) { e.emitPeers() })
	}
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

// Peers returns LAN-discovered peers followed by connected browser clients.
func (e *Engine) Peers() []protocol.DeviceInfo {
	var out []protocol.DeviceInfo
	if e.disco != nil {
		out = append(out, e.disco.Peers()...)
	}
	if e.webPresence != nil {
		out = append(out, e.webPresence.Peers()...)
	}
	return out
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
