// Package overlay provides invite-scoped mux tunnels between paired peers.
// Peers connect outbound to the rendezvous relay; the server bridges bytes
// only while an active invite session exists (no persistent peer registry).
package overlay

const (
	// Stream kinds — first byte on each yamux stream.
	StreamControl byte = 0x01 // HTTP/1.1 request/response to local control plane
	StreamData    byte = 0x02 // raw TCP proxy to local data plane (token prefix)
	StreamUpgrade byte = 0x03 // relay → direct QUIC negotiate (invite-scoped)
)
