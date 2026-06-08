// Package pairing stores peers added via verified SwoopInvite (out-of-band).
// They appear in the device grid alongside LAN discovery and browser clients.
package pairing

import (
	"context"
	"sync"
	"time"

	"swoop/core/invite"
	"swoop/core/protocol"
)

const (
	StatusConnecting = "connecting"
	StatusConnected  = "connected"
	StatusError      = "error"

	reapInterval  = 30 * time.Second
	probeInterval = 25 * time.Second
)

type entry struct {
	info      protocol.DeviceInfo
	expiresAt int64
	invite    invite.Parsed
	status    string
}

// Registry holds invite-paired peers until their invite expires.
type Registry struct {
	mu       sync.RWMutex
	peers    map[string]entry
	order    []string
	onChange func([]protocol.DeviceInfo)
	onProbe  func(id string) // called on interval and after add
}

// New creates an empty paired-peer registry.
func New() *Registry {
	return &Registry{peers: make(map[string]entry)}
}

// OnChange registers a callback when the visible peer set changes.
func (r *Registry) OnChange(fn func([]protocol.DeviceInfo)) { r.onChange = fn }

// OnProbe registers a callback to re-check reachability (engine provides TLS /info probe).
func (r *Registry) OnProbe(fn func(id string)) { r.onProbe = fn }

// Add records a verified invite peer. Replaces an existing entry with the same ID.
func (r *Registry) Add(device protocol.DeviceInfo, inv invite.Parsed) {
	r.mu.Lock()
	if _, ok := r.peers[device.ID]; !ok {
		r.order = append(r.order, device.ID)
	}
	device.Paired = true
	device.PairStatus = StatusConnecting
	r.peers[device.ID] = entry{
		info:      device,
		expiresAt: inv.ExpiresAt,
		invite:    inv,
		status:    StatusConnecting,
	}
	id := device.ID
	r.mu.Unlock()
	r.emit()
	if r.onProbe != nil {
		go r.onProbe(id)
	}
}

// Update replaces stored device info after a successful probe.
func (r *Registry) Update(id string, device protocol.DeviceInfo) {
	r.mu.Lock()
	e, ok := r.peers[id]
	if !ok {
		r.mu.Unlock()
		return
	}
	device.Paired = true
	device.PairStatus = StatusConnected
	e.info = device
	e.status = StatusConnected
	r.peers[id] = e
	r.mu.Unlock()
	r.emit()
}

// SetStatus updates connection status without replacing device info.
func (r *Registry) SetStatus(id, status string) {
	r.mu.Lock()
	e, ok := r.peers[id]
	if !ok {
		r.mu.Unlock()
		return
	}
	e.status = status
	e.info.PairStatus = status
	e.info.Paired = true
	r.peers[id] = e
	r.mu.Unlock()
	r.emit()
}

// Get returns the stored device for probing.
func (r *Registry) Get(id string) (protocol.DeviceInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.peers[id]
	if !ok {
		return protocol.DeviceInfo{}, false
	}
	return e.info, true
}

// InviteMeta returns the stored invite used for punch retries.
func (r *Registry) InviteMeta(id string) (invite.Parsed, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.peers[id]
	if !ok {
		return invite.Parsed{}, false
	}
	return e.invite, true
}

// IDs returns paired peer IDs in order (for background polling).
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Peers returns paired peers in first-added order.
func (r *Registry) Peers() []protocol.DeviceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.peersSnapshot()
}

func (r *Registry) peersSnapshot() []protocol.DeviceInfo {
	out := make([]protocol.DeviceInfo, 0, len(r.order))
	for _, id := range r.order {
		if e, ok := r.peers[id]; ok {
			info := e.info
			info.Paired = true
			info.PairStatus = e.status
			out = append(out, info)
		}
	}
	return out
}

// Start reaps expired peers and periodically re-probes reachability.
func (r *Registry) Start(ctx context.Context) {
	reap := time.NewTicker(reapInterval)
	probe := time.NewTicker(probeInterval)
	defer reap.Stop()
	defer probe.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-reap.C:
			r.reap()
		case <-probe.C:
			r.probeAll()
		}
	}
}

func (r *Registry) probeAll() {
	if r.onProbe == nil {
		return
	}
	for _, id := range r.IDs() {
		go r.onProbe(id)
	}
}

func (r *Registry) reap() {
	now := time.Now().Unix()
	r.mu.Lock()
	changed := false
	for id, e := range r.peers {
		if e.expiresAt > 0 && now > e.expiresAt {
			delete(r.peers, id)
			r.order = removeID(r.order, id)
			changed = true
		}
	}
	r.mu.Unlock()
	if changed {
		r.emit()
	}
}

func (r *Registry) emit() {
	if r.onChange != nil {
		r.onChange(r.Peers())
	}
}

func removeID(order []string, id string) []string {
	for i, x := range order {
		if x == id {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}
