package llamacpp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

func TestHealthReportsMissingLlamaServerLocation(t *testing.T) {
	runtimesDir := t.TempDir()
	runtime := New(runtimesDir, t.TempDir())

	detection, err := runtime.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if detection.Installed {
		t.Fatal("llama-server unexpectedly detected as installed")
	}

	err = runtime.Health(context.Background())
	if err == nil {
		t.Fatal("expected health check to report missing llama-server")
	}
	if !strings.Contains(err.Error(), filepath.Join(runtimesDir, "llamacpp")) {
		t.Fatalf("health error %q does not include the install location", err)
	}

	_, err = runtime.StartModel(context.Background(), pluginapi.ModelStartConfig{})
	if err == nil {
		t.Fatal("expected StartModel to report missing llama-server")
	}
	if !strings.Contains(err.Error(), detection.Message) {
		t.Fatalf("StartModel error %q does not include detection message %q", err, detection.Message)
	}
}

// Installed is the cheap check made before every model start (#610).
func TestInstalledIsTheBinaryInPlace(t *testing.T) {
	r := New(t.TempDir(), t.TempDir())
	if r.Installed() {
		t.Fatal("installed with nothing there")
	}
	if err := os.MkdirAll(filepath.Dir(r.binaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.binaryPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !r.Installed() {
		t.Fatal("not installed with llama-server in place")
	}
}
