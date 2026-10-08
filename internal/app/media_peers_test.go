package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yeixio/toskar-core/internal/imagegen"
	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

func place(remote, accelerated, tight bool, free uint64) setupPlace {
	return setupPlace{remote: remote, nodeName: map[bool]string{true: "Workstation", false: "Laptop"}[remote], status: MediaSetupStatus{
		Status: imagegen.Status{Supported: true, Accelerated: accelerated, Models: []imagegen.ModelStatus{
			{Model: imagegen.Model{ID: "flux"}, SizeBytes: 5e9, Recommended: true, TightMemory: tight},
			{Model: imagegen.Model{ID: "big"}, SizeBytes: 20e9},
		}},
		FreeBytes: free,
	}}
}

// The setup goes where a GPU would do the work, then where there's memory
// to spare, and to this computer when they're equal (#153).
func TestBestSetupPlace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		places []setupPlace
		want   string
	}{
		{"a GPU elsewhere beats this computer's CPU", []setupPlace{place(false, false, false, 0), place(true, true, false, 100e9)}, "Workstation"},
		{"equal: this computer", []setupPlace{place(false, true, false, 0), place(true, true, false, 100e9)}, "Laptop"},
		{"memory to spare beats tight", []setupPlace{place(false, true, true, 0), place(true, true, false, 0)}, "Workstation"},
		{"no room for the download", []setupPlace{place(false, false, false, 0), place(true, true, false, 5.5e9)}, "Laptop"},
	} {
		got, ok := bestSetupPlace(tc.places)
		if !ok || got.nodeName != tc.want || got.model.ID != "flux" {
			t.Errorf("%s: got %q %v", tc.name, got.nodeName, ok)
		}
	}

	ready := place(true, true, false, 0)
	ready.status.Ready = true
	if _, ok := bestSetupPlace([]setupPlace{place(false, false, false, 0), ready}); ok {
		t.Error("offered a setup that's ready on another computer")
	}
	small := place(true, true, false, 0)
	small.status.Models[0].TooLittleMemory = true
	if _, ok := bestSetupPlace([]setupPlace{small}); ok {
		t.Error("offered a computer without the memory")
	}
}

// fakePeer is a paired computer's internal API for image setup.
type fakePeer struct {
	mu      sync.Mutex
	started string
	status  MediaSetupStatus
}

func (p *fakePeer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch r.Method + " " + r.URL.Path {
	case "GET /internal/v1/health":
		writeJSONStatus(w, http.StatusOK, map[string]string{"status": "ok"})
	case "GET /internal/v1/media/images/setup":
		writeJSONStatus(w, http.StatusOK, p.status)
	case "POST /internal/v1/media/images/setup":
		var body struct {
			ModelID string `json:"model_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		p.started = body.ModelID
		p.status.Job = &imagegen.Job{ModelID: body.ModelID, Stage: "model", Total: 5e9, Running: true}
		writeJSONStatus(w, http.StatusAccepted, p.status)
	default:
		// An older Toskar, or a computer without video.
		http.NotFound(w, r)
	}
}

// A laptop that can't make pictures well is offered the setup on its
// paired workstation, which it starts and follows there (#153).
func TestOfferSetupOnAPairedComputer(t *testing.T) {
	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	ctx := context.Background()
	// This computer can't set it up; the workstation can, on its GPU.
	a.Images = nil
	peer := &fakePeer{status: MediaSetupStatus{FreeBytes: 120e9, Status: imagegen.Status{Supported: true, Accelerated: true,
		Models: []imagegen.ModelStatus{{Model: imagegen.Model{ID: "flux-test", Name: "FLUX test"}, SizeBytes: 5.2e9, Recommended: true}}}}}
	srv := httptest.NewServer(peer)
	defer srv.Close()
	if _, err := a.DB.SQL.Exec(`INSERT INTO nodes (id, name, status, is_local, address) VALUES ('ws', 'Workstation', 'online', 0, ?)`, strings.TrimPrefix(srv.URL, "http://")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.SQL.Exec(`INSERT INTO node_trust (node_id, fingerprint) VALUES ('ws', 'fp')`); err != nil {
		t.Fatal(err)
	}

	conv, _ := a.Conversations.Create(ctx, "t", "", "")
	allow := profiles.Profile{Tools: []contracts.ToolPolicy{{ToolID: "image.generate", Policy: "allow"}, {ToolID: "image.edit", Policy: "allow"}}}
	ch, ok := a.offerSetup(ctx, allow, conv.ID, "Make me a picture of a Viking tree")
	if !ok {
		t.Fatal("no setup offered")
	}
	var reply string
	for c := range ch {
		reply += c.Content
	}
	for _, want := range []string{"set it up on Workstation: FLUX test, 5.2 GB to download.", "Its GPU will do the work.", "Workstation has 120 GB free.", "finish this on Workstation once it's ready"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply lacks %q: %q", want, reply)
		}
	}
	msgs, _ := a.Conversations.ListMessages(ctx, conv.ID)
	offer := msgs[len(msgs)-1].Meta.Setup
	if offer == nil || !offer.Remote || offer.NodeID != "ws" || offer.NodeName != "Workstation" || offer.FreeBytes != 120e9 || offer.Option != "flux-test" || offer.Slow {
		t.Fatalf("offer = %+v", offer)
	}

	// The card starts it there, and follows it.
	status, raw, handled, err := a.RemoteMediaSetup(ctx, "ws", "images", http.MethodPost, []byte(`{"model_id":"flux-test"}`))
	if err != nil || !handled || status != http.StatusAccepted || peer.started != "flux-test" {
		t.Fatalf("start: %d %v %v %q", status, handled, err, peer.started)
	}
	var st MediaSetupStatus
	if json.Unmarshal(raw, &st) != nil || st.Job == nil || !st.Job.Running {
		t.Fatalf("status = %s", raw)
	}
	// This computer's own setup answers for itself.
	if _, _, handled, _ := a.RemoteMediaSetup(ctx, a.Config.Get().NodeID, "images", http.MethodGet, nil); handled {
		t.Fatal("this computer's setup was sent to another")
	}
	// An older Toskar says so.
	if _, _, _, err := a.RemoteMediaSetup(ctx, "ws", "video", http.MethodGet, nil); err == nil || !strings.Contains(err.Error(), "newer Toskar") {
		t.Fatalf("old peer: %v", err)
	}
}

// Another computer can read, start, and stop this computer's setup.
func TestMediaSetupHandler(t *testing.T) {
	a := &App{Images: &imagegen.Setup{ProgramDir: t.TempDir(), ModelsDir: t.TempDir(), FreeBytes: func() (int64, error) { return 9e10, nil }}}
	h := a.mediaSetupHandler()
	get := func(method, path, body string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		raw, _ := io.ReadAll(rec.Body)
		return rec.Code, string(raw)
	}
	if code, body := get(http.MethodGet, "/media/images/setup", ""); code != http.StatusOK || !strings.Contains(body, `"models"`) || !strings.Contains(body, `"free_bytes":90000000000`) {
		t.Fatalf("status: %d %s", code, body)
	}
	if code, _ := get(http.MethodGet, "/media/video/setup", ""); code != http.StatusNotFound {
		t.Fatalf("video without a setup: %d", code)
	}
	if code, body := get(http.MethodPost, "/media/images/setup", `{"model_id":"no-such-model"}`); code != http.StatusConflict || !strings.Contains(body, "error") {
		t.Fatalf("bad model: %d %s", code, body)
	}
	if code, _ := get(http.MethodDelete, "/media/images/setup", ""); code != http.StatusOK {
		t.Fatalf("stop: %d", code)
	}
}
