package app

import (
	"context"
	"errors"
	"time"

	"github.com/yeixio/toskar-core/internal/runtimes"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// runtimeSetupFor bounds setting up llama.cpp, a download of tens of
// megabytes, which runs on after the request that needed it ends.
const runtimeSetupFor = 10 * time.Minute

// ensureRuntime sets up llama.cpp when it isn't installed yet, before a
// model starts (#610). Onboarding installs it, but a model added another
// way, such as From a link, could reach Start with no runtime, and Start
// then failed with nothing on screen. One setup runs at a time; a caller
// that arrives during one waits for it. A failure is RUNTIME_SETUP_FAILED,
// with why.
func (a *App) ensureRuntime(ctx context.Context, modelID string) error {
	if a.Runtimes == nil || a.stubInference {
		return nil
	}
	rt, err := a.Runtimes.Get("llamacpp")
	if err != nil {
		return nil
	}
	// Every chat turn passes here, so the check is a stat where the
	// runtime offers one, not a full Detect.
	installed := func() bool {
		if c, ok := rt.(interface{ Installed() bool }); ok {
			return c.Installed()
		}
		det, err := rt.Detect(ctx)
		return err == nil && det.Installed
	}
	if installed() {
		return nil
	}
	a.runtimeSetup.Lock()
	defer a.runtimeSetup.Unlock()
	// Another caller may have set it up while this one waited.
	if installed() {
		return nil
	}
	if a.Logger != nil {
		a.Logger.Info("setting up llama.cpp to start a model", "model", modelID)
	}
	// The download goes on if the person leaves the page; the next Start
	// finds it done.
	setup, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimeSetupFor)
	defer cancel()
	started := time.Now()
	if err := a.Runtimes.Install(setup, "llamacpp", runtimes.InstallOptions{}); err != nil {
		if a.Logger != nil {
			a.Logger.Warn("llama.cpp setup failed", "model", modelID, "error", err)
		}
		return contracts.NewError("RUNTIME_SETUP_FAILED", map[string]any{"runtime": "llama.cpp", "reason": err.Error()},
			errors.New("llama.cpp couldn't be set up: "+err.Error()))
	}
	if a.Logger != nil {
		a.Logger.Info("llama.cpp set up", "model", modelID, "seconds", int(time.Since(started).Seconds()))
	}
	return nil
}
