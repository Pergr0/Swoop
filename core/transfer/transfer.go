// Package transfer implements the file-transfer engine: a push model with
// receiver confirmation (AirDrop-like), built for throughput.
//
//   - Control plane (HTTPS, see package transport) carries the prepare-upload
//     handshake; the receiver blocks it until the user accepts or declines.
//   - Data plane (raw TCP here) carries file bytes over N parallel streams.
//     Each file is split into byte ranges distributed across the streams, so a
//     single large file still saturates the link. The receiver writes ranges
//     concurrently with WriteAt.
//
// Only one outgoing and one incoming session may be active at a time.
package transfer

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"swoop/core/protocol"
	"swoop/core/staging"
)

// Direction marks whether a transfer is outgoing or incoming.
type Direction string

const (
	DirSend Direction = "send"
	DirRecv Direction = "recv"
)

const (
	acceptTimeout = 60 * time.Second
	idleTimeout   = 30 * time.Second
	frameHeader   = 20 // uint32 fileIndex + int64 offset + int64 length
	tokenLen      = 32 // hex chars
)

// Offer describes an incoming upload request shown to the user.
type Offer struct {
	Sender     protocol.DeviceInfo      `json:"sender"`
	Files      []protocol.FileMeta      `json:"files"`
	TotalSize  int64                    `json:"totalSize"`
	Count      int                     `json:"count"`
	RootDirs   []staging.RootDirInfo    `json:"rootDirs"`
	LooseFiles int                      `json:"looseFiles"`
}

// Progress is a periodic transfer progress update.
type Progress struct {
	Direction  Direction `json:"direction"`
	Bytes      int64     `json:"bytes"`
	Total      int64     `json:"total"`
	Speed      float64   `json:"speed"`      // bytes per second
	ETASeconds float64   `json:"etaSeconds"` // estimated time remaining
	Streams    int       `json:"streams"`
	FileIndex  int       `json:"fileIndex"`
	FileName   string    `json:"fileName"`
	Peer       string    `json:"peer"`
}

// State is a transfer lifecycle update.
type State struct {
	Direction Direction `json:"direction"`
	State     string    `json:"state"` // waiting|transferring|completed|declined|failed|canceled
	Message   string    `json:"message"`
	Peer      string    `json:"peer"`
}

// Manager owns the transfer state and the data-plane listener.
type Manager struct {
	self         func() protocol.DeviceInfo
	streams      int
	chunkSize    int64
	downloadsDir string

	dataPort int

	mu       sync.Mutex
	outgoing *sendSession
	incoming *recvSession

	onOffer    func(Offer)
	onProgress func(Progress)
	onState    func(State)
	logf       func(string, ...any)

	webVerify func(clientID, remoteAddr, webToken string) bool
}

// SetWebVerifier installs browser client HMAC verification for HTTP pull APIs.
func (m *Manager) SetWebVerifier(fn func(clientID, remoteAddr, webToken string) bool) {
	m.webVerify = fn
}

// NewManager creates a transfer manager. downloadsDir is where received files
// are written.
func NewManager(self func() protocol.DeviceInfo, downloadsDir string) *Manager {
	return &Manager{
		self:         self,
		streams:      4,
		chunkSize:    4 << 20,
		downloadsDir: downloadsDir,
		logf:         func(string, ...any) {},
	}
}

func (m *Manager) SetOnOffer(fn func(Offer))       { m.onOffer = fn }
func (m *Manager) SetOnProgress(fn func(Progress)) { m.onProgress = fn }
func (m *Manager) SetOnState(fn func(State))       { m.onState = fn }

// SetLogf installs a logging function.
func (m *Manager) SetLogf(fn func(string, ...any)) {
	if fn != nil {
		m.logf = fn
	}
}

// DataPort returns the bound data-plane port (0 until StartDataPlane).
func (m *Manager) DataPort() int { return m.dataPort }

// safeCall runs a UI callback, containing any panic so it cannot propagate into
// (and abort) the HTTP handler or a transfer goroutine.
func (m *Manager) safeCall(what string, fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			m.logf("panic in %s callback: %v", what, rec)
		}
	}()
	if fn != nil {
		fn()
	}
}

