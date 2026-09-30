package training

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fakeSpec(t *testing.T) RunSpec {
	t.Helper()
	dir := t.TempDir()
	return RunSpec{WorkDir: dir, LogPath: filepath.Join(dir, "train.log"), AdapterOut: filepath.Join(dir, "adapter.gguf"),
		Hyper: Hyper{Iters: 10, Epochs: 2}}
}

func writeScript(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "fake.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunScriptTurnsEventsIntoUpdates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script trainer")
	}
	spec := fakeSpec(t)
	script := writeScript(t, spec.WorkDir, `
echo "Loading pretrained model"
echo '@@ygg {"event":"stage","stage":"loading_model","detail":"Downloading base weights"}'
echo '@@ygg {"event":"download","bytes":50,"total":100}'
echo '@@ygg {"event":"stage","stage":"training","detail":"Training"}'
echo '@@ygg {"event":"progress","iter":5,"iters":10,"train_loss":1.25,"tokens_per_sec":900,"peak_memory_gb":1.6}'
echo '@@ygg {"event":"val","iter":5,"val_loss":1.5}'
echo '@@ygg {"event":"stage","stage":"exporting","detail":"Writing the adapter"}'
echo '@@ygg {"event":"done","adapter":"`+spec.AdapterOut+`","train_loss":0.5,"val_loss":0.75}'
`)
	var updates []Update
	res, err := runScript(context.Background(), spec, []string{script}, func(u Update) { updates = append(updates, u) })
	if err != nil {
		t.Fatal(err)
	}
	if res.AdapterPath != spec.AdapterOut || *res.TrainLoss != 0.5 || *res.ValLoss != 0.75 {
		t.Fatalf("result = %+v", res)
	}
	var states []string
	for _, u := range updates {
		states = append(states, string(u.State))
	}
	want := "loading_model,loading_model,training,training,training,exporting"
	if strings.Join(states, ",") != want {
		t.Fatalf("states = %v", states)
	}
	train := updates[3].Progress
	if train.Iter != 5 || *train.TrainLoss != 1.25 || train.Epoch != 1 || train.PeakMemoryGB != 1.6 {
		t.Fatalf("progress = %+v", train)
	}
	if updates[1].Progress.DownloadBytes != 50 || updates[1].Progress.DownloadTotal != 100 {
		t.Fatalf("download = %+v", updates[1].Progress)
	}
	log, _ := os.ReadFile(spec.LogPath)
	if !strings.Contains(string(log), "Loading pretrained model") || strings.Contains(string(log), "@@ygg") {
		t.Fatalf("log = %q", log)
	}
}

func TestRunScriptReportsTheScriptError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script trainer")
	}
	spec := fakeSpec(t)
	script := writeScript(t, spec.WorkDir, `
echo '@@ygg {"event":"error","message":"OSError: repository not found"}'
exit 1
`)
	_, err := runScript(context.Background(), spec, []string{script}, func(Update) {})
	if err == nil || !strings.Contains(err.Error(), "repository not found") || !strings.Contains(err.Error(), spec.LogPath) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunScriptWithoutAdapterFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script trainer")
	}
	spec := fakeSpec(t)
	script := writeScript(t, spec.WorkDir, "exit 0\n")
	if _, err := runScript(context.Background(), spec, []string{script}, func(Update) {}); err == nil {
		t.Fatal("a trainer that wrote no adapter must fail")
	}
}

func TestCancelStopsTheWholeProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups")
	}
	spec := fakeSpec(t)
	pidFile := filepath.Join(spec.WorkDir, "child.pid")
	// The trainer starts a worker, as data loaders do, then waits.
	script := writeScript(t, spec.WorkDir, `
sleep 60 &
echo $! > `+pidFile+`
echo '@@ygg {"event":"stage","stage":"training","detail":"Training"}'
wait
`)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := runScript(ctx, spec, []string{script}, func(u Update) {
			if u.State == StateTraining {
				started <- struct{}{}
			}
		})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("trainer did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop the trainer")
	}
	raw, _ := os.ReadFile(pidFile)
	var pid int
	for _, c := range strings.TrimSpace(string(raw)) {
		pid = pid*10 + int(c-'0')
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = exec.Command("kill", "-9", strings.TrimSpace(string(raw))).Run()
	t.Fatal("the trainer's child process survived cancel")
}

func TestRateTrackerWaitsForASteadyRate(t *testing.T) {
	var r rateTracker
	start := time.Unix(1000, 0)
	if r.remaining(start, 1, 100) != nil {
		t.Fatal("first report has no rate")
	}
	if r.remaining(start.Add(2*time.Second), 2, 100) != nil {
		t.Fatal("too early to trust")
	}
	got := r.remaining(start.Add(10*time.Second), 11, 100)
	if got == nil || *got != 89 {
		t.Fatalf("remaining = %v", got)
	}
}
