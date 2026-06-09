package server

import (
	"net/http"
	"sync"
	"time"
)

const limitWindow = time.Minute

// Per-IP limits (sliding window) for rendezvous endpoints.
const (
	hostLimitPerMin    = 15
	joinLimitPerMin    = 20
	pollLimitPerMin    = 90
	touchLimitPerMin   = 40
	overlayLimitPerMin = 30
)

type ipLimiter struct {
	mu     sync.Mutex
	events map[string][]int64
	limit  int
	window time.Duration
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{events: make(map[string][]int64), limit: limit, window: window}
}

func (l *ipLimiter) allow(key string) bool {
	now := time.Now().UnixNano()
	cutoff := now - int64(l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := l.events[key]
	keep := ts[:0]
	for _, t := range ts {
		if t >= cutoff {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.limit {
		if len(keep) > 0 {
			l.events[key] = keep
		} else {
			delete(l.events, key)
		}
		return false
	}
	keep = append(keep, now)
	l.events[key] = keep
	return true
}

type endpointLimits struct {
	host    *ipLimiter
	join    *ipLimiter
	poll    *ipLimiter
	touch   *ipLimiter
	overlay *ipLimiter
}

func newEndpointLimits() *endpointLimits {
	return &endpointLimits{
		host:    newIPLimiter(hostLimitPerMin, limitWindow),
		join:    newIPLimiter(joinLimitPerMin, limitWindow),
		poll:    newIPLimiter(pollLimitPerMin, limitWindow),
		touch:   newIPLimiter(touchLimitPerMin, limitWindow),
		overlay: newIPLimiter(overlayLimitPerMin, limitWindow),
	}
}

func (s *Server) rateLimit(w http.ResponseWriter, r *http.Request, lim *ipLimiter) bool {
	if lim.allow(clientIP(r)) {
		return true
	}
	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	return false
}
