package server

import (
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var overlayUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleOverlayConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.rateLimit(w, r, s.limits.overlay) {
		return
	}
	q := r.URL.Query()
	sessionID := q.Get("sessionId")
	role := q.Get("role")
	peerID := q.Get("peerId")
	if sessionID == "" || (role != "host" && role != "joiner") || peerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.Store.AllowOverlay(sessionID, role, peerID) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	ws, err := overlayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// Block until the relay closes this socket. Returning from the handler
	// would make net/http tear down the WebSocket immediately.
	done := s.Store.relay.attach(sessionID, role, ws)
	<-done
}

type relayBridge struct {
	mu    sync.Mutex
	rooms map[string]*relayRoom
	logf  func(string, ...any)
}

type relayRoom struct {
	host       *websocket.Conn
	joiner     *websocket.Conn
	hostDone   chan struct{}
	joinerDone chan struct{}
}

func newRelayBridge(logf func(string, ...any)) *relayBridge {
	return &relayBridge{rooms: make(map[string]*relayRoom), logf: logf}
}

func (b *relayBridge) attach(sessionID, role string, ws *websocket.Conn) <-chan struct{} {
	done := make(chan struct{})
	b.mu.Lock()
	room := b.rooms[sessionID]
	if room == nil {
		room = &relayRoom{}
		b.rooms[sessionID] = room
	}
	switch role {
	case "host":
		if room.host != nil {
			b.signalDone(room.hostDone)
			_ = room.host.Close()
		}
		room.host = ws
		room.hostDone = done
		if b.logf != nil {
			b.logf("event=overlay_attach session=%s role=host", sessionID)
		}
	case "joiner":
		if room.joiner != nil {
			b.signalDone(room.joinerDone)
			_ = room.joiner.Close()
		}
		room.joiner = ws
		room.joinerDone = done
		if b.logf != nil {
			b.logf("event=overlay_attach session=%s role=joiner", sessionID)
		}
	}
	startBridge := room.host != nil && room.joiner != nil
	host, joiner := room.host, room.joiner
	b.mu.Unlock()

	if startBridge {
		if b.logf != nil {
			b.logf("event=overlay_bridge session=%s", sessionID)
		}
		go b.bridge(sessionID, host, joiner)
	}
	return done
}

func (b *relayBridge) signalDone(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func (b *relayBridge) bridge(sessionID string, host, joiner *websocket.Conn) {
	defer b.cleanup(sessionID)
	errc := make(chan error, 2)
	go func() { errc <- copyWS(host, joiner) }()
	go func() { errc <- copyWS(joiner, host) }()
	<-errc
}

func copyWS(dst, src *websocket.Conn) error {
	for {
		msgType, r, err := src.NextReader()
		if err != nil {
			return err
		}
		w, err := dst.NextWriter(msgType)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			_ = w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	}
}

func (b *relayBridge) cleanup(sessionID string) {
	b.mu.Lock()
	room := b.rooms[sessionID]
	delete(b.rooms, sessionID)
	b.mu.Unlock()
	if room == nil {
		return
	}
	if room.host != nil {
		_ = room.host.Close()
	}
	if room.joiner != nil {
		_ = room.joiner.Close()
	}
	b.signalDone(room.hostDone)
	b.signalDone(room.joinerDone)
}

func (b *relayBridge) dropSession(sessionID string) {
	b.cleanup(sessionID)
}

// AllowOverlay checks the invite session exists and the peer role is valid.
func (s *Store) AllowOverlay(sessionID, role, peerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	r, ok := s.rooms[sessionID]
	if !ok {
		return false
	}
	switch role {
	case "host":
		return r.host.peerID == peerID
	case "joiner":
		if r.joiner == nil {
			return false
		}
		return r.joiner.peerID == peerID
	default:
		return false
	}
}
