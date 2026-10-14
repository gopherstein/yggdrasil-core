package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/instances"
)

// This Toskar announces itself, sees another that runs, and isn't seen by
// itself (#602).
func TestAnnounceAndOthers(t *testing.T) {
	cfg, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reg := &instances.Registry{Dir: filepath.Join(t.TempDir(), "instances")}
	a := &App{Config: cfg, instances: reg}
	a.announce()
	if got := a.otherInstances(); len(got) != 0 {
		t.Fatalf("saw itself: %+v", got)
	}
	// The test's parent process stands in for another Toskar.
	other := instances.Info{PID: os.Getppid(), Name: "other", API: 7341, Version: "1.8.2", Started: time.Now()}
	if err := reg.Announce(other); err != nil {
		t.Fatal(err)
	}
	got := a.otherInstances()
	if len(got) != 1 || got[0].Name != "other" || got[0].API != 7341 {
		t.Fatalf("others = %+v", got)
	}
	// Shutdown withdraws it.
	reg.Withdraw(os.Getpid())
	if left := reg.Others(os.Getppid()); len(left) != 0 {
		t.Fatalf("this Toskar's entry stayed: %+v", left)
	}
	// Without a registry, as in tests, nothing is announced or seen.
	if (&App{Config: cfg}).otherInstances() != nil {
		t.Fatal("no registry")
	}
}