func (m *Manager) emitOffer(o Offer) {
	m.safeCall("offer", func() {
		if m.onOffer != nil {
			m.onOffer(o)
		}
	})
}
func (m *Manager) emitProgress(p Progress) {
	m.safeCall("progress", func() {
		if m.onProgress != nil {
			m.onProgress(p)
		}
	})
}
func (m *Manager) emitState(s State) {
	m.safeCall("state", func() {
		if m.onState != nil {
			m.onState(s)
		}
	})
}

// StartDataPlane binds the raw-TCP data listener and serves until ctx is done.
func (m *Manager) StartDataPlane(ctx context.Context) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(protocol.DefaultDataPort))
	if err != nil {
		// Fall back to an OS-chosen port.
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			return err
		}
	}
	m.dataPort = ln.Addr().(*net.TCPAddr).Port

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go m.handleDataConn(conn)
		}
	}()
	return nil
}

// ---------------------------------------------------------------------------
// Receiver side
// ---------------------------------------------------------------------------

type recvSession struct {
	id        string
	sender    protocol.DeviceInfo
	files     []protocol.FileMeta
	total     int64
	token     string
	mode      string // protocol.TransferTCPPush or TransferHTTPUpload
	decision  chan bool
	handles   []*os.File
	destPaths []string

	received int64 // atomic
	lastSeen int64 // atomic unix-nano
	start    time.Time

	done     chan struct{}
	cancel   int32 // atomic; 1 = canceled by user
	finalize sync.Once
	errOnce  sync.Once
	err      error
}

func (s *recvSession) touch()         { atomic.StoreInt64(&s.lastSeen, time.Now().UnixNano()) }
func (s *recvSession) setErr(e error) { s.errOnce.Do(func() { s.err = e }) }
func (s *recvSession) canceled() bool { return atomic.LoadInt32(&s.cancel) == 1 }

// PrepareUpload implements transport.UploadHandler. It registers the offer,
// notifies the UI, and blocks until the user accepts/declines or it times out.
func (m *Manager) PrepareUpload(req protocol.PrepareUploadRequest, remoteAddr string) (protocol.PrepareUploadResponse, int) {
	if err := ValidateOfferFiles(req.Files); err != nil {
		m.logf("incoming offer from %s rejected: %v", req.Sender.Name, err)
		return protocol.PrepareUploadResponse{}, http.StatusBadRequest
	}
	if req.Sender.Platform == protocol.PlatformWeb && remoteAddr != "" {
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			req.Sender.Address = host
		}
	}

	m.mu.Lock()
	if m.incoming != nil {
		m.mu.Unlock()
		m.logf("incoming offer from %s rejected: already receiving", req.Sender.Name)
		return protocol.PrepareUploadResponse{}, http.StatusConflict // 409: busy
	}
	var total int64
	for _, f := range req.Files {
		total += f.Size
	}
	sess := &recvSession{
		id:       randHex(8),
		sender:   req.Sender,
		files:    req.Files,
		total:    total,
		decision: make(chan bool, 1),
		done:     make(chan struct{}),
	}
	m.incoming = sess
	m.mu.Unlock()

	summary := staging.SummarizeOffer(fileMetasLite(req.Files))
	m.logf("incoming offer from %s: %d file(s), %d bytes; awaiting user decision", req.Sender.Name, len(req.Files), total)
	m.emitOffer(Offer{
		Sender: req.Sender, Files: req.Files, TotalSize: total, Count: len(req.Files),
		RootDirs: summary.RootDirs, LooseFiles: summary.LooseFiles,
	})
	m.emitState(State{Direction: DirRecv, State: "waiting", Message: "Запрос на приём файлов", Peer: req.Sender.Name})

	select {
	case accept := <-sess.decision:
		m.logf("incoming offer from %s: user decision accept=%v", req.Sender.Name, accept)
		if !accept {
			m.clearIncoming(sess)
			m.emitState(State{Direction: DirRecv, State: "declined", Peer: req.Sender.Name})
			return protocol.PrepareUploadResponse{}, http.StatusForbidden // 403
		}
	case <-time.After(acceptTimeout):
		m.clearIncoming(sess)
		m.emitState(State{Direction: DirRecv, State: "failed", Message: "Время ожидания истекло", Peer: req.Sender.Name})
		return protocol.PrepareUploadResponse{}, http.StatusRequestTimeout // 408
	}

	if err := m.prepareDestFiles(sess); err != nil {
		m.clearIncoming(sess)
		m.emitState(State{Direction: DirRecv, State: "failed", Message: err.Error(), Peer: req.Sender.Name})
		return protocol.PrepareUploadResponse{}, http.StatusInternalServerError
	}

	sess.token = randHex(16)
	sess.start = time.Now()
	sess.touch()
	m.emitState(State{Direction: DirRecv, State: "transferring", Peer: req.Sender.Name})
	go m.reportLoop(DirRecv, sess.sender.Name, sess.files, sess.total, &sess.received, sess.start, sess.done)
	go m.watchdog(sess)

	if total == 0 {
		sess.finalizeRecv(m, true)
	}

	if req.Sender.Platform == protocol.PlatformWeb {
		sess.mode = protocol.TransferHTTPUpload
		return protocol.PrepareUploadResponse{
			SessionID:  sess.id,
			Mode:       protocol.TransferHTTPUpload,
			Token:      sess.token,
			UploadPath: "/api/v1/upload/" + sess.id,
		}, http.StatusOK
	}

	sess.mode = protocol.TransferTCPPush
	return protocol.PrepareUploadResponse{
		SessionID: sess.id,
		Mode:      protocol.TransferTCPPush,
		DataPort:  m.dataPort,
		Token:     sess.token,
	}, http.StatusOK
}

