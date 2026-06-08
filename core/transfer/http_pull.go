package transfer



import (

	"archive/zip"

	"context"

	"fmt"

	"io"

	"net/http"

	"os"

	"path/filepath"

	"strconv"

	"strings"

	"sync/atomic"

	"time"



	"swoop/core/protocol"

)



const (
	pullArchiveID      = "archive"
	webPresenceHeader  = "X-Swoop-Web-Token"
)

func (m *Manager) verifyWebClient(clientID, remoteAddr, webToken string) bool {
	if m.webVerify == nil {
		return false
	}
	return m.webVerify(clientID, remoteAddr, webToken)
}



// GetPullOffer returns a pending desktop→browser offer for clientID, if any.

func (m *Manager) GetPullOffer(clientID, remoteAddr, webToken string) (protocol.PullOffer, bool) {
	if !m.verifyWebClient(clientID, remoteAddr, webToken) {
		return protocol.PullOffer{}, false
	}

	m.mu.Lock()

	sess := m.outgoing

	m.mu.Unlock()

	if sess == nil || sess.peer.Platform != protocol.PlatformWeb || sess.peer.ID != clientID {

		return protocol.PullOffer{}, false

	}

	if sess.token != "" {

		return protocol.PullOffer{}, false

	}

	return protocol.PullOffer{

		SessionID: sess.id,

		Sender:    m.self(),

		Files:     sess.files,

		TotalSize: sess.total,

		Count:     len(sess.files),

	}, true

}



// RespondPullOffer lets the browser accept or decline a pending pull offer.

func (m *Manager) RespondPullOffer(sessionID, clientID, remoteAddr, webToken string, accept bool) (protocol.PullAcceptResponse, int) {
	if !m.verifyWebClient(clientID, remoteAddr, webToken) {
		return protocol.PullAcceptResponse{}, http.StatusForbidden
	}

	m.mu.Lock()

	sess := m.outgoing

	if sess == nil || sess.id != sessionID || sess.peer.Platform != protocol.PlatformWeb || sess.peer.ID != clientID {

		m.mu.Unlock()

		return protocol.PullAcceptResponse{}, http.StatusNotFound

	}

	if sess.token != "" {

		m.mu.Unlock()

		return protocol.PullAcceptResponse{}, http.StatusConflict

	}

	if sess.canceled() {

		m.mu.Unlock()

		return protocol.PullAcceptResponse{}, http.StatusConflict

	}

	decision := sess.webDecision

	m.mu.Unlock()



	if decision == nil {

		return protocol.PullAcceptResponse{}, http.StatusConflict

	}



	if !accept {

		select {

		case decision <- false:

		default:

			return protocol.PullAcceptResponse{}, http.StatusConflict

		}

		return protocol.PullAcceptResponse{}, http.StatusForbidden

	}



	if needsPullArchive(sess.files) {

		if err := m.buildPullArchiveFile(sess); err != nil {

			m.logf("web pull archive build failed: %v", err)

			return protocol.PullAcceptResponse{}, http.StatusInternalServerError

		}

	}



	m.mu.Lock()

	sess.token = randHex(16)

	resp := m.buildPullAccept(sess)

	m.mu.Unlock()



	select {

	case decision <- true:

	default:

		m.removePullArchive(sess)

		return protocol.PullAcceptResponse{}, http.StatusConflict

	}

	return resp, http.StatusOK

}



func (m *Manager) buildPullAccept(sess *sendSession) protocol.PullAcceptResponse {

	resp := protocol.PullAcceptResponse{

		SessionID: sess.id,

		Mode:      protocol.TransferHTTPPull,

		Token:     sess.token,

	}

	if needsPullArchive(sess.files) {

		resp.ArchivePath = "/api/v1/download/" + sess.id + "/" + pullArchiveID

		resp.ArchiveName = pullArchiveName(sess.files)

		if sess.archiveTemp != "" {

			if st, err := os.Stat(sess.archiveTemp); err == nil {

				resp.ArchiveSize = st.Size()

			}

		}

		return resp

	}

	files := make([]protocol.DownloadFile, len(sess.files))

	for i, f := range sess.files {

		files[i] = protocol.DownloadFile{

			ID:           f.ID,

			Name:         f.Name,

			RelPath:      f.RelPath,

			Size:         f.Size,

			DownloadPath: "/api/v1/download/" + sess.id + "/" + f.ID,

		}

	}

	resp.Files = files

	return resp

}



func needsPullArchive(files []protocol.FileMeta) bool {

	if len(files) > 1 {

		return true

	}

	if len(files) == 1 {

		rel := filepath.ToSlash(files[0].RelPath)

		if strings.Contains(rel, "/") {

			return true

		}

		if rel != "" && rel != files[0].Name {

			return true

		}

	}

	return false

}



