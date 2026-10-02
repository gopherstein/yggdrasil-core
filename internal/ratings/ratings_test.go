package ratings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeixio/yggdrasil-core/internal/store"
	"github.com/yeixio/yggdrasil-core/internal/store/repositories"
	"github.com/yeixio/yggdrasil-core/pkg/contracts"
)

func TestIdentify(t *testing.T) {
	cases := []struct {
		m    contracts.Model
		want Identity
	}{
		{contracts.Model{Variant: "Q4_K_M", Source: contracts.ModelSource{URL: "https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/qwen2.5-coder-7b-instruct-q4_k_m.gguf"}},
			Identity{ID: "qwen2.5-coder-7b-instruct", Format: "gguf", Quantization: "Q4_K_M", Runtime: "llamacpp", Backend: "metal"}},
		// Another uploader's conversion is the same model.
		{contracts.Model{Source: contracts.ModelSource{URL: "https://huggingface.co/bartowski/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/Qwen2.5-Coder-7B-Instruct-IQ4_XS.gguf", Format: "gguf"}},
			Identity{ID: "qwen2.5-coder-7b-instruct", Format: "gguf", Quantization: "IQ4_XS", Runtime: "llamacpp", Backend: "metal"}},
		{contracts.Model{Variant: "e2e", Runtime: []string{"llamacpp"}, Source: contracts.ModelSource{URL: "https://huggingface.co/x/Phi-3.5-mini-instruct-GGUF/resolve/main/Phi-3.5-mini-instruct-Q8_0.gguf"}},
			Identity{ID: "phi-3.5-mini-instruct", Format: "gguf", Quantization: "Q8_0", Runtime: "llamacpp", Backend: "metal"}},
	}
	for _, c := range cases {
		got, err := Identify(c.m, "metal")
		if err != nil || got != c.want {
			t.Errorf("Identify(%s) = %+v, %v; want %+v", c.m.Source.URL, got, err, c.want)
		}
	}
	for _, u := range []string{"", "https://example.com/Qwen/Qwen-GGUF/resolve/main/q4_k_m.gguf", "https://huggingface.co/a/b-GGUF/resolve/main/model.gguf"} {
		if _, err := Identify(contracts.Model{Source: contracts.ModelSource{URL: u}}, "cpu"); !errors.Is(err, ErrUnknownModel) {
			t.Errorf("Identify(%q) err = %v, want ErrUnknownModel", u, err)
		}
	}
}

const gb = 1 << 30

func TestNormalize(t *testing.T) {
	cases := []struct {
		inv  contracts.HardwareInventory
		want Hardware
	}{
		{contracts.HardwareInventory{OS: "darwin", Arch: "arm64", CPU: contracts.CPUInfo{Model: "Apple M4 Max"},
			Accelerators: []contracts.Accelerator{{Vendor: "Apple", Model: "Apple M4 Max", UnifiedMemory: 36 * gb}}},
			Hardware{"macos", "arm64", "apple", "m4-max", "unified", "32-64"}},
		{contracts.HardwareInventory{OS: "linux", Arch: "amd64", Memory: contracts.MemoryInfo{TotalBytes: 64 * gb},
			Accelerators: []contracts.Accelerator{{Vendor: "NVIDIA", Model: "NVIDIA GeForce RTX 3060", DedicatedVRAM: 12 * gb}, {Vendor: "NVIDIA", Model: "NVIDIA GeForce RTX 4090", DedicatedVRAM: 24564 << 20}}},
			Hardware{"linux", "amd64", "nvidia", "rtx-4090", "dedicated", "16-32"}},
		{contracts.HardwareInventory{OS: "linux", Arch: "amd64",
			Accelerators: []contracts.Accelerator{{Vendor: "AMD", Model: "Advanced Micro Devices, Inc. [AMD/ATI] Navi 31 [Radeon RX 7900 XT/7900 XTX] (rev c8)", DedicatedVRAM: 8188 << 20}}},
			Hardware{"linux", "amd64", "amd", "rx-7900-xt", "dedicated", "8-16"}},
		{contracts.HardwareInventory{OS: "windows", Arch: "amd64", CPU: contracts.CPUInfo{Model: "AMD Ryzen 9 7950X"}, Memory: contracts.MemoryInfo{TotalBytes: 32 * gb},
			Accelerators: []contracts.Accelerator{{Vendor: "Generic", Model: "CPU inference"}}},
			Hardware{"windows", "amd64", "cpu", "amd", "system", "32-64"}},
	}
	for _, c := range cases {
		got, ok := Normalize(c.inv)
		if !ok || got != c.want {
			t.Errorf("Normalize = %+v, %v; want %+v", got, ok, c.want)
		}
	}
	if _, ok := Normalize(contracts.HardwareInventory{OS: "freebsd", Arch: "amd64"}); ok {
		t.Error("an unknown platform was normalized")
	}
	if Class("nvidia", "rtx-4090") != "rtx-40" || Class("apple", "m4-max") != "apple-silicon" || Class("amd", "rx-7900-xt") != "rx-7000" {
		t.Error("Class does not match the ratings service")
	}
}

