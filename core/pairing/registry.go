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

	// InternetIdleTimeout is how long an invite-paired peer may stay connected
	// without chat or file-transfer activity (active transfers pause the timer).
	InternetIdleTimeout = 20 * time.Minute
)

type entry struct {
	info         protocol.DeviceInfo
	invite       invite.Parsed
	status       string
	idleDeadline time.Time
}

// Registry holds invite-paired internet peers until idle timeout or removal.
type Registry struct {
	mu       sync.RWMutex
	peers    map[string]entry
	order    []string
	onChange func([]protocol.DeviceInfo)
	onProbe  func(id string) // called on interval and after add
	onRemove func(id string)
	onIdle func(protocol.DeviceInfo) // idle timeout: send goodbye, tear down overlay
	isBusy func(id string) bool
}

// New creates an empty paired-peer registry.
func New() *Registry {
	return &Registry{peers: make(map[string]entry)}
}

// OnChange registers a callback when the visible peer set changes.
func (r *Registry) OnChange(fn func([]protocol.DeviceInfo)) { r.onChange = fn }

// OnProbe registers a callback to re-check reachability (engine provides TLS /info probe).
func (r *Registry) OnProbe(fn func(id string)) { r.onProbe = fn }

// OnRemove registers a callback when a paired peer is removed.
func (r *Registry) OnRemove(fn func(id string)) { r.onRemove = fn }

// SetIdlePolicy configures idle reap: isBusy pauses the timer during transfers.
func (r *Registry) SetIdlePolicy(isBusy func(id string) bool, onIdle func(protocol.DeviceInfo)) {
	r.isBusy = isBusy
	r.onIdle = onIdle
}

// Add records a verified invite peer. Replaces an existing entry with the same ID.
func (r *Registry) Add(device protocol.DeviceInfo, inv invite.Parsed) {
	r.mu.Lock()
	if _, ok := r.peers[device.ID]; !ok {
		r.order = append(r.order, device.ID)
	}
	device.Paired = true
	device.PairStatus = StatusConnecting
	r.peers[device.ID] = entry{
		info:         device,
		invite:       inv,
		status:       StatusConnecting,
		idleDeadline: time.Now().Add(InternetIdleTimeout),
	}
	id := device.ID
	r.mu.Unlock()
	r.emit()
	if r.onProbe != nil {
		go r.onProbe(id)
	}
}

// UpdateInvite refreshes stored invite endpoints after rendezvous signaling.
func (r *Registry) UpdateInvite(id string, inv invite.Parsed, device protocol.DeviceInfo) {
	r.mu.Lock()
	e, ok := r.peers[id]
	if !ok {
		r.mu.Unlock()
		return
	}
	device.Paired = true
	device.PairStatus = e.status
	e.invite = inv
	e.info = device
	r.peers[id] = e
	r.mu.Unlock()
	r.emit()
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

// TouchActivity resets the internet idle timer for a paired peer.
func (r *Registry) TouchActivity(id string) bool {
	r.mu.Lock()
	e, ok := r.peers[id]
	if !ok {
		r.mu.Unlock()
		return false
	}
	e.idleDeadline = time.Now().Add(InternetIdleTimeout)
	r.peers[id] = e
	r.mu.Unlock()
	return true
}

// Remove drops a paired peer immediately (goodbye or unreachable).
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	_, ok := r.peers[id]
	if ok {
		delete(r.peers, id)
		r.order = removeID(r.order, id)
	}
	r.mu.Unlock()
	if !ok {
		return
	}
	if r.onRemove != nil {
		r.onRemove(id)
	}
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
			r.reapIdle()
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

func (r *Registry) reapIdle() {
	now := time.Now()
	var expired []protocol.DeviceInfo
	r.mu.Lock()
	for id, e := range r.peers {
		if r.isBusy != nil && r.isBusy(id) {
			continue
		}
		if !e.idleDeadline.IsZero() && now.After(e.idleDeadline) {
			expired = append(expired, e.info)
			delete(r.peers, id)
			r.order = removeID(r.order, id)
		}
	}
	r.mu.Unlock()
	if len(expired) == 0 {
		return
	}
	for _, peer := range expired {
		if r.onIdle != nil {
			r.onIdle(peer)
		} else if r.onRemove != nil {
			r.onRemove(peer.ID)
		}
	}
	r.emit()
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
