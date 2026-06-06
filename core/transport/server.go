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
	"strings"
	"time"

	"swoop/core/identity"
	"swoop/core/protocol"
	"swoop/core/webui"
)

// UploadHandler handles an incoming prepare-upload request. It returns the
// response body and an HTTP status code (200 accept, 403 decline, 409 busy,
// 408 timeout). Implementations may block until the user decides.
type UploadHandler interface {
	PrepareUpload(req protocol.PrepareUploadRequest, remoteAddr string) (protocol.PrepareUploadResponse, int)
}

// HTTPUploadHandler receives browser multipart uploads for an accepted session.
type HTTPUploadHandler interface {
	HandleHTTPUpload(sessionID string, r *http.Request) int
}

// HTTPPullHandler serves desktop→browser pull offers and file downloads.
type HTTPPullHandler interface {
	GetPullOffer(clientID, remoteAddr, webToken string) (protocol.PullOffer, bool)
	RespondPullOffer(sessionID, clientID, remoteAddr, webToken string, accept bool) (protocol.PullAcceptResponse, int)
	HandleHTTPDownload(sessionID, fileID string, w http.ResponseWriter, r *http.Request) int
}

// PresenceHandler records browser clients for the desktop device grid.
type PresenceHandler interface {
	Touch(req protocol.PresenceRequest, userAgent, remoteAddr string) (protocol.PresenceResponse, int)
}

// MessageHandler handles incoming chat traffic and returns an HTTP status code
// (200 accepted, 400 invalid, 403 forbidden, 429 rate-limited, 500 storage error).
// It also handles read receipts acknowledging previously sent messages.
type MessageHandler interface {
	ReceiveMessage(msg protocol.ChatMessage, remoteAddr, webToken string) int
	ReceiveRead(rr protocol.ReadReceipt, remoteAddr, webToken string) int
}

// WebChatHandler serves browser chat polling (desktop→browser delivery).
type WebChatHandler interface {
	PollWebChat(clientID, remoteAddr, webToken string, since int64) (protocol.WebChatPollResponse, int)
}

// Server exposes the control-plane HTTP(S) API for a device.
type Server struct {
	id       *identity.Identity
	self     func() protocol.DeviceInfo
	uploads      UploadHandler
	httpUploads  HTTPUploadHandler
	httpPulls    HTTPPullHandler
	presence     PresenceHandler
	messages     MessageHandler
	webChat      WebChatHandler
	http     *http.Server
	port     int
	logf     func(string, ...any)
}

// NewServer creates a control-plane server. self provides the current
// DeviceInfo at request time (the control port is only known after Start).
func NewServer(id *identity.Identity, self func() protocol.DeviceInfo, uploads UploadHandler) *Server {
	return &Server{id: id, self: self, uploads: uploads, logf: func(string, ...any) {}}
}

// SetHTTPUploadHandler installs the browser upload handler. Must be called before Start.
func (s *Server) SetHTTPUploadHandler(h HTTPUploadHandler) { s.httpUploads = h }

// SetHTTPPullHandler installs the browser pull handler. Must be called before Start.
func (s *Server) SetHTTPPullHandler(h HTTPPullHandler) { s.httpPulls = h }

// SetPresenceHandler installs the browser presence handler. Must be called before Start.
func (s *Server) SetPresenceHandler(h PresenceHandler) { s.presence = h }

// SetMessageHandler installs the chat message handler. Must be called before Start.
func (s *Server) SetMessageHandler(h MessageHandler) { s.messages = h }

// SetWebChatHandler installs the browser chat poll handler. Must be called before Start.
func (s *Server) SetWebChatHandler(h WebChatHandler) { s.webChat = h }

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
	mux.HandleFunc("/api/v1/presence", s.recover(s.handlePresence))
	mux.HandleFunc("/api/v1/upload/", s.recover(s.handleHTTPUpload))
	mux.HandleFunc("/api/v1/pull-offer", s.recover(s.handlePullOffer))
	mux.HandleFunc("/api/v1/pull-offer/", s.recover(s.handlePullRespond))
	mux.HandleFunc("/api/v1/download/", s.recover(s.handleHTTPDownload))
	mux.HandleFunc("/api/v1/message", s.recover(s.handleMessage))
	mux.HandleFunc("/api/v1/read", s.recover(s.handleRead))
	mux.HandleFunc("/api/v1/chat", s.recover(s.handleChatPoll))
	mux.Handle("/", webui.Handler())

	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       120 * time.Second,
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

func (s *Server) handlePresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.presence == nil {
		http.Error(w, "presence not supported", http.StatusServiceUnavailable)
		return
	}
	var req protocol.PresenceRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, status := s.presence.Touch(req, r.Header.Get("User-Agent"), r.RemoteAddr)
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handlePullOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.httpPulls == nil {
		http.Error(w, "pull not supported", http.StatusServiceUnavailable)
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	offer, ok := s.httpPulls.GetPullOffer(clientID, r.RemoteAddr, r.Header.Get("X-Swoop-Web-Token"))
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(offer)
}

func (s *Server) handlePullRespond(w http.ResponseWriter, r *http.Request) {
	if s.httpPulls == nil {
		http.Error(w, "pull not supported", http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/pull-offer/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "respond" || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	sessionID := parts[0]
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ClientID string `json:"clientId"`
		Accept   bool   `json:"accept"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.ClientID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, status := s.httpPulls.RespondPullOffer(sessionID, body.ClientID, r.RemoteAddr, r.Header.Get("X-Swoop-Web-Token"), body.Accept)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusOK {
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (s *Server) handleHTTPDownload(w http.ResponseWriter, r *http.Request) {
	if s.httpPulls == nil {
		http.Error(w, "pull not supported", http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	status := s.httpPulls.HandleHTTPDownload(parts[0], parts[1], w, r)
	if status != http.StatusOK {
		if status == http.StatusMethodNotAllowed {
			http.Error(w, "method not allowed", status)
		} else {
			http.Error(w, http.StatusText(status), status)
		}
	}
}

func (s *Server) handleHTTPUpload(w http.ResponseWriter, r *http.Request) {
	if s.httpUploads == nil {
		http.Error(w, "uploads not supported", http.StatusServiceUnavailable)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/upload/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		http.NotFound(w, r)
		return
	}
	status := s.httpUploads.HandleHTTPUpload(sessionID, r)
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
	}
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
	dec := json.NewDecoder(io.LimitReader(r.Body, protocol.MaxPrepareUploadBodyBytes))
	if err := dec.Decode(&req); err != nil {
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
	status := s.messages.ReceiveMessage(msg, r.RemoteAddr, r.Header.Get("X-Swoop-Web-Token"))
	w.WriteHeader(status)
}

func (s *Server) handleChatPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.webChat == nil {
		http.Error(w, "chat not supported", http.StatusServiceUnavailable)
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	resp, status := s.webChat.PollWebChat(clientID, r.RemoteAddr, r.Header.Get("X-Swoop-Web-Token"), since)
	if status == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
	status := s.messages.ReceiveRead(rr, r.RemoteAddr, r.Header.Get("X-Swoop-Web-Token"))
	w.WriteHeader(status)
}
