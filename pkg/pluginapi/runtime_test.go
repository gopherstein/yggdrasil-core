package pluginapi

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLoadFailed(t *testing.T) {
	cause := errors.New("llama-server exited: out of memory")
	err := LoadFailed(cause)
	if err.Error() != cause.Error() {
		t.Fatalf("message = %q, want %q", err, cause)
	}
	if !errors.Is(err, ErrLoadFailed) || !errors.Is(err, cause) {
		t.Fatal("LoadFailed does not match both ErrLoadFailed and its cause")
	}
	if other := fmt.Errorf("start: %w", context.Canceled); errors.Is(other, ErrLoadFailed) || LoadFailed(nil) != nil {
		t.Fatal("LoadFailed marks what it should not")
	}
}
