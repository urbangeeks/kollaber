package api

import (
	"sync"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// orgRateLimiter hands out a token-bucket limiter per org for burst protection
// on expensive endpoints. It is in-memory and per-process: it guards against a
// single org hammering the endpoint, not against distributed abuse across
// replicas — the persistent monthly quota is the hard cost ceiling.
type orgRateLimiter struct {
	mu      sync.Mutex
	buckets map[uuid.UUID]*rate.Limiter
	rate    rate.Limit
	burst   int
}

func newOrgRateLimiter(r rate.Limit, burst int) *orgRateLimiter {
	return &orgRateLimiter{
		buckets: make(map[uuid.UUID]*rate.Limiter),
		rate:    r,
		burst:   burst,
	}
}

// Allow reports whether the org may make a request right now, consuming a token
// if so.
func (l *orgRateLimiter) Allow(org uuid.UUID) bool {
	l.mu.Lock()
	lim, ok := l.buckets[org]
	if !ok {
		lim = rate.NewLimiter(l.rate, l.burst)
		l.buckets[org] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}
