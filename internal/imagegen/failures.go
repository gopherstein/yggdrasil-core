package imagegen

import (
	"context"
	"fmt"
	"strings"
)

// A picture or clip uses most of the memory this computer has (#541).
// Before a job, Room lets the app make space, unloading idle chat models,
// and hold it while the job runs. When a job still fails, what sd-cli
// printed is turned into a sentence the person can act on; the raw output
// goes to the log.

// Failure kinds sd-cli's output is sorted into.
const (
	FailureMemory  = "out_of_memory"
	FailureKilled  = "killed"
	FailureGPULost = "gpu_lost"
	FailureOther   = "failed"
)

// memorySigns are what ggml and its backends print when an allocation
// fails: system memory, or graphics memory on Vulkan, CUDA, ROCm, or Metal.
var memorySigns = []string{
	"out of memory", "out_of_memory", "outofmemory", "out of device memory", "erroroutofdevicememory",
	"vk_error_out_of_device_memory", "vk_error_out_of_host_memory", "cannot allocate", "failed to allocate",
	"memory allocation of", "allocation failed", "insufficient memory", "not enough memory",
	"cudamalloc failed", "hipmalloc failed", "kiogpucommandbuffercallbackerroroutofmemory", "std::bad_alloc",
}

// gpuLostSigns are a graphics driver giving up partway through.
var gpuLostSigns = []string{"vk_error_device_lost", "device lost", "device_lost", "gpu hang", "kiogpucommandbuffercallbackerrortimeout"}

// failureKind sorts a failed run by its output and how it ended.
func failureKind(output string, runErr error) string {
	lower := strings.ToLower(output)
	for _, s := range memorySigns {
		if strings.Contains(lower, s) {
			return FailureMemory
		}
	}
	for _, s := range gpuLostSigns {
		if strings.Contains(lower, s) {
			return FailureGPULost
		}
	}
	// The system ends a process that takes too much memory with SIGKILL.
	if runErr != nil {
		if msg := runErr.Error(); strings.Contains(msg, "signal: killed") || strings.Contains(msg, "exit status 137") {
			return FailureKilled
		}
	}
	return FailureOther
}

// thing is what a job makes, in a sentence: "picture" or "clip".
func thing(what string) string {
	if what == "clip" {
		return "clip"
	}
	return "picture"
}

// failureMessage is what the person is told when a job fails.
func failureMessage(kind, what, lastLine string) string {
	smaller := "Try a smaller picture"
	if what == "clip" {
		smaller = "Try a shorter or smaller clip"
	}
	switch kind {
	case FailureMemory:
		return fmt.Sprintf("This computer ran out of memory making the %s. %s, or make it on a paired computer with more memory.", thing(what), smaller)
	case FailureKilled:
		return fmt.Sprintf("The system stopped the %s, most likely because the computer ran out of memory. %s, or make it on a paired computer with more memory.", thing(what), smaller)
	case FailureGPULost:
		return fmt.Sprintf("The graphics card stopped responding while making the %s. Try again; if it keeps happening, restart the computer or update its graphics driver.", thing(what))
	}
	return fmt.Sprintf("the %s could not be made: %s", what, lastLine)
}

// makeRoom asks the app to free memory for m before a job, and returns
// what gives it back. Without Room, nothing is freed.
func (e *Engine) makeRoom(ctx context.Context, m Model, what string) (func(), error) {
	if e.Room == nil {
		return func() {}, nil
	}
	release, err := e.Room(ctx, m.MemoryBytes, thing(what))
	if err != nil {
		return nil, err
	}
	if release == nil {
		release = func() {}
	}
	return release, nil
}