// fakeService is the ratings service.
type fakeService struct {
	mu      sync.Mutex
	posts   []Rating
	deletes []string
	snap    Snapshot
	down    bool
}

func (f *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		http.Error(w, `{"error":"down"}`, http.StatusServiceUnavailable)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/ratings":
		var rt Rating
		_ = json.NewDecoder(r.Body).Decode(&rt)
		f.posts = append(f.posts, rt)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"key":"k1","created":true}`))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/ratings/"):
		f.deletes = append(f.deletes, strings.TrimPrefix(r.URL.Path, "/v1/ratings/")+" "+r.Header.Get("X-Ratings-Client"))
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/v1/aggregates" || r.URL.Path == "/summary.json":
		_ = json.NewEncoder(w).Encode(f.snap)
	default:
		http.NotFound(w, r)
	}
}

func newService(t *testing.T, srv *httptest.Server) (*Service, *repositories.SettingsRepo, *[]string) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settings := repositories.NewSettingsRepo(db.SQL)
	var records []string
	s := &Service{
		DB: db.SQL, Settings: settings,
		Client: &Client{ServiceURL: srv.URL, SummaryURL: srv.URL + "/summary.json", HTTP: srv.Client()},
		Hardware: func(context.Context) (contracts.HardwareInventory, error) {
			return contracts.HardwareInventory{OS: "darwin", Arch: "arm64",
				Accelerators: []contracts.Accelerator{{Vendor: "Apple", Model: "Apple M4 Max", UnifiedMemory: 36 * gb}}}, nil
		},
		Models: func(context.Context) ([]contracts.Model, error) {
			return []contracts.Model{
				{ID: "qwen2.5-coder-7b-q4", Variant: "Q4_K_M", Installed: true, Source: contracts.ModelSource{URL: "https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/qwen2.5-coder-7b-instruct-q4_k_m.gguf"}},
				{ID: "local-file", Installed: true, Source: contracts.ModelSource{URL: "https://example.com/m.gguf"}},
			}, nil
		},
		Record:     func(_ context.Context, dest, detail string) { records = append(records, dest+": "+detail) },
		AppVersion: "0.9.0",
	}
	return s, settings, &records
}

func TestRateShareAndWithdraw(t *testing.T) {
	fake := &fakeService{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s, _, records := newService(t, srv)
	ctx := context.Background()

	v, err := s.Get(ctx, "qwen2.5-coder-7b-q4")
	if err != nil || !v.Rateable || v.Shares == nil || v.Shares.Hardware.Key() != "apple:m4-max:unified:32-64" || v.Stars != 0 {
		t.Fatalf("Get = %+v, %v", v, err)
	}
	// Kept on this computer: nothing is sent.
	if _, err := s.Put(ctx, "qwen2.5-coder-7b-q4", Input{Stars: 4, Tags: []string{"fast"}}); err != nil {
		t.Fatal(err)
	}
	if len(fake.posts) != 0 || len(*records) != 0 {
		t.Fatalf("a private rating was sent: %v %v", fake.posts, *records)
	}
	if _, err := s.Put(ctx, "qwen2.5-coder-7b-q4", Input{Stars: 7}); !errors.Is(err, ErrStars) {
		t.Fatalf("7 stars err = %v", err)
	}
	v, err = s.Put(ctx, "qwen2.5-coder-7b-q4", Input{Stars: 5, Tags: []string{"fast", "stable"}, Share: true})
	if err != nil || !v.Shared || v.Stars != 5 {
		t.Fatalf("share = %+v, %v", v, err)
	}
	p := fake.posts[0]
	if p.Model.ID != "qwen2.5-coder-7b-instruct" || p.Runtime.Backend != "metal" || p.Hardware.Family != "m4-max" || len(p.ClientID) != 32 || p.AppVersion != "0.9.0" {
		t.Fatalf("posted %+v", p)
	}
	if len(*records) != 1 || !strings.Contains((*records)[0], "5-star rating of qwen2.5-coder-7b-instruct") {
		t.Fatalf("records = %v", *records)
	}
	// Keeping it private again withdraws it.
	v, err = s.Put(ctx, "qwen2.5-coder-7b-q4", Input{Stars: 3})
	if err != nil || v.Shared || len(fake.deletes) != 1 || fake.deletes[0] != "k1 "+p.ClientID {
		t.Fatalf("withdraw = %+v, %v, %v", v, err, fake.deletes)
	}
	// A model that cannot be compared is rated here only.
	v, _ = s.Get(ctx, "local-file")
	if v.Rateable || v.Reason == "" {
		t.Fatalf("local-file = %+v", v)
	}
	if _, err := s.Put(ctx, "local-file", Input{Stars: 2, Share: true}); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("share local-file err = %v", err)
	}
	// The service down: saved here, and the error says so.
	if _, err := s.Put(ctx, "qwen2.5-coder-7b-q4", Input{Stars: 4, Share: true}); err != nil {
		t.Fatal(err)
	}
	fake.down = true
	var se *ShareError
	if err := s.Delete(ctx, "qwen2.5-coder-7b-q4"); !errors.As(err, &se) {
		t.Fatalf("delete with the service down err = %v", err)
	}
	fake.down = false
	if err := s.Delete(ctx, "qwen2.5-coder-7b-q4"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(ctx, "qwen2.5-coder-7b-q4"); v.Stars != 0 {
		t.Fatalf("deleted rating remains: %+v", v)
	}
}

func TestAsk(t *testing.T) {
	srv := httptest.NewServer(&fakeService{})
	defer srv.Close()
	s, settings, _ := newService(t, srv)
	ctx := context.Background()
	add := func(day string, n int) {
		for i := range n {
			if _, err := s.DB.Exec(`INSERT INTO generation_metrics (id, model_id, created_at) VALUES (?, 'qwen2.5-coder-7b-q4', ?)`, fmt.Sprintf("%s-%d", day, i), day+"T10:00:00Z"); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("2026-09-01", 9)
	if v, _ := s.Get(ctx, "qwen2.5-coder-7b-q4"); v.Ask {
		t.Fatal("asked after one day")
	}
	add("2026-09-02", 1)
	if v, _ := s.Get(ctx, "qwen2.5-coder-7b-q4"); !v.Ask {
		t.Fatal("not asked after 10 replies on 2 days")
	}
	_ = settings.SetBool(ctx, SettingAsk, false)
	if v, _ := s.Get(ctx, "qwen2.5-coder-7b-q4"); v.Ask {
		t.Fatal("asked with asking off")
	}
	_ = settings.SetBool(ctx, SettingAsk, true)
	if err := s.Dismiss(ctx, "qwen2.5-coder-7b-q4"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(ctx, "qwen2.5-coder-7b-q4"); v.Ask {
		t.Fatal("asked after dismissing")
	}
}

func TestCommunity(t *testing.T) {
	fake := &fakeService{}
	if err := json.Unmarshal([]byte(`{"schema_version":1,"generated_at":"2026-10-01T04:17:00Z","prior":3.5,"weight":5,"min_ratings":3,"models":[
		{"model":"qwen2.5-coder-7b-instruct","format":"gguf","quantization":"Q4_K_M","runtime":"llamacpp","backend":"metal","cohorts":[
			{"tier":"class","cohort":"apple:apple-silicon:32-64","ratings":12,"average":4.5,"weighted_score":4.2,"confidence":"community"},
			{"tier":"backend","cohort":"metal:32-64","ratings":20,"average":4.4,"weighted_score":4.2,"confidence":"community"},
			{"tier":"global","ratings":40,"average":4.1,"weighted_score":4.0,"confidence":"community"}]},
		{"model":"qwen2.5-coder-7b-instruct","format":"gguf","quantization":"Q4_K_M","runtime":"llamacpp","backend":"cpu","cohorts":[
			{"tier":"global","ratings":3,"average":2,"weighted_score":3,"confidence":"early"}]}]}`), &fake.snap); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s, settings, records := newService(t, srv)
	ctx := context.Background()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }

	// Off by default: nothing is downloaded.
	c, err := s.Community(ctx)
	if err != nil || c.Enabled || len(*records) != 0 {
		t.Fatalf("Community off = %+v, %v, %v", c, err, *records)
	}
	_ = settings.SetBool(ctx, SettingShow, true)
	c, err = s.Community(ctx)
	if err != nil || !c.Enabled || c.FetchedAt == nil {
		t.Fatalf("Community = %+v, %v", c, err)
	}
	mc := c.Models["qwen2.5-coder-7b-q4"]
	if mc.Similar == nil || mc.Similar.Tier != "class" || mc.Similar.Ratings != 12 || mc.Overall == nil || mc.Overall.Ratings != 40 {
		t.Fatalf("model = %+v", mc)
	}
	if len(*records) != 1 {
		t.Fatalf("records = %v", *records)
	}
	// Within a day, the kept summary is used; when the service is down
	// after that, the dataset is.
	_, _ = s.Community(ctx)
	if len(*records) != 1 {
		t.Fatalf("downloaded again within a day: %v", *records)
	}
	now = now.Add(25 * time.Hour)
	c, err = s.Community(ctx)
	if err != nil || !strings.HasSuffix(c.Source, "/v1/aggregates") {
		t.Fatalf("refresh = %+v, %v", c, err)
	}
	fake.down = true
	now = now.Add(25 * time.Hour)
	c, err = s.Community(ctx)
	if err != nil || c.Error == "" || c.Models["qwen2.5-coder-7b-q4"].Overall == nil {
		t.Fatalf("offline = %+v, %v", c, err)
	}
}