// RespondIncoming resolves the pending incoming offer.
func (m *Manager) RespondIncoming(accept bool) {
	m.mu.Lock()
	sess := m.incoming
	m.mu.Unlock()
	if sess == nil {
		return
	}
	select {
	case sess.decision <- accept:
	default:
	}
}

// CancelIncoming aborts the active incoming transfer, if any.
func (m *Manager) CancelIncoming() {
	m.mu.Lock()
	sess := m.incoming
	m.mu.Unlock()
	if sess == nil || sess.token == "" {
		return
	}
	atomic.StoreInt32(&sess.cancel, 1)
	sess.finalizeRecv(m, false)
}

func (m *Manager) prepareDestFiles(sess *recvSession) error {
	if err := os.MkdirAll(m.downloadsDir, 0o755); err != nil {
		return err
	}
	for _, f := range sess.files {
		rel := f.RelPath
		if rel == "" {
			rel = f.Name
		}
		dest := uniquePathInTree(m.downloadsDir, sanitizeRelPath(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		fh, err := os.Create(dest)
		if err != nil {
			return err
		}
		if f.Size > 0 {
			_ = fh.Truncate(f.Size)
		}
		sess.handles = append(sess.handles, fh)
		sess.destPaths = append(sess.destPaths, dest)
	}
	return nil
}

func (m *Manager) handleDataConn(conn net.Conn) {
	defer conn.Close()

	tok := make([]byte, tokenLen)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadFull(conn, tok); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	m.mu.Lock()
	sess := m.incoming
	m.mu.Unlock()
	if sess == nil || sess.token == "" || string(tok) != sess.token {
		m.logf("data stream from %s rejected: no matching session/token", conn.RemoteAddr())
		return
	}
	m.logf("data stream from %s accepted", conn.RemoteAddr())

	hdr := make([]byte, frameHeader)
	buf := make([]byte, 256*1024)
	for {
		select {
		case <-sess.done:
			return
		default:
		}
		if sess.canceled() {
			return
		}
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return // EOF: sender finished this stream
		}
		fi := binary.BigEndian.Uint32(hdr[0:4])
		off := int64(binary.BigEndian.Uint64(hdr[4:12]))
		ln := int64(binary.BigEndian.Uint64(hdr[12:20]))
		if int(fi) >= len(sess.handles) || int(fi) >= len(sess.files) {
			sess.setErr(errors.New("bad file index"))
			sess.finalizeRecv(m, false)
			return
		}
		fileSize := sess.files[fi].Size
		if ln < 0 || off < 0 || off+ln > fileSize {
			sess.setErr(errors.New("invalid chunk range"))
			sess.finalizeRecv(m, false)
			return
		}
		pos := off
		remaining := ln
		for remaining > 0 {
			toRead := int64(len(buf))
			if toRead > remaining {
				toRead = remaining
			}
			n, err := io.ReadFull(conn, buf[:toRead])
			if n > 0 {
				if _, werr := sess.handles[fi].WriteAt(buf[:n], pos); werr != nil {
					sess.setErr(werr)
					sess.finalizeRecv(m, false)
					return
				}
				pos += int64(n)
				remaining -= int64(n)
				sess.touch()
				if atomic.AddInt64(&sess.received, int64(n)) >= sess.total {
					sess.finalizeRecv(m, true)
					return
				}
			}
			if err != nil {
				return
			}
		}
	}
}

func (s *recvSession) finalizeRecv(m *Manager, success bool) {
	s.finalize.Do(func() {
		close(s.done)
		canceled := s.canceled()
		for _, h := range s.handles {
			_ = h.Close()
		}
		if canceled {
			for _, p := range s.destPaths {
				_ = os.Remove(p)
			}
		}
		m.clearIncoming(s)
		switch {
		case success && !canceled:
			m.logf("receive from %s completed: %d bytes -> %s", s.sender.Name, s.total, m.downloadsDir)
			m.emitProgress(Progress{Direction: DirRecv, Bytes: s.total, Total: s.total, Streams: m.streams, Peer: s.sender.Name})
			m.emitState(State{Direction: DirRecv, State: "completed", Message: "Файлы сохранены в " + m.downloadsDir, Peer: s.sender.Name})
		case canceled:
			m.logf("receive from %s canceled", s.sender.Name)
			m.emitState(State{Direction: DirRecv, State: "canceled", Message: "Отменено", Peer: s.sender.Name})
		default:
			msg := "Передача прервана"
			if s.err != nil {
				msg = s.err.Error()
			}
			m.logf("receive from %s failed: %s", s.sender.Name, msg)
			m.emitState(State{Direction: DirRecv, State: "failed", Message: msg, Peer: s.sender.Name})
		}
	})
}

func (m *Manager) watchdog(sess *recvSession) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-sess.done:
			return
		case <-t.C:
			last := atomic.LoadInt64(&sess.lastSeen)
			if time.Since(time.Unix(0, last)) > idleTimeout {
				sess.setErr(errors.New("нет данных от отправителя"))
				sess.finalizeRecv(m, false)
				return
			}
		}
	}
}

