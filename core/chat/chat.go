// Package chat persists short text messages to a temporary, line-delimited JSON
// file next to the binary. It deliberately keeps no message history in memory:
// the file is the source of truth, and it is removed when the app shuts down.
//
// Safety: message text is stored as a JSON string, so any content (newlines,
// quotes, control bytes, "SQL", shell metacharacters, etc.) is escaped on write
// and decoded as inert text on read. Nothing in this package interprets or
// executes message content; it is opaque bytes throughout.
package chat

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// maxFileBytes bounds the on-disk chat log so a flood of messages cannot fill
// the disk. Once exceeded, new messages are refused until shutdown clears it.
const maxFileBytes = 32 << 20 // 32 MiB

// Message is a single stored chat entry.
type Message struct {
	Ts       int64  `json:"ts"`       // unix milliseconds (sender's send time)
	PeerID   string `json:"peerId"`   // the other device's id
	PeerName string `json:"peerName"` // display name at the time
	Dir      string `json:"dir"`      // "in" | "out"
	Text     string `json:"text"`     // opaque UTF-8 text
	// Read is meaningful only for outgoing messages: true once the peer has
	// acknowledged reading it. It is computed on read (not persisted), since
	// read state lives only for the app session.
	Read bool `json:"read,omitempty"`
}

// Store is an append-only, file-backed message log.
type Store struct {
	mu      sync.Mutex
	path    string
	f       *os.File
	written int64
}

// NewStore creates (truncating any stale file) the chat log at path.
func NewStore(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, f: f}, nil
}

// Path returns the backing file path.
func (s *Store) Path() string { return s.path }

// Append writes one message as a JSON line. It is safe for arbitrary text.
func (s *Store) Append(m Message) error {
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return errors.New("chat store closed")
	}
	if s.written+int64(len(line)) > maxFileBytes {
		return errors.New("chat log full")
	}
	n, err := s.f.Write(line)
	s.written += int64(n)
	return err
}

// History returns up to limit most-recent messages for peerID, oldest first.
// It reads from the file so no history is retained in memory.
func (s *Store) History(peerID string, limit int) ([]Message, error) {
	s.mu.Lock()
	path := s.path
	s.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Message
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var m Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue // skip any partial/corrupt line
		}
		if m.PeerID != peerID {
			continue
		}
		all = append(all, m)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

// Close closes and removes the backing file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	_ = s.f.Close()
	s.f = nil
	return os.Remove(s.path)
}
