package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type fakeStop struct {
	mu       sync.Mutex
	alive    bool
	graceful func(context.Context) error
	forced   int
	stopped  int
}

func (f *fakeStop) Graceful(ctx context.Context, _ Instance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped++
	if f.graceful != nil {
		return f.graceful(ctx)
	}
	f.alive = false
	return nil
}

func (f *fakeStop) Force(context.Context, Instance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forced++
	f.alive = false
	return nil
}

func (f *fakeStop) Alive(Instance) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.alive
}

func testMonitor(stop *fakeStop, probe func(context.Context, string) error) *Monitor {
	settings := DefaultSettings()
	settings.ProbeFailures = 3
	m := NewMonitor(settings)
	m.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	m.Stopper = stop
	m.Probe = probe
	m.Now = func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }
	return m
}

func inst() Instance {
	return Instance{ModelID: "qwen", RunningModelID: "run-1", NodeID: "local", RuntimeID: "llamacpp", Endpoint: "http://127.0.0.1:9"}
}

func TestProcessExitDuringGenerationCleansUp(t *testing.T) {
	stop := &fakeStop{alive: true}
	m := testMonitor(stop, func(context.Context, string) error { return nil })
	m.Register(inst())
	m.BeginGeneration("run-1")
	m.NoteProgress("run-1")
	m.ReportProcessExit("run-1", 137, "signal")

	if m.Status("run-1") != StatusStopped {
		t.Fatalf("status %s", m.Status("run-1"))
	}
	if m.Accepting("run-1") {
		t.Fatal("still accepting work")
	}
	fail, ok := m.FailureOf("run-1")
	if !ok || fail.Reason != ReasonProcessExit || fail.ExitCode == nil || *fail.ExitCode != 137 {
		t.Fatalf("failure %+v ok=%v", fail, ok)
	}
	if stop.stopped == 0 && stop.forced == 0 {
		t.Fatal("expected a stop attempt")
	}
	if fail.Message != UserMessage(false) {
		t.Fatal(fail.Message)
	}
}

func TestThreeProbeFailuresMarkUnresponsive(t *testing.T) {
	stop := &fakeStop{alive: true}
	m := testMonitor(stop, func(context.Context, string) error { return errors.New("down") })
	m.Register(inst())
	m.ProbeOnce("run-1")
	m.ProbeOnce("run-1")
	if m.Status("run-1") != StatusDegraded {
		t.Fatalf("two failures should degrade, got %s", m.Status("run-1"))
	}
	if !m.Accepting("run-1") {
		t.Fatal("degraded model should still accept work")
	}
	m.ProbeOnce("run-1")
	if m.Status("run-1") != StatusStopped {
		t.Fatalf("third failure status %s", m.Status("run-1"))
	}
	fail, ok := m.FailureOf("run-1")
	if !ok || fail.Reason != ReasonHealthProbeFailed {
		t.Fatalf("%+v", fail)
	}
}

func TestOneProbeFailureDoesNotKill(t *testing.T) {
	stop := &fakeStop{alive: true}
	calls := 0
	m := testMonitor(stop, func(context.Context, string) error {
		calls++
		if calls == 1 {
			return errors.New("blip")
		}
		return nil
	})
	m.Register(inst())
	m.ProbeOnce("run-1")
	m.ProbeOnce("run-1")
	if m.Status("run-1") != StatusHealthy {
		t.Fatalf("recovered status %s", m.Status("run-1"))
	}
	if stop.stopped != 0 || stop.forced != 0 {
		t.Fatal("transient probe must not stop the model")
	}
}

func TestStalledGenerationAndFailedProbe(t *testing.T) {
	stop := &fakeStop{alive: true}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	m := testMonitor(stop, func(context.Context, string) error { return errors.New("down") })
	m.Now = func() time.Time { return now }
	m.Register(inst())
	m.BeginGeneration("run-1")
	m.NoteProgress("run-1")
	now = now.Add(31 * time.Second)
	fail, ok := m.CheckGeneration("run-1")
	if !ok || fail.Reason != ReasonGenerationStalled {
		t.Fatalf("%+v ok=%v", fail, ok)
	}
	if m.Status("run-1") != StatusStopped {
		t.Fatalf("status %s", m.Status("run-1"))
	}
}

func TestSlowGenerationWithHealthyProbeStaysUp(t *testing.T) {
	stop := &fakeStop{alive: true}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	m := testMonitor(stop, func(context.Context, string) error { return nil })
	m.Now = func() time.Time { return now }
	m.Register(inst())
	m.BeginGeneration("run-1")
	m.NoteProgress("run-1")
	now = now.Add(31 * time.Second)
	if _, ok := m.CheckGeneration("run-1"); ok {
		t.Fatal("healthy runtime must not be killed for a slow stretch")
	}
	if stop.stopped != 0 {
		t.Fatal("unexpected stop")
	}
}

func TestGracefulStopDoesNotForceKill(t *testing.T) {
	stop := &fakeStop{alive: true}
	m := testMonitor(stop, func(context.Context, string) error { return nil })
	m.Register(inst())
	m.ReportProcessExit("run-1", 1, "")
	if stop.stopped != 1 || stop.forced != 0 {
		t.Fatalf("graceful=%d force=%d", stop.stopped, stop.forced)
	}
}