func (m *Manager) clearIncoming(sess *recvSession) {
	m.mu.Lock()
	if m.incoming == sess {
		m.incoming = nil
	}
	m.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Sender side
// ---------------------------------------------------------------------------

type sendSession struct {
	id       string
	peer     protocol.DeviceInfo
	files    []protocol.FileMeta
	srcPaths []string
	total    int64
	token    string

	sent  int64 // atomic
	start time.Time

	stop   chan struct{}
	cancel int32 // atomic; 1 = canceled by user

	webDecision  chan bool
	pullDone     map[string]bool
	pullDoneMu   sync.Mutex
	pullWait     chan struct{}
	pullWaitOnce sync.Once
	archiveTemp  string // temp .zip for multi-file browser pull; removed after session

	httpCancel   context.CancelFunc
	httpCancelMu sync.Mutex

	connsMu sync.Mutex
	conns   []net.Conn

	errOnce sync.Once
	err     error
}

func (s *sendSession) setErr(e error)  { s.errOnce.Do(func() { s.err = e }) }
func (s *sendSession) canceled() bool  { return atomic.LoadInt32(&s.cancel) == 1 }
func (s *sendSession) setHTTPCancel(cancel context.CancelFunc) {
	s.httpCancelMu.Lock()
	s.httpCancel = cancel
	s.httpCancelMu.Unlock()
}
func (s *sendSession) abortHTTP() {
	s.httpCancelMu.Lock()
	if s.httpCancel != nil {
		s.httpCancel()
	}
	s.httpCancelMu.Unlock()
}
func (s *sendSession) addConn(c net.Conn) {
	s.connsMu.Lock()
	s.conns = append(s.conns, c)
	s.connsMu.Unlock()
}
func (s *sendSession) closeConns() {
	s.connsMu.Lock()
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.connsMu.Unlock()
}

type chunk struct {
	fileIndex int
	offset    int64
	length    int64
}

// Send starts an outgoing transfer to peer. Returns an error immediately if a
// transfer is already in progress or there are no files.
func (m *Manager) Send(peer protocol.DeviceInfo, items []protocol.SendItem) error {
	if peer.Platform != protocol.PlatformWeb {
		if peer.Fingerprint == "" {
			return errors.New("у устройства нет отпечатка TLS")
		}
		if peer.Address == "" || peer.ControlPort == 0 {
			return errors.New("у устройства нет адреса для подключения")
		}
	}
	files, srcPaths, total, err := resolveSendFiles(items)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if m.outgoing != nil {
		m.mu.Unlock()
		return errors.New("идёт другая отправка")
	}
	sess := &sendSession{
		id:       randHex(8),
		peer:     peer,
		files:    files,
		srcPaths: srcPaths,
		total:    total,
		stop:     make(chan struct{}),
	}
	if peer.Platform == protocol.PlatformWeb {
		sess.webDecision = make(chan bool, 1)
		sess.pullDone = make(map[string]bool)
		sess.pullWait = make(chan struct{})
	}
	m.outgoing = sess
	m.mu.Unlock()

	go m.runSend(sess)
	return nil
}

// CancelOutgoing cancels the current outgoing transfer, if any.
func (m *Manager) CancelOutgoing() {
	m.mu.Lock()
	sess := m.outgoing
	m.mu.Unlock()
	if sess == nil {
		return
	}
	atomic.StoreInt32(&sess.cancel, 1)
	if sess.webDecision != nil && sess.token == "" {
		select {
		case sess.webDecision <- false:
		default:
		}
	}
	m.removePullArchive(sess)
	sess.abortHTTP()
	sess.closeConns()
}

func (m *Manager) clearOutgoing(sess *sendSession) {
	m.mu.Lock()
	if m.outgoing == sess {
		m.outgoing = nil
	}
	m.mu.Unlock()
}

func (m *Manager) runSend(sess *sendSession) {
	defer m.clearOutgoing(sess)
	peer := sess.peer

	if peer.Platform == protocol.PlatformWeb {
		m.runSendWebPull(sess)
		return
	}

	m.logf("send to %s (%s:%d): %d file(s), %d bytes", peer.Name, peer.Address, peer.ControlPort, len(sess.files), sess.total)
	m.emitState(State{Direction: DirSend, State: "waiting", Message: "Ожидание подтверждения...", Peer: peer.Name})

	ctx, cancel := context.WithCancel(context.Background())
	sess.setHTTPCancel(cancel)
	defer cancel()

	resp, status, err := m.postPrepare(ctx, peer, sess.files)
	if err != nil {
		if sess.canceled() {
			m.logf("send to %s canceled while waiting", peer.Name)
			m.emitState(State{Direction: DirSend, State: "canceled", Message: "Отменено", Peer: peer.Name})
		} else {
			m.logf("send to %s: prepare-upload failed: %v", peer.Name, err)
			m.emitState(State{Direction: DirSend, State: "failed", Message: "Не удалось связаться: " + err.Error(), Peer: peer.Name})
		}
		return
	}
	if sess.canceled() {
		m.logf("send to %s canceled", peer.Name)
		m.emitState(State{Direction: DirSend, State: "canceled", Message: "Отменено", Peer: peer.Name})
		return
	}
	m.logf("send to %s: prepare-upload returned HTTP %d (dataPort=%d)", peer.Name, status, resp.DataPort)
	switch status {
	case http.StatusOK:
	case http.StatusForbidden:
		m.emitState(State{Direction: DirSend, State: "declined", Message: "Получатель отклонил передачу", Peer: peer.Name})
		return
	case http.StatusConflict:
		m.emitState(State{Direction: DirSend, State: "failed", Message: "Получатель занят другой передачей", Peer: peer.Name})
		return
	case http.StatusRequestTimeout:
		m.emitState(State{Direction: DirSend, State: "failed", Message: "Получатель не ответил вовремя", Peer: peer.Name})
		return
	default:
		m.emitState(State{Direction: DirSend, State: "failed", Message: "Ошибка получателя (HTTP " + strconv.Itoa(status) + ")", Peer: peer.Name})
		return
	}

	srcs, err := openAll(sess.srcPaths)
	if err != nil {
		m.emitState(State{Direction: DirSend, State: "failed", Message: err.Error(), Peer: peer.Name})
		return
	}
	defer closeAll(srcs)

	chunks := buildChunks(sess.files, m.chunkSize)
	if len(chunks) == 0 { // all files empty
		m.emitProgress(Progress{Direction: DirSend, Bytes: sess.total, Total: sess.total, Streams: m.streams, Peer: peer.Name})
		m.emitState(State{Direction: DirSend, State: "completed", Message: "Отправлено", Peer: peer.Name})
		return
	}

	sess.start = time.Now()
	m.emitState(State{Direction: DirSend, State: "transferring", Peer: peer.Name})
	go m.reportLoop(DirSend, peer.Name, sess.files, sess.total, &sess.sent, sess.start, sess.stop)

	m.runSendWorkers(sess, resp, srcs, sess.files, chunks)
	close(sess.stop)

	switch {
	case sess.canceled():
		m.logf("send to %s canceled", peer.Name)
		m.emitState(State{Direction: DirSend, State: "canceled", Message: "Отменено", Peer: peer.Name})
	case sess.err != nil:
		m.logf("send to %s failed: %v", peer.Name, sess.err)
		m.emitState(State{Direction: DirSend, State: "failed", Message: sess.err.Error(), Peer: peer.Name})
	default:
		m.logf("send to %s completed: %d bytes", peer.Name, sess.total)
		m.emitProgress(Progress{Direction: DirSend, Bytes: sess.total, Total: sess.total, Streams: m.streams, Peer: peer.Name})
		m.emitState(State{Direction: DirSend, State: "completed", Message: "Готово", Peer: peer.Name})
	}
}

// runSendWorkers dispatches chunk work across parallel data streams. When a
// batch mixes small files with large ones (>= chunkSize), at least one stream is
// reserved for large-file chunks so tiny files cannot monopolize every TCP
// connection; the remaining streams prefer small chunks and help with large
// work once the small queue is drained.
func (m *Manager) runSendWorkers(sess *sendSession, resp protocol.PrepareUploadResponse, srcs []*os.File, files []protocol.FileMeta, chunks []chunk) {
	streams := m.streams
	if streams < 1 {
		streams = 1
	}
	large, small := partitionChunks(files, chunks, m.chunkSize)

	var wg sync.WaitGroup
	if len(large) == 0 || len(small) == 0 {
		ch := make(chan chunk, len(chunks))
		for _, c := range chunks {
			ch <- c
		}
		close(ch)
		for i := 0; i < streams; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m.sendWorker(sess, resp, srcs, ch)
			}()
		}
		wg.Wait()
		return
	}

	reserved := reservedLargeStreams(streams)
	m.logf("send to %s: %d large + %d small chunks, %d/%d streams reserved for large files",
		sess.peer.Name, len(large), len(small), reserved, streams)

	largeCh := make(chan chunk, len(large))
	smallCh := make(chan chunk, len(small))
	for _, c := range large {
		largeCh <- c
	}
	for _, c := range small {
		smallCh <- c
	}
	close(largeCh)
	close(smallCh)

	for i := 0; i < streams; i++ {
		wg.Add(1)
		if i < reserved {
			go func() {
				defer wg.Done()
				m.sendWorker(sess, resp, srcs, largeCh)
			}()
		} else {
			go func() {
				defer wg.Done()
				m.sendWorkerPreferSmall(sess, resp, srcs, largeCh, smallCh)
			}()
		}
	}
	wg.Wait()
}

