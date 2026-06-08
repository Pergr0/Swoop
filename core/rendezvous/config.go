package rendezvous

import "strconv"

// Default server address (signaling only — no file bytes pass through).
// Replace DefaultServerHost with your VPS hostname or static IP before deploy.
const (
	DefaultServerHost = "217.65.79.40"
	DefaultServerPort = 53400
)

// DefaultServer returns host:port for the rendezvous HTTP API.
func DefaultServer() string {
	return DefaultServerHost + ":" + strconv.Itoa(DefaultServerPort)
}
