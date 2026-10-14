package instances

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOthers(t *testing.T) {
	alive := map[int]bool{100: true, 200: true, 300: false}
	r := &Registry{Dir: filepath.Join(t.TempDir(), "instances"), alive: func(pid int) bool { return alive[pid] }}
	now := time.Now()
	for _, info := range []Info{
		{PID: 100, Name: "linix-rig", DataDir: "/var/lib/yggdrasil", API: 7331, Version: "1.8.3", Started: now.Add(-time.Hour), Remote: true},
		{PID: 200, Name: "linix-rig", DataDir: "/home/mike/.local/share/Toskar", API: 7341, Version: "1.8.2", Started: now},
		{PID: 300, Name: "gone", Started: now},
	} {
		if err := r.Announce(info); err != nil {
			t.Fatal(err)
		}
	}
	// This Toskar (100) sees the other one that runs, not itself or the one that's gone.
	got := r.Others(100)
	if len(got) != 1 || got[0].PID != 200 || got[0].API != 7341 || got[0].DataDir != "/home/mike/.local/share/Toskar" {
		t.Fatalf("others = %+v", got)
	}
	if _, err := os.Stat(r.path(300)); !os.IsNotExist(err) {
		t.Fatal("a gone process's file was kept")
	}
	// Oldest first, and a withdrawn file is gone.
	if got := r.Others(999); len(got) != 2 || got[0].PID != 100 {
		t.Fatalf("from outside: %+v", got)
	}
	r.Withdraw(200)
	if got := r.Others(100); len(got) != 0 {
		t.Fatalf("after withdraw: %+v", got)
	}
	// A missing folder is no others.
	if (&Registry{Dir: filepath.Join(t.TempDir(), "none")}).Others(1) != nil {
		t.Fatal("missing folder")
	}
}

func TestThisProcessIsAlive(t *testing.T) {
	if !processAlive(os.Getpid()) || processAlive(-1) {
		t.Fatal("alive")
	}
}