func (m *Manager) openSendConn(sess *sendSession, resp protocol.PrepareUploadResponse) (net.Conn, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(sess.peer.Address, strconv.Itoa(resp.DataPort)))
	if err != nil {
		m.logf("send to %s: data dial failed: %v", sess.peer.Name, err)
		sess.setErr(err)
		return nil, err
	}
	sess.addConn(conn)
	if _, err := conn.Write([]byte(resp.Token)); err != nil {
		_ = conn.Close()
		sess.setErr(err)
		return nil, err
	}
	return conn, nil
}

func (m *Manager) sendWorker(sess *sendSession, resp protocol.PrepareUploadResponse, srcs []*os.File, chunkCh <-chan chunk) {
	conn, err := m.openSendConn(sess, resp)
	if err != nil {
		return
	}
	defer conn.Close()

	for c := range chunkCh {
		if sess.canceled() {
			return
		}
		if err := sendChunk(conn, c, srcs, &sess.sent); err != nil {
			if !sess.canceled() {
				sess.setErr(err)
			}
			return
		}
	}
}

func (m *Manager) sendWorkerPreferSmall(sess *sendSession, resp protocol.PrepareUploadResponse, srcs []*os.File, largeCh, smallCh <-chan chunk) {
	conn, err := m.openSendConn(sess, resp)
	if err != nil {
		return
	}
	defer conn.Close()

	for largeCh != nil || smallCh != nil {
		if sess.canceled() {
			return
		}
		// Prefer small chunks when both queues have work so metadata-heavy
		// tiny files drain without blocking this worker on large ranges.
		if smallCh != nil {
			select {
			case c, ok := <-smallCh:
				if !ok {
					smallCh = nil
					continue
				}
				if err := sendChunk(conn, c, srcs, &sess.sent); err != nil {
					if !sess.canceled() {
						sess.setErr(err)
					}
					return
				}
				continue
			default:
			}
		}
		select {
		case c, ok := <-smallCh:
			if !ok {
				smallCh = nil
				continue
			}
			if err := sendChunk(conn, c, srcs, &sess.sent); err != nil {
				if !sess.canceled() {
					sess.setErr(err)
				}
				return
			}
		case c, ok := <-largeCh:
			if !ok {
				largeCh = nil
				continue
			}
			if err := sendChunk(conn, c, srcs, &sess.sent); err != nil {
				if !sess.canceled() {
					sess.setErr(err)
				}
				return
			}
		}
	}
}