func TestForceKillWhenGracefulTimesOut(t *testing.T) {
	stop := &fakeStop{alive: true, graceful: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	m := testMonitor(stop, nil)
	m.Settings.GracefulStopWait = 15 * time.Millisecond
	m.Settings.ForceStopWait = 15 * time.Millisecond
	m.Register(inst())
	m.ReportProcessExit("run-1", 1, "")
	if stop.forced != 1 {
		t.Fatalf("force=%d", stop.forced)
	}
	if stop.Alive(inst()) {
		t.Fatal("process still alive")
	}
}

func TestPartialProgressIsMarkedInterrupted(t *testing.T) {
	m := testMonitor(&fakeStop{alive: true}, nil)
	m.Register(inst())
	m.BeginGeneration("run-1")
	m.NoteProgress("run-1")
	m.ReportProcessExit("run-1", 1, "")
	fail, ok := m.FailureOf("run-1")
	if !ok || !fail.Interrupted {
		t.Fatalf("%+v", fail)
	}
	raw := Encode(fail)
	parsed, ok := Parse(raw)
	if !ok || !parsed.Interrupted || parsed.Message == "" {
		t.Fatalf("parsed %+v", parsed)
	}
}

func TestRepeatedFailuresFlagNode(t *testing.T) {
	m := testMonitor(&fakeStop{alive: true}, nil)
	first := inst()
	m.Register(first)
	m.ReportProcessExit(first.RunningModelID, 1, "")
	second := first
	second.RunningModelID = "run-2"
	m.Register(second)
	m.ReportProcessExit(second.RunningModelID, 1, "")
	nodes := m.UnstableNodes("qwen")
	if len(nodes) != 1 || nodes[0] != "local" {
		t.Fatalf("nodes %v", nodes)
	}
}

func TestMemoryWording(t *testing.T) {
	m := testMonitor(&fakeStop{alive: false}, nil)
	m.Memory = func(context.Context) bool { return true }
	m.Register(inst())
	m.ReportProcessExit("run-1", 9, "")
	fail, _ := m.FailureOf("run-1")
	if fail.Reason != ReasonOOM || !fail.LikelyMemoryPressure {
		t.Fatalf("%+v", fail)
	}
	if fail.Message != UserMessage(true) {
		t.Fatal(fail.Message)
	}
}

func TestComputeErrorStopsTheModel(t *testing.T) {
	m := testMonitor(&fakeStop{alive: true}, nil)
	m.Register(inst())
	m.BeginGeneration("run-1")
	if !m.ReportRuntimeError("run-1", "Compute error.") {
		t.Fatal("compute error should be fatal")
	}
	fail, ok := m.FailureOf("run-1")
	if !ok || !fail.LikelyMemoryPressure || fail.Reason != ReasonOOM {
		t.Fatalf("%+v ok=%v", fail, ok)
	}
	if m.Accepting("run-1") {
		t.Fatal("failed model still accepting work")
	}
}

func TestIntentionalStopIsNotAFailure(t *testing.T) {
	stop := &fakeStop{alive: true}
	m := testMonitor(stop, nil)
	m.Register(inst())
	m.MarkIntentional("run-1")
	m.ReportProcessExit("run-1", 0, "")
	if _, ok := m.FailureOf("run-1"); ok {
		t.Fatal("intentional stop should not be a health failure")
	}
	if stop.stopped != 0 {
		t.Fatal("monitor must not stop an intentional unload")
	}
}

// Health events say why the process died: its exit code and the end of
// what it wrote, or the last probe's error (#9).
func TestHealthEventsCarryStderrAndProbe(t *testing.T) {
	var mu sync.Mutex
	var events []Event
	record := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}
	last := func(kind string) map[string]any {
		mu.Lock()
		defer mu.Unlock()
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Type == kind {
				return events[i].Payload
			}
		}
		return nil
	}

	exited := testMonitor(&fakeStop{alive: true}, func(context.Context, string) error { return nil })
	exited.Publish = record
	exited.Register(inst())
	exited.ReportProcessExit("run-1", 1, "error: failed to load model '/m/qwen.gguf'")
	p := last(EventCleanupStarted)
	if p == nil || p["exit_code"] != 1 || p["stderr_tail"] != "error: failed to load model '/m/qwen.gguf'" || p["reason"] != ReasonProcessExit {
		t.Fatalf("process exit payload = %v", p)
	}

	probed := testMonitor(&fakeStop{alive: true}, func(context.Context, string) error { return errors.New("dial tcp 127.0.0.1:9: connection refused") })
	probed.Publish = record
	probed.Register(inst())
	probed.ProbeOnce("run-1")
	probed.ProbeOnce("run-1")
	p = last(EventDegraded)
	if p == nil || p["last_probe"] != "dial tcp 127.0.0.1:9: connection refused" || p["reason"] != ReasonHealthProbeFailed {
		t.Fatalf("probe payload = %v", p)
	}
}
