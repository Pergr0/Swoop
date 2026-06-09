package server

import (
	"sync"
	"time"
)

const defaultTTL = 15 * time.Minute

type hostRecord struct {
	peerID      string
	deviceName  string
	lanAddr     string
	controlPort int
	punchPort   int
	reachAddr   string
	reachPort   int
	reflexiveIP string
	expiresAt   time.Time
}

type joinerRecord struct {
	peerID      string
	punchPort   int
	lanAddr     string
	reflexiveIP string
	joinedAt    time.Time
	seenByHost  bool
}

type room struct {
	sessionID string
	host      hostRecord
	joiner    *joinerRecord
}

// Store holds in-memory rendezvous rooms (no accounts).
type Store struct {
	mu    sync.Mutex
	rooms map[string]*room
	relay *relayBridge
	logf  func(string, ...any)
}

// NewStore creates an empty session store.
func NewStore(logf func(string, ...any)) *Store {
	return &Store{rooms: make(map[string]*room), relay: newRelayBridge(logf), logf: logf}
}

func (s *Store) RegisterHost(sessionID, peerID, deviceName, lanAddr, reachAddr, reflexiveIP string, controlPort, punchPort, reachPort int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	s.rooms[sessionID] = &room{
		sessionID: sessionID,
		host: hostRecord{
			peerID: peerID, deviceName: deviceName, lanAddr: lanAddr,
			controlPort: controlPort, punchPort: punchPort,
			reachAddr: reachAddr, reachPort: reachPort,
			reflexiveIP: reflexiveIP,
			expiresAt:   time.Now().Add(defaultTTL),
		},
	}
	if s.logf != nil {
		s.logf("event=host_register session=%s peer=%s reflexive=%s punch=%d reach=%s control=%d",
			sessionID, peerID, reflexiveIP, punchPort, reachAddr, controlPort)
	}
}

func (s *Store) Join(sessionID, peerID, lanAddr, reflexiveIP string, punchPort int) (*room, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	r, ok := s.rooms[sessionID]
	if !ok || time.Now().After(r.host.expiresAt) {
		return nil, false
	}
	r.joiner = &joinerRecord{
		peerID: peerID, punchPort: punchPort, lanAddr: lanAddr,
		reflexiveIP: reflexiveIP, joinedAt: time.Now(),
	}
	if s.logf != nil {
		s.logf("event=join session=%s host=%s joiner=%s joiner_reflexive=%s joiner_punch=%d",
			sessionID, r.host.peerID, peerID, reflexiveIP, punchPort)
	}
	return r, true
}

func (s *Store) PollJoiner(sessionID, hostPeerID string) (*joinerRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rooms[sessionID]
	if !ok || r.host.peerID != hostPeerID || r.joiner == nil {
		return nil, false
	}
	if r.joiner.seenByHost {
		return nil, false
	}
	r.joiner.seenByHost = true
	if s.logf != nil {
		s.logf("event=poll_deliver session=%s joiner=%s", sessionID, r.joiner.peerID)
	}
	j := *r.joiner
	return &j, true
}

func (s *Store) reapLocked() {
	now := time.Now()
	for id, r := range s.rooms {
		if now.After(r.host.expiresAt) {
			if s.relay != nil {
				s.relay.dropSession(id)
			}
			delete(s.rooms, id)
		}
	}
}
