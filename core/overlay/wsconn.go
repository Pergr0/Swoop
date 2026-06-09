package overlay

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const wsPingInterval = 25 * time.Second

// wsConn adapts a WebSocket to net.Conn for yamux.
type wsConn struct {
	conn   *websocket.Conn
	r      io.Reader
	mu     sync.Mutex
	closed chan struct{}
}

func newWSConn(conn *websocket.Conn) *wsConn {
	c := &wsConn{conn: conn, closed: make(chan struct{})}
	_ = conn.SetReadDeadline(time.Now().Add(3 * wsPingInterval))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(3 * wsPingInterval))
	})
	go c.pingLoop()
	return c
}

func (c *wsConn) pingLoop() {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			c.mu.Lock()
			err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
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
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return c.conn.Close()
}

func (c *wsConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *wsConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *wsConn) SetDeadline(t time.Time) error      { return nil }
func (c *wsConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *wsConn) SetWriteDeadline(t time.Time) error { return nil }
