package transfer

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"swoop/core/protocol"
)

func TestHTTPUploadReceive(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(func() protocol.DeviceInfo { return protocol.DeviceInfo{ID: "self", Name: "self"} }, dir)

	req := protocol.PrepareUploadRequest{
		Sender: protocol.DeviceInfo{ID: "web-1", Name: "Phone", Platform: protocol.PlatformWeb},
		Files: []protocol.FileMeta{
			{ID: "a", Name: "one.txt", Size: 5},
			{ID: "b", Name: "two.txt", Size: 3},
		},
	}

	done := make(chan protocol.PrepareUploadResponse, 1)
	go func() {
		resp, status := m.PrepareUpload(req, "127.0.0.1:1")
		if status != http.StatusOK {
			t.Errorf("prepare status=%d", status)
		}
		done <- resp
	}()

	time.Sleep(20 * time.Millisecond)
	m.RespondIncoming(true)

	resp := <-done
	if resp.Mode != protocol.TransferHTTPUpload || resp.UploadPath == "" || resp.Token == "" {
		t.Fatalf("resp=%+v", resp)
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for _, payload := range []string{"hello", "bye"} {
		part, err := w.CreateFormFile("file", "x")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	httpReq := httptest.NewRequest(http.MethodPost, resp.UploadPath, body)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set(uploadTokenHeader, resp.Token)

	status := m.HandleHTTPUpload(resp.SessionID, httpReq)
	if status != http.StatusOK {
		t.Fatalf("upload status=%d", status)
	}

	if b, err := os.ReadFile(filepath.Join(dir, "one.txt")); err != nil || string(b) != "hello" {
		t.Fatalf("one.txt=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "two.txt")); err != nil || string(b) != "bye" {
		t.Fatalf("two.txt=%q err=%v", b, err)
	}
}

func TestHTTPUploadBadToken(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(func() protocol.DeviceInfo { return protocol.DeviceInfo{ID: "self"} }, dir)
	m.mu.Lock()
	m.incoming = &recvSession{
		id: "sess1", mode: protocol.TransferHTTPUpload, token: "secret",
		files: []protocol.FileMeta{{Name: "a", Size: 0}}, decision: make(chan bool, 1),
		done: make(chan struct{}),
	}
	m.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/sess1", nil)
	req.Header.Set(uploadTokenHeader, "wrong")
	if st := m.HandleHTTPUpload("sess1", req); st != http.StatusForbidden {
		t.Fatalf("status=%d", st)
	}
}

// silence unused import in case of build tags
var _ = io.Discard
