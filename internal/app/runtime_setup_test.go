package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/runtimes"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// setupRuntime is llama.cpp as it is before onboarding: not installed until
// Install runs.
type setupRuntime struct {
	pluginapi.Runtime
	installed atomic.Bool
	installs  atomic.Int32
	fail      error
}

func (r *setupRuntime) ID() string { return "llamacpp" }
func (r *setupRuntime) Detect(context.Context) (pluginapi.RuntimeDetection, error) {
	return pluginapi.RuntimeDetection{Installed: r.installed.Load()}, nil
}
func (r *setupRuntime) Install(ctx context.Context, _ pluginapi.InstallOptions) error {
	r.installs.Add(1)
	time.Sleep(20 * time.Millisecond)
	if r.fail != nil {
		return r.fail
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.installed.Store(true)
	return nil
}

func setupApp(rt *setupRuntime) *App {
	reg := runtimes.NewRegistry()
	reg.Register(rt)
	return &App{Runtimes: runtimes.NewManager(reg, nil, nil, nil)}
}

// Start sets llama.cpp up when it's missing, once however many starts
// arrive together, and not again once it's there (#610).
func TestStartSetsUpTheRuntime(t *testing.T) {
	rt := &setupRuntime{}
	a := setupApp(rt)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.ensureRuntime(context.Background(), "qwen"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if !rt.installed.Load() || rt.installs.Load() != 1 {
		t.Fatalf("installed=%v installs=%d", rt.installed.Load(), rt.installs.Load())
	}
	if err := a.ensureRuntime(context.Background(), "qwen"); err != nil || rt.installs.Load() != 1 {
		t.Fatalf("again: %v, installs=%d", err, rt.installs.Load())
	}
}

// The setup goes on when the request that asked for it ends.
func TestRuntimeSetupOutlivesTheRequest(t *testing.T) {
	rt := &setupRuntime{}
	a := setupApp(rt)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.ensureRuntime(ctx, "qwen"); err != nil || !rt.installed.Load() {
		t.Fatalf("err=%v installed=%v", err, rt.installed.Load())
	}
}

// A setup that fails says so with its own code and why.
func TestRuntimeSetupFailureIsCoded(t *testing.T) {
	a := setupApp(&setupRuntime{fail: errors.New("no network")})
	err := a.ensureRuntime(context.Background(), "qwen")
	code, details := contracts.ErrorCode(err)
	if code != "RUNTIME_SETUP_FAILED" || details["reason"] != "no network" {
		t.Fatalf("err = %v (%s %v)", err, code, details)
	}
}