func sendChunk(conn net.Conn, c chunk, srcs []*os.File, counter *int64) error {
	var hdr [frameHeader]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(c.fileIndex))
	binary.BigEndian.PutUint64(hdr[4:12], uint64(c.offset))
	binary.BigEndian.PutUint64(hdr[12:20], uint64(c.length))
	if _, err := conn.Write(hdr[:]); err != nil {
		return err
	}
	sr := io.NewSectionReader(srcs[c.fileIndex], c.offset, c.length)
	_, err := io.Copy(&countingWriter{w: conn, counter: counter}, sr)
	return err
}

type countingWriter struct {
	w       io.Writer
	counter *int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 {
		atomic.AddInt64(cw.counter, int64(n))
	}
	return n, err
}

func (m *Manager) postPrepare(ctx context.Context, peer protocol.DeviceInfo, files []protocol.FileMeta) (protocol.PrepareUploadResponse, int, error) {
	body, _ := json.Marshal(protocol.PrepareUploadRequest{Sender: m.self(), Files: files})
	client := &http.Client{
		Timeout: acceptTimeout + 10*time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // self-signed; trust is pinned by fingerprint
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					return verifyFingerprint(rawCerts, peer.Fingerprint)
				},
			},
		},
	}
	url := "https://" + net.JoinHostPort(peer.Address, strconv.Itoa(peer.ControlPort)) + "/api/v1/prepare-upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return protocol.PrepareUploadResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return protocol.PrepareUploadResponse{}, 0, err
	}
	defer resp.Body.Close()
	var out protocol.PrepareUploadResponse
	if resp.StatusCode == http.StatusOK {
		_ = json.NewDecoder(resp.Body).Decode(&out)
	}
	return out, resp.StatusCode, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func (m *Manager) reportLoop(dir Direction, peerName string, files []protocol.FileMeta, total int64, counter *int64, start time.Time, stop <-chan struct{}) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	cum := cumulative(files)
	var lastBytes int64
	lastTime := start
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			cur := atomic.LoadInt64(counter)
			dt := now.Sub(lastTime).Seconds()
			var speed float64
			if dt > 0 {
				speed = float64(cur-lastBytes) / dt
			}
			lastBytes = cur
			lastTime = now
			var eta float64
			if speed > 0 && total > cur {
				eta = float64(total-cur) / speed
			}
			idx, name := currentFile(cum, files, cur)
			m.emitProgress(Progress{
				Direction: dir, Bytes: cur, Total: total, Speed: speed,
				ETASeconds: eta, Streams: m.streams, FileIndex: idx, FileName: name, Peer: peerName,
			})
		}
	}
}