func pullArchiveName(files []protocol.FileMeta) string {

	for _, f := range files {

		rel := filepath.ToSlash(f.RelPath)

		if i := strings.Index(rel, "/"); i > 0 {

			return sanitizeName(rel[:i]) + ".zip"

		}

	}

	return "Swoop-files.zip"

}



func zipEntryName(f protocol.FileMeta) string {

	if f.RelPath != "" {

		return sanitizeRelPath(f.RelPath)

	}

	return sanitizeName(f.Name)

}



func (m *Manager) buildPullArchiveFile(sess *sendSession) error {

	m.removePullArchive(sess)



	f, err := os.CreateTemp("", "swoop-pull-*.zip")

	if err != nil {

		return err

	}

	path := f.Name()

	zw := zip.NewWriter(f)

	for i, meta := range sess.files {

		entry, err := zw.Create(zipEntryName(meta))

		if err != nil {

			_ = zw.Close()

			_ = f.Close()

			_ = os.Remove(path)

			return err

		}

		src, err := os.Open(sess.srcPaths[i])

		if err != nil {

			_ = zw.Close()

			_ = f.Close()

			_ = os.Remove(path)

			return err

		}

		_, err = io.Copy(entry, src)

		_ = src.Close()

		if err != nil {

			_ = zw.Close()

			_ = f.Close()

			_ = os.Remove(path)

			return err

		}

	}

	if err := zw.Close(); err != nil {

		_ = f.Close()

		_ = os.Remove(path)

		return err

	}

	if err := f.Close(); err != nil {

		_ = os.Remove(path)

		return err

	}

	sess.archiveTemp = path

	m.logf("web pull archive ready: %s (%d file(s))", path, len(sess.files))

	return nil

}



func (m *Manager) removePullArchive(sess *sendSession) {

	if sess == nil || sess.archiveTemp == "" {

		return

	}

	_ = os.Remove(sess.archiveTemp)

	sess.archiveTemp = ""

}



// HandleHTTPDownload serves one file from an accepted pull session.

func (m *Manager) HandleHTTPDownload(sessionID, fileID string, w http.ResponseWriter, r *http.Request) int {

	if r.Method != http.MethodGet {

		return http.StatusMethodNotAllowed

	}



	m.mu.Lock()

	sess := m.outgoing

	if sess == nil || sess.id != sessionID || sess.peer.Platform != protocol.PlatformWeb {

		m.mu.Unlock()

		return http.StatusNotFound

	}

	if sess.token == "" || r.Header.Get(uploadTokenHeader) != sess.token {

		m.mu.Unlock()

		return http.StatusForbidden

	}

	if !m.verifyWebClient(sess.peer.ID, r.RemoteAddr, r.Header.Get(webPresenceHeader)) {

		m.mu.Unlock()

		return http.StatusForbidden

	}

	if sess.canceled() {

		m.mu.Unlock()

		return http.StatusConflict

	}



	if fileID == pullArchiveID {

		archivePath := sess.archiveTemp

		m.mu.Unlock()

		return m.servePullArchive(sess, archivePath, w)

	}



	idx := -1

	for i, f := range sess.files {

		if f.ID == fileID {

			idx = i

			break

		}

	}

	if idx < 0 || idx >= len(sess.srcPaths) {

		m.mu.Unlock()

		return http.StatusNotFound

	}

	path := sess.srcPaths[idx]

	meta := sess.files[idx]

	m.mu.Unlock()



	f, err := os.Open(path)

	if err != nil {

		return http.StatusInternalServerError

	}

	defer f.Close()



	name := pullDownloadName(meta)

	w.Header().Set("Content-Type", "application/octet-stream")

	w.Header().Set("Content-Disposition", contentDispositionAttachment(name))

	if meta.Size >= 0 {

		w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))

	}



	n, err := io.Copy(w, f)

	if err != nil {

		return http.StatusInternalServerError

	}

	atomic.AddInt64(&sess.sent, n)

	sess.markPullFileDone(fileID)

	return http.StatusOK

}



