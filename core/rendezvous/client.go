package rendezvous

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const clientTimeout = 8 * time.Second

// Client talks to the rendezvous server (signaling only).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client using the hardcoded default server.
func NewClient() *Client {
	return &Client{
		BaseURL: "http://" + DefaultServer(),
		HTTP:    &http.Client{Timeout: clientTimeout},
	}
}

// RegisterHost announces an active invite session to the rendezvous server.
func (c *Client) RegisterHost(ctx context.Context, req HostRegisterRequest) error {
	return c.post(ctx, "/api/v1/rendezvous/host", req, nil)
}

// Join looks up the host for a session and registers this joiner (one-way import).
func (c *Client) Join(ctx context.Context, req JoinRequest) (HostInfo, error) {
	var out HostInfo
	err := c.post(ctx, "/api/v1/rendezvous/join", req, &out)
	return out, err
}

// PollJoiner returns joiner reflexive endpoints when the importer has checked in.
func (c *Client) PollJoiner(ctx context.Context, sessionID, hostPeerID string) (JoinerInfo, bool, error) {
	url := fmt.Sprintf("%s/api/v1/rendezvous/poll?sessionId=%s&hostPeerId=%s",
		c.BaseURL, sessionID, hostPeerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return JoinerInfo{}, false, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return JoinerInfo{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return JoinerInfo{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return JoinerInfo{}, false, fmt.Errorf("poll HTTP %d", resp.StatusCode)
	}
	var out JoinerInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out); err != nil {
		return JoinerInfo{}, false, err
	}
	return out, true, nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("rendezvous %s HTTP %d: %s", path, resp.StatusCode, string(b))
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(out)
	}
	return nil
}
