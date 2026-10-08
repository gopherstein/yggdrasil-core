package auth

import (
	"sync"
	"time"
)

// Limiter counts failures per key, such as a username or an address, and
// blocks a key once it has failed max times within window.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	fails map[string][]time.Time
}

// NewLimiter blocks a key after max failures within window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, now: time.Now, fails: map[string][]time.Time{}}
}

func (l *Limiter) recent(key string, now time.Time) []time.Time {
	var kept []time.Time
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = kept
	}
	return kept
}

// Blocked reports a key that has failed too often lately.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, l.now())) >= l.max
}

// Fail counts a failure for key.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	// Keep the map small: an address scanning usernames shouldn't grow it
	// without bound.
	if len(l.fails) > 10000 {
		for k := range l.fails {
			l.recent(k, now)
		}
	}
	l.fails[key] = append(l.recent(key, now), now)
}

// Forget clears a key's failures, such as after a successful sign-in.
func (l *Limiter) Forget(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
