package imagegen

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// What sd-cli prints when it fails is sorted, and the person is told what
// happened in words they can act on (#541).
func TestFailureMessages(t *testing.T) {
	for _, c := range []struct {
		output string
		err    error
		kind   string
	}{
		{"ggml_vulkan: Device memory allocation of size 4294967296 failed.\nggml_vulkan: vk::Device::allocateMemory: ErrorOutOfDeviceMemory", errors.New("exit status 1"), FailureMemory},
		{"ggml_backend_cuda_buffer_type_alloc_buffer: allocating 9000.00 MiB on device 0: cudaMalloc failed: out of memory", errors.New("exit status 1"), FailureMemory},
		{"[INFO] sampling 3/20", errors.New("signal: killed"), FailureKilled},
		{"ggml_vulkan: VK_ERROR_DEVICE_LOST", errors.New("exit status 1"), FailureGPULost},
		{"error: model file is corrupt", errors.New("exit status 1"), FailureOther},
	} {
		if got := failureKind(c.output, c.err); got != c.kind {
			t.Errorf("%q = %s, want %s", c.output, got, c.kind)
		}
	}
	msg := failureMessage(FailureMemory, "clip", "")
	if !strings.Contains(msg, "ran out of memory making the clip") || !strings.Contains(msg, "shorter or smaller clip") || strings.Contains(strings.ToLower(msg), "vulkan") {
		t.Fatalf("memory message %q", msg)
	}
	if msg := failureMessage(FailureKilled, "image", ""); !strings.Contains(msg, "stopped the picture") || !strings.Contains(msg, "smaller picture") {
		t.Fatalf("killed message %q", msg)
	}
	if msg := failureMessage(FailureOther, "image", "model file is corrupt"); msg != "the image could not be made: model file is corrupt" {
		t.Fatalf("other message %q", msg)
	}
}

// Before a job, the app is asked for room for the model's memory, and it
// is given back after; a job the app can't make room for doesn't start.
func TestJobMakesRoomFirst(t *testing.T) {
	f, eng, _, _ := setupTools(t)
	var asked []string
	held := false
	eng.Room = func(_ context.Context, need int64, what string) (func(), error) {
		m := f.model(t)
		if need != m.MemoryBytes || what != "picture" {
			t.Errorf("room for %d %s, want %d picture", need, what, m.MemoryBytes)
		}
		asked = append(asked, "room")
		held = true
		return func() { held = false; asked = append(asked, "release") }, nil
	}
	if _, err := eng.Generate(context.Background(), Request{Prompt: "a dog"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(asked, ",") != "room,release" || held {
		t.Fatalf("asked %v, still held %v", asked, held)
	}
	// Failing jobs give the room back too.
	if _, err := eng.Generate(context.Background(), Request{Prompt: "fail"}); err == nil || held {
		t.Fatalf("failure = %v, held %v", err, held)
	}
	eng.Room = func(context.Context, int64, string) (func(), error) { return nil, context.Canceled }
	if _, err := eng.Generate(context.Background(), Request{Prompt: "a cat"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ran without room: %v", err)
	}
	if got := args(t, f); flag(got, "-p") == "a cat" {
		t.Fatal("sd-cli ran without room")
	}
}
