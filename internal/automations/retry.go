package automations

import (
	"strings"
	"time"
)

const (
	// MaxAttempts is the initial execution plus retries. The occurrence stops after this many tries.
	MaxAttempts = 3
)

// FailureClass decides whether an occurrence may be tried again.
type FailureClass string

const (
	FailureTransient FailureClass = "transient"
	FailurePermanent FailureClass = "permanent"
)

// RetryDelay is the wait before another attempt. The attempt is the try that just failed.
func RetryDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return time.Minute
	case attempt == 2:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

// ClassifyFailure retries timeouts and connection drops.
// A model that cannot start, an out-of-memory failure, or a tool failure stays failed.
// Anything else stays failed as well, so an unknown error cannot retry without a bound that was chosen for it.
func ClassifyFailure(err error) FailureClass {
	if err == nil {
		return FailurePermanent
	}
	msg := strings.ToLower(err.Error())
	if permanentFailure(msg) {
		return FailurePermanent
	}
	if transientFailure(msg) {
		return FailureTransient
	}
	return FailurePermanent
}

func permanentFailure(msg string) bool {
	switch {
	case strings.Contains(msg, "out of memory"),
		strings.Contains(msg, "cannot allocate"),
		strings.Contains(msg, "oom killed"),
		strings.Contains(msg, "(oom)"),
		strings.Contains(msg, " oom"),
		strings.Contains(msg, "no model"),
		strings.Contains(msg, "model failed"),
		strings.Contains(msg, "failed to start"),
		strings.Contains(msg, "not allowed for unattended"),
		strings.Contains(msg, "denied by policy"),
		strings.Contains(msg, "denied by user"),
		strings.Contains(msg, "tool "):
		return true
	default:
		return false
	}
}

func transientFailure(msg string) bool {
	switch {
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "timed out"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "temporarily"),
		strings.Contains(msg, "temporary"),
		strings.Contains(msg, "unreachable"),
		strings.Contains(msg, "i/o timeout"):
		return true
	default:
		return false
	}
}
