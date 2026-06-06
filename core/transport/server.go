// Package transport implements the control plane: a small HTTPS API used for
// discovery metadata, the upload handshake and trust. The high-throughput
// file bytes travel on a separate data plane (see package transfer).
package transport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"swoop/core/identity"
	"swoop/core/protocol"
)

// UploadHandler handles an incoming prepare-upload request. It returns the
// response body and an HTTP status code (200 accept, 403 decline, 409 busy,
// 408 timeout). Implementations may block until the user decides.
type UploadHandler interface {
	PrepareUpload(req protocol.PrepareUploadRequest, remoteAddr string) (protocol.PrepareUploadResponse, int)
}

// MessageHandler handles incoming chat traffic and returns an HTTP status code
// (200 accepted, 400 invalid, 429 rate-limited, 500 storage error). It also
// handles read receipts acknowledging previously sent messages.
type MessageHandler interface {
	ReceiveMessage(msg protocol.ChatMessage, remoteAddr string) int
	ReceiveRead(rr protocol.ReadReceipt, remoteAddr string) int
}

// Server exposes the control-plane HTTP(S) API for a device.
type Server struct {
	id       *identity.Identity
	self     func() protocol.DeviceInfo
	uploads  UploadHandler
	messages MessageHandler
	http     *http.Server
	port     int
	logf     func(string, ...any)
}

// NewServer creates a control-plane server. self provides the current
// DeviceInfo at request time (the control port is only known after Start).
func NewServer(id *identity.Identity, self func() protocol.DeviceInfo, uploads UploadHandler) *Server {
	return &Server{id: id, self: self, uploads: uploads, logf: func(string, ...any) {}}
}

// SetMessageHandler installs the chat message handler. Must be called before Start.
func (s *Server) SetMessageHandler(h MessageHandler) { s.messages = h }

// SetLogf installs a logging function. Must be called before Start.
func (s *Server) SetLogf(fn func(string, ...any)) {
	if fn != nil {
		s.logf = fn
	}
}

// logfWriter adapts the server's logf to an io.Writer so net/http internal
// errors (notably TLS handshake failures) become visible.
type logfWriter struct{ logf func(string, ...any) }

func (w logfWriter) Write(p []byte) (int, error) {
	w.logf("http: %s", string(p))
	return len(p), nil
}

// Start binds the control port and serves over TLS until ctx is cancelled.
// Passing port 0 lets the OS choose a free port.
func (s *Server) Start(ctx context.Context, port int) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	s.port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/info", s.recover(s.handleInfo))
	mux.HandleFunc("/api/v1/prepare-upload", s.recover(s.handlePrepareUpload))
	mux.HandleFunc("/api/v1/message", s.recover(s.handleMessage))
	mux.HandleFunc("/api/v1/read", s.recover(s.handleRead))

	s.http = &http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{s.id.Certificate},
			MinVersion:   tls.VersionTLS12,
		},
		ErrorLog: log.New(logfWriter{s.logf}, "", 0),
	}

	go func() {
		<-ctx.Done()
		_ = s.http.Close()
	}()
	go func() {
		// Certs come from TLSConfig, so the file arguments are empty.
		if err := s.http.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			s.logf("control server stopped: %v", err)
		}
	}()
	return nil
}

// recover wraps a handler so a panic is logged and turned into a 500 instead of
// abruptly closing the connection (which the peer would see as a bare EOF).
func (s *Server) recover(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logf("panic handling %s from %s: %v", r.URL.Path, r.RemoteAddr, rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		h(w, r)
	}
}

// Port returns the bound control-plane port.
func (s *Server) Port() int { return s.port }

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.self())
}

func (s *Server) handlePrepareUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.uploads == nil {
		http.Error(w, "uploads not supported", http.StatusServiceUnavailable)
		return
	}
	var req protocol.PrepareUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logf("prepare-upload: bad body from %s: %v", r.RemoteAddr, err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.logf("prepare-upload from %s (%s): %d file(s)", req.Sender.Name, r.RemoteAddr, len(req.Files))
	resp, status := s.uploads.PrepareUpload(req, r.RemoteAddr)
	s.logf("prepare-upload from %s resolved with HTTP %d", req.Sender.Name, status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusOK {
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.messages == nil {
		http.Error(w, "messages not supported", http.StatusServiceUnavailable)
		return
	}
	// Hard byte cap before decoding so an oversized body can't exhaust memory.
	var msg protocol.ChatMessage
	dec := json.NewDecoder(io.LimitReader(r.Body, protocol.MaxMessageBytes*4+1024))
	if err := dec.Decode(&msg); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	status := s.messages.ReceiveMessage(msg, r.RemoteAddr)
	w.WriteHeader(status)
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.messages == nil {
		http.Error(w, "messages not supported", http.StatusServiceUnavailable)
		return
	}
	var rr protocol.ReadReceipt
	dec := json.NewDecoder(io.LimitReader(r.Body, 8192))
	if err := dec.Decode(&rr); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	status := s.messages.ReceiveRead(rr, r.RemoteAddr)
	w.WriteHeader(status)
}
