package overlay

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsConn adapts a WebSocket to net.Conn for yamux.
type wsConn struct {
	conn *websocket.Conn
	r    io.Reader
	mu   sync.Mutex
}

func newWSConn(conn *websocket.Conn) *wsConn {
	return &wsConn{conn: conn}
}

func (c *wsConn) Read(p []byte) (int, error) {
	if c.r == nil {
		_, r, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		c.r = r
	}
	n, err := c.r.Read(p)
	if err == io.EOF {
		c.r = nil
		if n > 0 {
			return n, nil
		}
		return c.Read(p)
	}
	return n, err
}

func (c *wsConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsConn) Close() error {
	return c.conn.Close()
}

func (c *wsConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *wsConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *wsConn) SetDeadline(t time.Time) error      { return nil }
func (c *wsConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *wsConn) SetWriteDeadline(t time.Time) error { return nil }
