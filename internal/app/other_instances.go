package app

import (
	"os"
	"time"

	"github.com/yeixio/toskar-core/internal/instances"
	"github.com/yeixio/toskar-core/internal/version"
)

// announce writes this Toskar's entry in the machine's shared folder, so
// another Toskar here can say so (#602). It's written again when access
// from anywhere is turned on or off.
func (a *App) announce() {
	if a.instances == nil || a.Config == nil {
		return
	}
	cfg := a.Config.Get()
	a.startedOnce.Do(func() { a.started = time.Now().UTC() })
	err := a.instances.Announce(instances.Info{
		PID: os.Getpid(), Name: cfg.NodeName, DataDir: cfg.DataDir, API: cfg.APIPort,
		Version: version.Version, Started: a.started, Remote: cfg.RemoteAccess.Enabled,
	})
	if err != nil && a.Logger != nil {
		a.Logger.Debug("could not announce this Toskar to others on this computer", "dir", a.instances.Dir, "error", err)
	}
}

// otherInstances are the other Toskars running on this computer now.
func (a *App) otherInstances() []instances.Info {
	if a.instances == nil {
		return nil
	}
	return a.instances.Others(os.Getpid())
}
