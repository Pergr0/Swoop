package transfer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"swoop/core/protocol"
)

const uploadTokenHeader = "X-Swoop-Token"

// HandleHTTPUpload receives a browser multipart upload for an accepted session.
func (m *Manager) HandleHTTPUpload(sessionID string, r *http.Request) int {
	if r.Method != http.MethodPost {
		return http.StatusMethodNotAllowed
	}

	m.mu.Lock()
	sess := m.incoming
	if sess == nil || sess.id != sessionID || sess.mode != protocol.TransferHTTPUpload {
		m.mu.Unlock()
		return http.StatusNotFound
	}
	if sess.token == "" || r.Header.Get(uploadTokenHeader) != sess.token {
		m.mu.Unlock()
		return http.StatusForbidden
	}
	if sess.canceled() {
		m.mu.Unlock()
		return http.StatusConflict
	}
	total := sess.total
	fileCount := len(sess.files)
	m.mu.Unlock()

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		return http.StatusBadRequest
	}

	r.Body = http.MaxBytesReader(nil, r.Body, total+8<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		return http.StatusBadRequest
	}

	fileIndex := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			m.failHTTPUpload(sess, err)
			return http.StatusBadRequest
		}
		if sess.canceled() {
			return http.StatusConflict
		}

		if fileIndex >= fileCount {
			m.failHTTPUpload(sess, fmt.Errorf("слишком много файлов в загрузке"))
			return http.StatusBadRequest
		}

		lim := sess.files[fileIndex].Size
		n, err := copyLimited(sess.handles[fileIndex], part, lim, sess)
		_ = part.Close()
		if err != nil {
			m.failHTTPUpload(sess, err)
			return http.StatusInternalServerError
		}
		if n != lim {
			m.failHTTPUpload(sess, fmt.Errorf("файл %q: получено %d байт, ожидалось %d", sess.files[fileIndex].Name, n, lim))
			return http.StatusBadRequest
		}
		fileIndex++
	}

	if fileIndex != fileCount {
		m.failHTTPUpload(sess, fmt.Errorf("получено %d файлов, ожидалось %d", fileIndex, fileCount))
		return http.StatusBadRequest
	}

	sess.finalizeRecv(m, true)
	return http.StatusOK
}

func (m *Manager) failHTTPUpload(sess *recvSession, err error) {
	sess.setErr(err)
	sess.finalizeRecv(m, false)
}

func copyLimited(dst *os.File, src io.Reader, limit int64, sess *recvSession) (int64, error) {
	buf := make([]byte, 256*1024)
	limited := io.LimitReader(src, limit)
	var written int64
	for {
		n, err := limited.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return written, werr
			}
			written += int64(n)
			sess.touch()
			atomic.AddInt64(&sess.received, int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}
	// Drain extra part bytes so the multipart reader stays aligned.
	_, _ = io.Copy(io.Discard, src)
	return written, nil
}
