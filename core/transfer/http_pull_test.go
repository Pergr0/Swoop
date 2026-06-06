package transfer

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"swoop/core/protocol"
)

func TestWebPullOfferAndDownload(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(fpath, []byte("hello pull"), 0o644); err != nil {
		t.Fatal(err)
	}

	self := func() protocol.DeviceInfo {
		return protocol.DeviceInfo{ID: "desktop", Name: "PC", Platform: protocol.PlatformLinux}
	}
	m := NewManager(self, dir)
	m.SetWebVerifier(func(_, _, _ string) bool { return true })

	peer := protocol.DeviceInfo{ID: "web-1", Name: "Phone", Platform: protocol.PlatformWeb}
	items := []protocol.SendItem{{Path: fpath, RelPath: ""}}
	if err := m.Send(peer, items); err != nil {
		t.Fatal(err)
	}

	offer, ok := m.GetPullOffer("web-1", "127.0.0.1:1234", "test")
	if !ok || offer.SessionID == "" || offer.Count != 1 {
		t.Fatalf("offer=%+v ok=%v", offer, ok)
	}

	resp, status := m.RespondPullOffer(offer.SessionID, "web-1", "127.0.0.1:1234", "test", true)
	if status != http.StatusOK || resp.Mode != protocol.TransferHTTPPull || len(resp.Files) != 1 {
		t.Fatalf("status=%d resp=%+v", status, resp)
	}

	req := httptest.NewRequest(http.MethodGet, resp.Files[0].DownloadPath, nil)
	req.Header.Set(uploadTokenHeader, resp.Token)
	req.Header.Set(webPresenceHeader, "test")
	rr := httptest.NewRecorder()
	if st := m.HandleHTTPDownload(offer.SessionID, resp.Files[0].ID, rr, req); st != http.StatusOK {
		t.Fatalf("download status=%d body=%q", st, rr.Body.String())
	}
	if rr.Body.String() != "hello pull" {
		t.Fatalf("body=%q", rr.Body.String())
	}

	waitSendDone(t, m)
}

func TestWebPullArchive(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(root, "a.txt")
	f2 := filepath.Join(root, "sub", "b.txt")
	_ = os.WriteFile(f1, []byte("aa"), 0o644)
	_ = os.WriteFile(f2, []byte("bbb"), 0o644)

	self := func() protocol.DeviceInfo {
		return protocol.DeviceInfo{ID: "desktop", Name: "PC", Platform: protocol.PlatformLinux}
	}
	m := NewManager(self, dir)
	m.SetWebVerifier(func(_, _, _ string) bool { return true })
	peer := protocol.DeviceInfo{ID: "web-1", Name: "Phone", Platform: protocol.PlatformWeb}
	items := []protocol.SendItem{
		{Path: f1, RelPath: "project/a.txt"},
		{Path: f2, RelPath: "project/sub/b.txt"},
	}
	if err := m.Send(peer, items); err != nil {
		t.Fatal(err)
	}

	offer, ok := m.GetPullOffer("web-1", "127.0.0.1:1234", "test")
	if !ok {
		t.Fatal("no offer")
	}
	resp, status := m.RespondPullOffer(offer.SessionID, "web-1", "127.0.0.1:1234", "test", true)
	if status != http.StatusOK || resp.ArchivePath == "" || resp.ArchiveName != "project.zip" || resp.ArchiveSize < 32 {
		t.Fatalf("status=%d resp=%+v", status, resp)
	}

	req := httptest.NewRequest(http.MethodGet, resp.ArchivePath, nil)
	req.Header.Set(uploadTokenHeader, resp.Token)
	req.Header.Set(webPresenceHeader, "test")
	rr := httptest.NewRecorder()
	if st := m.HandleHTTPDownload(offer.SessionID, pullArchiveID, rr, req); st != http.StatusOK {
		t.Fatalf("archive status=%d", st)
	}
	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("zip entries=%d", len(zr.File))
	}

	waitSendDone(t, m)
}

func waitSendDone(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		m.mu.Lock()
		busy := m.outgoing != nil
		m.mu.Unlock()
		if !busy {
			return
		}
		select {
		case <-deadline:
			t.Fatal("send session did not finish")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