func buildChunks(files []protocol.FileMeta, chunkSize int64) []chunk {
	var chunks []chunk
	for i, f := range files {
		for off := int64(0); off < f.Size; off += chunkSize {
			length := chunkSize
			if off+length > f.Size {
				length = f.Size - off
			}
			chunks = append(chunks, chunk{fileIndex: i, offset: off, length: length})
		}
	}
	return chunks
}

// reservedLargeStreams returns how many parallel data streams are dedicated to
// large-file chunks when a batch also contains small files. At least one stream
// is reserved; roughly 25% for wider pools, always leaving at least one flex
// stream for small files when streams > 1.
func reservedLargeStreams(streams int) int {
	if streams <= 1 {
		return 1
	}
	r := streams / 4
	if r < 1 {
		r = 1
	}
	if r >= streams {
		return streams - 1
	}
	return r
}

func partitionChunks(files []protocol.FileMeta, chunks []chunk, largeThreshold int64) (large, small []chunk) {
	for _, c := range chunks {
		if files[c.fileIndex].Size >= largeThreshold {
			large = append(large, c)
		} else {
			small = append(small, c)
		}
	}
	return large, small
}

func cumulative(files []protocol.FileMeta) []int64 {
	out := make([]int64, len(files))
	var sum int64
	for i, f := range files {
		sum += f.Size
		out[i] = sum
	}
	return out
}

