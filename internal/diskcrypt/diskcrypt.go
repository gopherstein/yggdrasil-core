// Package diskcrypt tells whether the disk holding Toskar's data is
// encrypted (#213): FileVault on macOS, BitLocker on Windows, and LUKS on
// Linux. Toskar doesn't encrypt its own files; full-disk encryption keeps
// them encrypted while the computer is off or locked.
package diskcrypt

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// States.
const (
	On         = "on"
	Off        = "off"
	Encrypting = "encrypting"
	Unknown    = "unknown"
)

// Status is whether the data's disk is encrypted, and with what.
type Status struct {
	// State is on, off, encrypting, or unknown.
	State string `json:"state"`
	// Method is FileVault, BitLocker, or LUKS; empty when unknown.
	Method string `json:"method,omitempty"`
}

// run is a command's trimmed output, for the per-system checks; tests
// replace it.
var run = func(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

var cache struct {
	sync.Mutex
	path string
	at   time.Time
	st   Status
}

// cacheFor is how long an answer is kept: turning encryption on or off
// takes a restart or hours of encrypting anyway.
const cacheFor = 10 * time.Minute

// Detect reports the encryption of the disk path is on, remembered for a
// while.
func Detect(ctx context.Context, path string) Status {
	cache.Lock()
	if cache.path == path && time.Since(cache.at) < cacheFor {
		st := cache.st
		cache.Unlock()
		return st
	}
	cache.Unlock()
	st := detect(ctx, path)
	if st.State == "" {
		st.State = Unknown
	}
	cache.Lock()
	cache.path, cache.at, cache.st = path, time.Now(), st
	cache.Unlock()
	return st
}
