package overlay

import (
	"errors"
	"net"
)

// P2P note codes surfaced to the UI when direct QUIC upgrade does not succeed.
const (
	P2PNoteUpgrading  = "upgrading"
	P2PNoteNoUpnp     = "no_upnp"
	P2PNotePunch      = "punch_timeout"
	P2PNoteQuic       = "quic_failed"
	P2PNoteNegotiate  = "negotiate_failed"
	P2PNoteNoPunch    = "no_punch"
)

var (
	errUpgradeNoRelay   = errors.New("no relay session")
	errUpgradeNoQuic    = errors.New("host offered no quic port")
	errUpgradePunch     = errors.New("punch timeout")
	errUpgradeQuicDial  = errors.New("quic dial failed")
	errUpgradeNegotiate = errors.New("upgrade negotiate failed")
)

func (l *Link) setP2PNote(note string) {
	l.mu.Lock()
	l.p2pNote = note
	l.mu.Unlock()
	l.notifyModeChange()
}

func (l *Link) P2PNote() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.p2pNote
}

// SetOnModeChange is called when relay/direct mode or P2P note changes.
func (l *Link) SetOnModeChange(fn func()) {
	l.mu.Lock()
	l.onModeChange = fn
	l.mu.Unlock()
}

func (l *Link) notifyModeChange() {
	l.mu.RLock()
	fn := l.onModeChange
	l.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// QuicPort returns the host-side QUIC UDP port (0 until ServeHost binds).
func (l *Link) QuicPort() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.quicPort
}

func classifyUpgradeFail(err error, offer upgradeOffer) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errUpgradePunch) {
		if offer.ReachAddr == "" && isPrivateIPv4(offer.LanAddr) {
			return P2PNoteNoUpnp
		}
		return P2PNotePunch
	}
	if errors.Is(err, errUpgradeQuicDial) || errors.Is(err, errUpgradeNoQuic) {
		if offer.ReachAddr == "" && isPrivateIPv4(offer.LanAddr) {
			return P2PNoteNoUpnp
		}
		return P2PNoteQuic
	}
	if errors.Is(err, errUpgradeNoRelay) || errors.Is(err, errUpgradeNegotiate) {
		return P2PNoteNegotiate
	}
	if offer.ReachAddr == "" && isPrivateIPv4(offer.LanAddr) {
		return P2PNoteNoUpnp
	}
	return P2PNoteQuic
}

func isPrivateIPv4(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	ip4 := ip.To4()
	return ip4 == nil || ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast()
}
