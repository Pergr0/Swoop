package core

import (
	"net/http"
	"testing"

	"swoop/core/protocol"
	"swoop/core/webpresence"
)

func TestWebChatPollAndSend(t *testing.T) {
	e, err := New(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e.webPresence = webpresence.New(func() int { return 53317 })

	const clientID = "web-test"
	resp, st := e.webPresence.Touch(
		protocol.PresenceRequest{ID: clientID, Name: "Phone"},
		"Mozilla", "192.168.1.5:1234",
	)
	if st != http.StatusOK || resp.Token == "" {
		t.Fatalf("touch status=%d token=%q", st, resp.Token)
	}

	peer := protocol.DeviceInfo{ID: clientID, Name: "Phone", Platform: protocol.PlatformWeb}
	if err := e.SendMessage(clientID, "hello browser"); err != nil {
		t.Fatal(err)
	}
	_ = peer

	poll, status := e.PollWebChat(clientID, "192.168.1.5:1234", resp.Token, 0)
	if status != http.StatusOK || len(poll.Messages) != 1 || poll.Messages[0].Text != "hello browser" {
		t.Fatalf("poll status=%d resp=%+v", status, poll)
	}

	empty, status := e.PollWebChat(clientID, "192.168.1.5:1234", resp.Token, poll.Messages[0].Ts)
	if status != http.StatusNoContent && (status != http.StatusOK || len(empty.Messages) != 0) {
		t.Fatalf("second poll status=%d resp=%+v", status, empty)
	}

	in := protocol.ChatMessage{
		Sender: protocol.DeviceInfo{ID: clientID, Name: "Phone", Platform: protocol.PlatformWeb},
		Text:   "hi desktop",
		Ts:     42,
	}
	if st := e.ReceiveMessage(in, "192.168.1.5:1234", resp.Token); st != http.StatusOK {
		t.Fatalf("receive status=%d", st)
	}
	if st := e.ReceiveMessage(in, "192.168.1.5:1234", "bad"); st != http.StatusForbidden {
		t.Fatalf("forged token status=%d", st)
	}
}