func (m *Manager) servePullArchive(sess *sendSession, archivePath string, w http.ResponseWriter) int {

	if archivePath == "" {

		m.logf("web pull archive serve: missing temp file")

		return http.StatusNotFound

	}

	f, err := os.Open(archivePath)

	if err != nil {

		m.logf("web pull archive serve open: %v", err)

		return http.StatusNotFound

	}

	defer f.Close()



	st, err := f.Stat()

	if err != nil || st.Size() == 0 {

		return http.StatusInternalServerError

	}



	name := pullArchiveName(sess.files)

	w.Header().Set("Content-Type", "application/zip")

	w.Header().Set("Content-Disposition", contentDispositionAttachment(name))

	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))



	n, err := io.Copy(w, f)

	if err != nil {

		return http.StatusInternalServerError

	}

	atomic.AddInt64(&sess.sent, n)

	for _, meta := range sess.files {

		sess.markPullFileDone(meta.ID)

	}

	return http.StatusOK

}



func pullDownloadName(f protocol.FileMeta) string {

	if f.RelPath != "" {

		return strings.ReplaceAll(f.RelPath, "/", "_")

	}

	return f.Name

}



func contentDispositionAttachment(name string) string {

	safe := strings.Map(func(r rune) rune {

		if r < 32 || r == '"' || r == '\\' {

			return '_'

		}

		return r

	}, filepath.Base(name))

	return fmt.Sprintf(`attachment; filename="%s"`, safe)

}



func (m *Manager) runSendWebPull(sess *sendSession) {

	defer func() {

		m.removePullArchive(sess)

		m.clearOutgoing(sess)

	}()

	peer := sess.peer



	m.logf("web pull to %s (%s): %d file(s), %d bytes; awaiting phone decision", peer.Name, peer.ID, len(sess.files), sess.total)

	m.emitState(State{Direction: DirSend, State: "waiting", Message: "Ожидание подтверждения на телефоне…", Peer: peer.Name})



	ctx, cancel := context.WithCancel(m.transferCtx())

	sess.setHTTPCancel(cancel)

	defer cancel()



	select {

	case accept := <-sess.webDecision:

		if sess.canceled() {

			m.emitState(State{Direction: DirSend, State: "canceled", Message: "Отменено", Peer: peer.Name})

			return

		}

		if !accept {

			m.logf("web pull to %s: declined on phone", peer.Name)

			m.emitState(State{Direction: DirSend, State: "declined", Message: "Получатель отклонил передачу", Peer: peer.Name})

			return

		}

	case <-time.After(acceptTimeout):

		m.logf("web pull to %s: timed out waiting for phone", peer.Name)

		m.emitState(State{Direction: DirSend, State: "failed", Message: "Телефон не ответил вовремя", Peer: peer.Name})

		return

	case <-ctx.Done():

		msg := "Приложение закрывается"

		if sess.canceled() {

			msg = "Отменено"

		}

		m.emitState(State{Direction: DirSend, State: "canceled", Message: msg, Peer: peer.Name})

		return

	}



	if sess.total == 0 {

		m.emitProgress(Progress{Direction: DirSend, Bytes: 0, Total: 0, Peer: peer.Name})

		m.emitState(State{Direction: DirSend, State: "completed", Message: "Готово", Peer: peer.Name})

		return

	}



	sess.start = time.Now()

	m.emitState(State{Direction: DirSend, State: "transferring", Peer: peer.Name})

	go m.reportLoop(DirSend, peer.Name, sess.files, sess.total, &sess.sent, sess.start, sess.stop)



	transferBudget := idleTimeout + time.Duration(sess.total/(256*1024)+30)*time.Second

	if transferBudget < 2*time.Minute {

		transferBudget = 2 * time.Minute

	}



	select {

	case <-sess.pullWait:

	case <-time.After(transferBudget):

		if atomic.LoadInt64(&sess.sent) < sess.total {

			sess.setErr(fmt.Errorf("таймаут загрузки на телефоне"))

		}

	case <-ctx.Done():

	}



	close(sess.stop)



	switch {

	case sess.canceled():

		m.emitState(State{Direction: DirSend, State: "canceled", Message: "Отменено", Peer: peer.Name})

	case sess.err != nil:

		m.emitState(State{Direction: DirSend, State: "failed", Message: sess.err.Error(), Peer: peer.Name})

	default:

		m.emitProgress(Progress{Direction: DirSend, Bytes: sess.total, Total: sess.total, Peer: peer.Name})

		m.emitState(State{Direction: DirSend, State: "completed", Message: "Готово", Peer: peer.Name})

	}

}



func (s *sendSession) markPullFileDone(fileID string) {

	s.pullDoneMu.Lock()

	s.pullDone[fileID] = true

	all := len(s.pullDone) == len(s.files)

	if all {

		for _, f := range s.files {

			if !s.pullDone[f.ID] {

				all = false

				break

			}

		}

	}

	s.pullDoneMu.Unlock()

	if all {

		s.pullWaitOnce.Do(func() { close(s.pullWait) })

	}

}


