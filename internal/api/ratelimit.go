package api

import (
	"sync"
	"time"
)

// rateLimiter is a fixed-window counter per key, used to slow down password
// guessing.
type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newRateLimiter(limit int, w time.Duration) *rateLimiter {
	return &rateLimiter{max: limit, window: w, hits: map[string]*window{}}
}

// allow records a hit for key and reports whether it is within the limit.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.hits) > 10000 {
		for k, w := range l.hits {
			if now.Sub(w.start) > l.window {
				delete(l.hits, k)
			}
		}
	}
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) > l.window {
		l.hits[key] = &window{start: now, count: 1}
		return true
	}
	w.count++
	return w.count <= l.max
}
