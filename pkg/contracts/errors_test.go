package contracts

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCode(t *testing.T) {
	sentinel := NewError("MEMORY_NOT_FOUND", nil, errors.New("memory not found"))
	wrapped := fmt.Errorf("delete: %w", sentinel)
	if code, _ := ErrorCode(wrapped); code != "MEMORY_NOT_FOUND" {
		t.Fatalf("code through wrapping = %q", code)
	}
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("a coded sentinel still matches errors.Is")
	}
	if wrapped.Error() != "delete: memory not found" {
		t.Fatalf("message = %q", wrapped.Error())
	}

	err := Errorf("MODEL_NOT_INSTALLED", map[string]any{"model_id": "qwen3"}, "model %q not installed: %w", "qwen3", errors.ErrUnsupported)
	code, params := ErrorCode(err)
	if code != "MODEL_NOT_INSTALLED" || params["model_id"] != "qwen3" {
		t.Fatalf("got %q %v", code, params)
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatal("Errorf wraps %w")
	}

	if code, params := ErrorCode(errors.New("plain")); code != "" || params != nil {
		t.Fatalf("an uncoded error has no code, got %q", code)
	}
	if code, _ := ErrorCode(nil); code != "" {
		t.Fatal("nil has no code")
	}
}
