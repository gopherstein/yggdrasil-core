package imagegen

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// Linux and Windows take the Vulkan build when a GPU can run it, and the
// CPU build otherwise or when asked; macOS takes Metal (#154).
func TestPickArchive(t *testing.T) {
	yes, no := func() bool { return true }, func() bool { return false }
	for name, tc := range map[string]struct {
		platform string
		gpu      func() bool
		prefer   string
		want     string
	}{
		"linux with a GPU":     {"linux/amd64", yes, "", BuildVulkan},
		"linux without one":    {"linux/amd64", no, "", BuildCPU},
		"linux, not checked":   {"linux/amd64", nil, "", BuildCPU},
		"linux, CPU chosen":    {"linux/amd64", yes, BuildCPU, BuildCPU},
		"windows with a GPU":   {"windows/amd64", yes, "", BuildVulkan},
		"windows without one":  {"windows/amd64", no, "", BuildCPU},
		"macOS":                {"darwin/arm64", no, "", BuildMetal},
		"macOS, CPU asked for": {"darwin/arm64", no, BuildCPU, BuildMetal},
	} {
		a, ok := pickArchive(archives[tc.platform], tc.gpu, tc.prefer)
		if !ok || a.Build != tc.want {
			t.Errorf("%s: %+v %v, want %s", name, a, ok, tc.want)
		}
	}
	if _, ok := pickArchive(nil, yes, ""); ok {
		t.Error("a platform without builds got one")
	}
	for platform, list := range archives {
		for _, a := range list {
			if len(a.SHA256) != 64 || a.Size <= 0 || !strings.Contains(a.Name, "3f8527a") {
				t.Errorf("%s: %+v", platform, a)
			}
		}
	}
}

// zipWith is an archive whose sd-cli is script.
func zipWith(t *testing.T, script string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string]string{"bin/sd-cli": script} {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		w, _ := zw.CreateHeader(h)
		_, _ = w.Write([]byte(data))
	}
	h := &zip.FileHeader{Name: "bin/sample.png", Method: zip.Deflate}
	h.SetMode(0o644)
	w, _ := zw.CreateHeader(h)
	_, _ = w.Write(samplePNG(t, 64, 64))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A Vulkan build that can't start on the GPU is replaced by the CPU build,
// which makes the picture, and the computer keeps the CPU build; the
// installed build is what's reported (#154).
func TestVulkanFallsBackToCPU(t *testing.T) {
	f := newFixture(t)
	noGPU := zipWith(t, "#!/bin/sh\necho 'ggml_vulkan: No Vulkan devices found' >&2\nexit 1\n")
	cpu := zipWith(t, fakeCLI)
	f.files["sd-vulkan.zip"], f.files["sd-cpu.zip"] = noGPU, cpu
	f.setup.archive = nil
	f.setup.builds = []Archive{
		{Name: "sd-vulkan.zip", Size: int64(len(noGPU)), SHA256: sum(noGPU), Build: BuildVulkan},
		{Name: "sd-cpu.zip", Size: int64(len(cpu)), SHA256: sum(cpu), Build: BuildCPU},
	}
	f.setup.GPU = func() bool { return true }
	m := f.model(t)
	archive, _ := f.setup.platform()
	if archive.Build != BuildVulkan {
		t.Fatalf("chose %+v", archive)
	}
	if err := f.setup.install(context.Background(), &job{}, m, true, archive); err != nil {
		t.Fatal(err)
	}
	st := f.setup.Status()
	if st.Build != BuildVulkan || !st.Accelerated || st.GPUBuild != BuildVulkan {
		t.Fatalf("installed %+v", st)
	}
	eng := &Engine{Setup: f.setup, WorkDir: t.TempDir()}
	res, err := eng.Generate(context.Background(), Request{Prompt: "a dog"})
	if err != nil || !bytes.HasPrefix(res.PNG, []byte("\x89PNG")) {
		t.Fatalf("generate = %v", err)
	}
	st = f.setup.Status()
	if st.Build != BuildCPU || st.Accelerated {
		t.Fatalf("after the fallback %+v", st)
	}
	// It stays on the CPU build; switching back to the GPU build is the
	// person's choice.
	if archive, _ := f.setup.platform(); archive.Build != BuildCPU {
		t.Fatalf("would reinstall %+v", archive)
	}
	if err := f.setup.UseBuild("gpu"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, f.setup)
	if f.setup.Status().Build != BuildVulkan {
		t.Fatalf("switched back: %+v", f.setup.Status())
	}
	raw, _ := os.ReadFile(f.setup.ProgramDir + "/prefer")
	if strings.TrimSpace(string(raw)) != "" {
		t.Fatalf("prefer = %q", raw)
	}
}

// Other failures, such as running out of memory, don't switch builds.
func TestOtherFailuresKeepTheGPUBuild(t *testing.T) {
	for _, out := range []string{"error: out of memory", "ggml_vulkan: Found 1 Vulkan devices\nerror: model file is corrupt", ""} {
		if gpuFailure(out) {
			t.Errorf("%q counted as the GPU build not starting", out)
		}
	}
	for _, out := range []string{"ggml_vulkan: No Vulkan devices found", "sd-cli: error while loading shared libraries: libvulkan.so.1: cannot open shared object file", "VK_ERROR_INCOMPATIBLE_DRIVER"} {
		if !gpuFailure(out) {
			t.Errorf("%q not counted", out)
		}
	}
}

// Switching to a GPU build where none can run is refused.
func TestUseBuildNeedsAGPU(t *testing.T) {
	f := newFixture(t)
	f.setup.archive = nil
	f.setup.builds = []Archive{{Name: "a.zip", Build: BuildVulkan}, {Name: "b.zip", Build: BuildCPU}}
	f.setup.GPU = func() bool { return false }
	if err := f.setup.UseBuild("gpu"); err == nil {
		t.Fatal("switched to the GPU build without a GPU")
	}
	if err := f.setup.UseBuild("fast"); err == nil {
		t.Fatal("unknown build accepted")
	}
}
