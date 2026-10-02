package pluginapi

import (
	"context"
	"errors"
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
	if errors.Is(context.Canceled, ErrLoadFailed) || LoadFailed(nil) != nil {
		t.Fatal("LoadFailed marks what it should not")
	}
}
