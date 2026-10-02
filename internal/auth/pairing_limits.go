package auth

import (
	"sync"
	"time"
)

// attemptLimiter counts failed pairing attempts per source address and
// blocks a source that fails max times within window.
type attemptLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	m      map[string]*attempts
}

type attempts struct {
	n     int
	start time.Time
}

func newAttemptLimiter(max int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{max: max, window: window, m: make(map[string]*attempts)}
}

func (l *attemptLimiter) blocked(source string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.m[source]
	return a != nil && time.Since(a.start) < l.window && a.n >= l.max
}

func (l *attemptLimiter) fail(source string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, a := range l.m {
		if now.Sub(a.start) >= l.window {
			delete(l.m, k)
		}
	}
	a := l.m[source]
	if a == nil {
		a = &attempts{start: now}
		l.m[source] = a
	}
	a.n++
}
