package llamacpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// When no release has a build for this computer, or GitHub won't list
// them, the error says what was looked for, where, and what to do (#64).
func TestReleaseErrorsSayWhatAndWhere(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`[
			{"tag_name":"b6500","assets":[{"name":"llama-b6500-bin-macos-arm64.zip","browser_download_url":"x"}]},
			{"tag_name":"b6499","assets":[{"name":"llama-b6499-bin-macos-arm64.zip","browser_download_url":"x"}]}
		]`))
	}))
	defer srv.Close()
	old := releasesURL
	releasesURL = srv.URL
	defer func() { releasesURL = old }()

	r := New(t.TempDir(), t.TempDir())
	r.goos, r.goarch = "freebsd", "riscv64"
	r.vulkan = func(context.Context) bool { return false }

	_, err := r.chooseAsset(context.Background())
	if err == nil {
		t.Fatal("no build for this computer")
	}
	msg := err.Error()
	for _, want := range []string{"no llama.cpp build for this computer", "the 2 newest releases, b6499 to b6500", r.installDir(), "docs/runtimes.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q is missing %q", msg, want)
		}
	}

	status = http.StatusForbidden
	_, err = r.chooseAsset(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 403 Forbidden") || !strings.Contains(err.Error(), "rate limit") || !strings.Contains(err.Error(), r.installDir()) {
		t.Fatalf("GitHub refusing: %v", err)
	}
}