func currentFile(cum []int64, files []protocol.FileMeta, sent int64) (int, string) {
	for i, end := range cum {
		if sent < end {
			name := files[i].Name
			if files[i].RelPath != "" {
				name = files[i].RelPath
			}
			return i, name
		}
	}
	if len(files) > 0 {
		i := len(files) - 1
		name := files[i].Name
		if files[i].RelPath != "" {
			name = files[i].RelPath
		}
		return i, name
	}
	return 0, ""
}

func fileMetasLite(files []protocol.FileMeta) []staging.FileMetaLite {
	out := make([]staging.FileMetaLite, len(files))
	for i, f := range files {
		out[i] = staging.FileMetaLite{Name: f.Name, RelPath: f.RelPath, Size: f.Size}
	}
	return out
}

func resolveSendFiles(items []protocol.SendItem) ([]protocol.FileMeta, []string, int64, error) {
	var files []protocol.FileMeta
	var srcPaths []string
	var total int64
	for _, it := range items {
		fi, err := os.Stat(it.Path)
		if err != nil {
			return nil, nil, 0, err
		}
		if fi.IsDir() {
			continue
		}
		rel := filepath.ToSlash(it.RelPath)
		if rel == "" {
			rel = filepath.Base(it.Path)
		}
		files = append(files, protocol.FileMeta{
			ID: randHex(8), Name: filepath.Base(rel), RelPath: rel,
			Size: fi.Size(), MIME: mimeByExt(it.Path),
		})
		srcPaths = append(srcPaths, it.Path)
		total += fi.Size()
	}
	if len(files) == 0 {
		return nil, nil, 0, errors.New("нет файлов для отправки")
	}
	return files, srcPaths, total, nil
}

func openAll(paths []string) ([]*os.File, error) {
	out := make([]*os.File, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			closeAll(out)
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func closeAll(files []*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

func sanitizeRelPath(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = sanitizeName(p)
		if p != "" && p != "." && p != ".." {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "file"
	}
	return filepath.Join(out...)
}

func uniquePathInTree(baseDir, rel string) string {
	dest := filepath.Join(baseDir, rel)
	if _, err := os.Stat(dest); err != nil {
		return dest
	}
	dir := filepath.Dir(dest)
	name := filepath.Base(dest)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

func mimeByExt(path string) string {
	return mime.TypeByExtension(filepath.Ext(path))
}

func verifyFingerprint(rawCerts [][]byte, expected string) error {
	if len(rawCerts) == 0 {
		return errors.New("no peer certificate")
	}
	if expected == "" {
		return errors.New("missing peer fingerprint")
	}
	sum := sha256.Sum256(rawCerts[0])
	if "sha256:"+hex.EncodeToString(sum[:]) != expected {
		return errors.New("fingerprint mismatch")
	}
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}
