// Package webpresence tracks mobile/browser clients that open the desktop's

// HTTPS upload page. They do not use UDP discovery; heartbeats keep them in

// the device grid until they go away.

package webpresence



import (
	"context"
	"sync"
	"time"

	"swoop/core/protocol"
)



const defaultInterval = 8 * time.Second



// Registry holds browser clients seen via POST /api/v1/presence.

type Registry struct {

	interval time.Duration

	secret   []byte



	mu       sync.RWMutex

	clients  map[string]clientEntry

	order    []string

	onChange func([]protocol.DeviceInfo)



	controlPort func() int

}



type clientEntry struct {

	info     protocol.DeviceInfo

	lastSeen time.Time

	remoteIP string

}



// New creates a registry. controlPort supplies the desktop control port for

// DeviceInfo (used in the grid subtitle).

func New(controlPort func() int) *Registry {

	return &Registry{

		interval:    defaultInterval,

		secret:      newPresenceSecret(),

		clients:     make(map[string]clientEntry),

		controlPort: controlPort,

	}

}



// OnChange registers a callback when the visible client set changes.

func (r *Registry) OnChange(fn func([]protocol.DeviceInfo)) { r.onChange = fn }



// Touch records or refreshes a browser client. Returns an HMAC token bound to

// clientID and remote IP, or empty token on hijack attempt (403).

func (r *Registry) Touch(req protocol.PresenceRequest, userAgent, remoteAddr string) (protocol.PresenceResponse, int) {

	if req.ID == "" {

		return protocol.PresenceResponse{}, 400

	}

	ip := remoteHost(remoteAddr)

	browser := ParseBrowser(userAgent)

	name := req.Name

	if name == "" {

		name = browser

	}



	info := protocol.DeviceInfo{

		ID:          req.ID,

		Name:        name,

		Address:     ip,

		Platform:    protocol.PlatformWeb,

		ControlPort: r.controlPort(),

		Browser:     browser,

		Version:     protocol.Version,

	}



	r.mu.Lock()

	prev, existed := r.clients[req.ID]

	if existed && prev.remoteIP != "" && prev.remoteIP != ip {

		r.mu.Unlock()

		return protocol.PresenceResponse{}, 403

	}

	r.clients[req.ID] = clientEntry{info: info, lastSeen: time.Now(), remoteIP: ip}

	if !existed {

		r.order = append(r.order, req.ID)

	}

	r.mu.Unlock()

	if !existed {

		r.emit()

	}

	return protocol.PresenceResponse{Token: presenceToken(r.secret, req.ID, ip)}, 200

}



// Verify checks that clientID, remoteAddr and token match a live registration.

func (r *Registry) Verify(clientID, remoteAddr, token string) bool {

	ip := remoteHost(remoteAddr)

	r.mu.RLock()

	e, ok := r.clients[clientID]

	r.mu.RUnlock()

	if !ok || e.remoteIP != ip {

		return false

	}

	return verifyPresenceToken(r.secret, clientID, ip, token)

}



// ClientIP returns the bound IP for a browser client, if known.

func (r *Registry) ClientIP(clientID string) string {

	r.mu.RLock()

	defer r.mu.RUnlock()

	if e, ok := r.clients[clientID]; ok {

		return e.remoteIP

	}

	return ""

}



// Peers returns browser clients in first-seen order.

func (r *Registry) Peers() []protocol.DeviceInfo {

	r.mu.RLock()

	defer r.mu.RUnlock()

	out := make([]protocol.DeviceInfo, 0, len(r.order))

	for _, id := range r.order {

		if e, ok := r.clients[id]; ok {

			out = append(out, e.info)

		}

	}

	return out

}



// Start runs the stale-client reaper until ctx is cancelled.

func (r *Registry) Start(ctx context.Context) {

	t := time.NewTicker(r.interval)

	defer t.Stop()

	for {

		select {

		case <-ctx.Done():

			return

		case <-t.C:

			cutoff := time.Now().Add(-3 * r.interval)

			changed := false

			r.mu.Lock()

			for id, e := range r.clients {

				if e.lastSeen.Before(cutoff) {

					delete(r.clients, id)

					r.order = removeID(r.order, id)

					changed = true

				}

			}

			r.mu.Unlock()

			if changed {

				r.emit()

			}

		}

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


