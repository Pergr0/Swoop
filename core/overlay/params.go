package overlay

import (
	"crypto/tls"
	"net"
)

// HostParams configures an invite host overlay (relay + optional direct QUIC).
type HostParams struct {
	SessionID   string
	PeerID      string
	ControlPort int
	DataPort    int
	HostCert    tls.Certificate
	ReachAddr   string
	LanAddr     string
	PunchPort   int
	PunchConn   *net.UDPConn
	Logf        func(string, ...any)
}

// UpgradeParams configures joiner-side relay → direct QUIC upgrade after punch.
type UpgradeParams struct {
	SessionID       string
	Fingerprint     string
	ReachAddr       string
	LanAddr         string
	HostPunchPort   int
	HostQuicPort    int // filled after upgrade negotiate if host uses dynamic port
	JoinerPunchPort int
	JoinerReflexive string
	PunchConn       *net.UDPConn
	Logf            func(string, ...any)
}
