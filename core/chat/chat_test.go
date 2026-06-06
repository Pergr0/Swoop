package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.tmp")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Adversarial content: newlines, quotes, fake SQL/shell, HTML. Must round-trip
	// verbatim and never corrupt the line-delimited file.
	tricky := "line1\nline2 \"q\" '; DROP TABLE users;-- `rm -rf /` <script>x</script>\t end"
	msgs := []Message{
		{Ts: 1, PeerID: "p1", PeerName: "A", Dir: "in", Text: tricky},
		{Ts: 2, PeerID: "p2", PeerName: "B", Dir: "out", Text: "other peer"},
		{Ts: 3, PeerID: "p1", PeerName: "A", Dir: "out", Text: "second to p1"},
	}
	for _, m := range msgs {
		if err := s.Append(m); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	hist, err := s.History("p1", 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 messages for p1, got %d", len(hist))
	}
	if hist[0].Text != tricky {
		t.Fatalf("tricky text not preserved: %q", hist[0].Text)
	}
	if hist[1].Text != "second to p1" {
		t.Fatalf("order/content wrong: %q", hist[1].Text)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("chat file should be removed on close, stat err=%v", err)
	}
}

func TestHistoryLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.tmp")
	s, _ := NewStore(path)
	defer s.Close()
	for i := 0; i < 10; i++ {
		_ = s.Append(Message{Ts: int64(i), PeerID: "p", Dir: "in", Text: "m"})
	}
	hist, _ := s.History("p", 3)
	if len(hist) != 3 {
		t.Fatalf("limit not applied: got %d", len(hist))
	}
	if hist[0].Ts != 7 || hist[2].Ts != 9 {
		t.Fatalf("expected most-recent window 7..9, got %d..%d", hist[0].Ts, hist[2].Ts)
	}
}
