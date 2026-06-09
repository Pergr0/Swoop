package server

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"swoop/core/rendezvous"
)

// Server is a minimal rendezvous HTTP server (signaling only, no file relay).
type Server struct {
	Store  *Store
	Addr   string
	limits *endpointLimits
}

// New creates a rendezvous server listening on addr (e.g. ":53400").
func New(addr string) *Server {
	logf := func(format string, args ...any) { log.Printf(format, args...) }
	return &Server{Store: NewStore(logf), Addr: addr, limits: newEndpointLimits()}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rendezvous/host", s.handleHost)
	mux.HandleFunc("/api/v1/rendezvous/join", s.handleJoin)
	mux.HandleFunc("/api/v1/rendezvous/poll", s.handlePoll)
	mux.HandleFunc("/api/v1/rendezvous/touch", s.handleTouch)
	mux.HandleFunc("/api/v1/overlay/connect", s.handleOverlayConnect)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// No ReadTimeout/WriteTimeout: overlay relay uses long-lived WebSocket bridges
	// for the full invite session (up to 15 minutes).
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("swoop-rendezvous listening on %s (signaling + invite-scoped overlay relay)", s.Addr)
	return srv.ListenAndServe()
}

func (s *Server) handleHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.rateLimit(w, r, s.limits.host) {
		return
	}
	var req rendezvous.HostRegisterRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&req); err != nil || req.SessionID == "" || req.PeerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	reflexive := clientIP(r)
	s.Store.RegisterHost(req.SessionID, req.PeerID, req.DeviceName, req.LanAddr, req.ReachAddr, reflexive,
		req.ControlPort, req.PunchPort, req.ReachPort)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.rateLimit(w, r, s.limits.join) {
		return
	}
	var req rendezvous.JoinRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&req); err != nil || req.SessionID == "" || req.PeerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	reflexive := clientIP(r)
	room, ok := s.Store.Join(req.SessionID, req.PeerID, req.LanAddr, reflexive,
		req.DeviceName, req.Fingerprint, req.PunchPort, req.ControlPort, req.Capabilities)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	h := room.host
	reachAddr, reachPort := h.reachAddr, h.reachPort
	if reachPort == 0 {
		reachPort = h.controlPort
	}
	out := rendezvous.HostInfo{
		PeerID:          h.peerID,
		DeviceName:      h.deviceName,
		LanAddr:         h.lanAddr,
		ReachAddr:       reachAddr,
		ReflexiveAddr:   h.reflexiveIP,
		ControlPort:     h.controlPort,
		PunchPort:       h.punchPort,
		JoinerReflexive: reflexive,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleTouch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.rateLimit(w, r, s.limits.touch) {
		return
	}
	var req rendezvous.TouchRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.Store.TouchSession(req.SessionID) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.rateLimit(w, r, s.limits.poll) {
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	hostPeerID := r.URL.Query().Get("hostPeerId")
	if sessionID == "" || hostPeerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	j, ok := s.Store.PollJoiner(sessionID, hostPeerID)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := rendezvous.JoinerInfo{
		PeerID:        j.peerID,
		DeviceName:    j.deviceName,
		LanAddr:       j.lanAddr,
		Fingerprint:   j.fingerprint,
		ControlPort:   j.controlPort,
		Capabilities:  j.capabilities,
		ReflexiveAddr: j.reflexiveIP,
		PunchPort:     j.punchPort,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if host == "::1" {
		return "127.0.0.1"
	}
	return host
}
