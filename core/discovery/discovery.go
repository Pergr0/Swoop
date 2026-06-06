// Package discovery finds Swoop peers on the local network. This is the
// dependency-free UDP multicast fallback: each device periodically announces
// its DeviceInfo and listens for others. mDNS/DNS-SD and a QR/manual path
// will be layered on top later without changing this interface.
//
// Discovery joins the multicast group and announces on every multicast-capable
// interface, which is what makes it work reliably across machines with several
// adapters (Ethernet + Wi-Fi + VPN), a common cause of "devices can't see each
// other" on both Linux and Windows.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/ipv4"

	"swoop/core/protocol"
)

// Discoverer announces this device and tracks discovered peers.
type Discoverer struct {
	self     protocol.DeviceInfo
	interval time.Duration

	mu    sync.RWMutex
	peers map[string]peerEntry

	onChange func([]protocol.DeviceInfo)

	only *net.Interface // when set, announce/listen only on this interface

	sendMu sync.Mutex
}

// SetInterface restricts discovery to a single interface. Must be called before
// Start. A nil interface (the default) uses every multicast-capable interface.
func (d *Discoverer) SetInterface(ifi *net.Interface) { d.only = ifi }

type peerEntry struct {
	info     protocol.DeviceInfo
	lastSeen time.Time
}

// New creates a Discoverer that advertises self.
func New(self protocol.DeviceInfo) *Discoverer {
	return &Discoverer{
		self:     self,
		interval: 3 * time.Second,
		peers:    make(map[string]peerEntry),
	}
}

// OnChange registers a callback invoked whenever the known peer set changes.
func (d *Discoverer) OnChange(fn func([]protocol.DeviceInfo)) { d.onChange = fn }

// Start begins announcing and listening until ctx is cancelled.
func (d *Discoverer) Start(ctx context.Context) error {
	group := net.ParseIP(protocol.DiscoveryMulticastGroup)
	dst := &net.UDPAddr{IP: group, Port: protocol.DiscoveryPort}

	conn, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", protocol.DiscoveryPort))
	if err != nil {
		return err
	}
	pconn := ipv4.NewPacketConn(conn)
	_ = pconn.SetMulticastLoopback(true)

	ifaces := multicastInterfaces()
	if d.only != nil {
		ifaces = []net.Interface{*d.only}
	}
	joined := 0
	for i := range ifaces {
		if err := pconn.JoinGroup(&ifaces[i], &net.UDPAddr{IP: group}); err == nil {
			joined++
		}
	}
	if joined == 0 {
		// Fall back to the default interface chosen by the OS.
		_ = pconn.JoinGroup(nil, &net.UDPAddr{IP: group})
	}

	go d.readLoop(ctx, pconn)
	go d.announceLoop(ctx, pconn, ifaces, dst)
	go d.reapLoop(ctx)

	go func() {
		<-ctx.Done()
		_ = pconn.Close()
	}()
	return nil
}

func (d *Discoverer) announceLoop(ctx context.Context, pconn *ipv4.PacketConn, ifaces []net.Interface, dst *net.UDPAddr) {
	payload, _ := json.Marshal(d.self)
	t := time.NewTicker(d.interval)
	defer t.Stop()
	for {
		d.broadcast(pconn, ifaces, dst, payload)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// broadcast sends payload to the multicast group on every interface so peers
// on any adapter receive the announcement.
func (d *Discoverer) broadcast(pconn *ipv4.PacketConn, ifaces []net.Interface, dst *net.UDPAddr, payload []byte) {
	d.sendMu.Lock()
	defer d.sendMu.Unlock()

	if len(ifaces) == 0 {
		_, _ = pconn.WriteTo(payload, nil, dst)
		return
	}
	for i := range ifaces {
		if err := pconn.SetMulticastInterface(&ifaces[i]); err != nil {
			continue
		}
		_, _ = pconn.WriteTo(payload, nil, dst)
	}
}

func (d *Discoverer) readLoop(ctx context.Context, pconn *ipv4.PacketConn) {
	buf := make([]byte, 64*1024)
	for {
		n, _, src, err := pconn.ReadFrom(buf)
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		var info protocol.DeviceInfo
		if err := json.Unmarshal(buf[:n], &info); err != nil {
			continue
		}
		if info.ID == "" || info.ID == d.self.ID {
			continue
		}
		// Prefer the address the peer advertises (it reflects the interface it
		// chose / is bound to). Only fall back to the packet source address if
		// the peer did not report one. Trusting the source address unconditionally
		// is wrong behind NAT/multi-interface setups, where it is the gateway's
		// address rather than the peer's.
		if info.Address == "" {
			if ua, ok := src.(*net.UDPAddr); ok && ua.IP != nil {
				info.Address = ua.IP.String()
			}
		}
		d.upsert(info)
	}
}

func (d *Discoverer) upsert(info protocol.DeviceInfo) {
	d.mu.Lock()
	_, existed := d.peers[info.ID]
	d.peers[info.ID] = peerEntry{info: info, lastSeen: time.Now()}
	d.mu.Unlock()
	if !existed {
		d.emit()
	}
}

func (d *Discoverer) reapLoop(ctx context.Context) {
	t := time.NewTicker(d.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cutoff := time.Now().Add(-3 * d.interval)
			changed := false
			d.mu.Lock()
			for id, e := range d.peers {
				if e.lastSeen.Before(cutoff) {
					delete(d.peers, id)
					changed = true
				}
			}
			d.mu.Unlock()
			if changed {
				d.emit()
			}
		}
	}
}

// Peers returns a snapshot of currently known peers.
func (d *Discoverer) Peers() []protocol.DeviceInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]protocol.DeviceInfo, 0, len(d.peers))
	for _, e := range d.peers {
		out = append(out, e.info)
	}
	return out
}

func (d *Discoverer) emit() {
	if d.onChange != nil {
		d.onChange(d.Peers())
	}
}

// multicastInterfaces returns interfaces that are up and multicast-capable.
func multicastInterfaces() []net.Interface {
	all, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]net.Interface, 0, len(all))
	for _, ifi := range all {
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if ifi.Flags&net.FlagMulticast == 0 {
			continue
		}
		out = append(out, ifi)
	}
	return out
}
