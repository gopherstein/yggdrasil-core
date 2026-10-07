package imagegen

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Release is the stable-diffusion.cpp build setup installs. It is pinned,
// with each archive's checksum, so an update is a deliberate change here.
const Release = "master-929-3f8527a"

// Archive is one platform's build.
type Archive struct {
	Name   string
	Size   int64
	SHA256 string
	// Build is how it runs: metal or vulkan on the GPU, or cpu (#154).
	Build string
}

// Builds.
const (
	BuildMetal  = "metal"
	BuildVulkan = "vulkan"
	BuildCPU    = "cpu"
)

// gpuBuild reports a build that runs on the GPU.
func gpuBuild(build string) bool { return build == BuildMetal || build == BuildVulkan }

// URL is where the archive is downloaded from.
func (a Archive) URL() string {
	return "https://github.com/leejet/stable-diffusion.cpp/releases/download/" + Release + "/" + a.Name
}

// archives are the builds for each GOOS/GOARCH, the GPU build first. macOS
// uses Metal. Linux and Windows use Vulkan when a GPU has a Vulkan driver,
// which covers NVIDIA, AMD, and Intel at a fraction of the CUDA and ROCm
// builds' size, and the CPU build otherwise (#154).
var archives = map[string][]Archive{
	"darwin/arm64": {{Name: "sd-master-3f8527a-bin-Darwin-macOS-26.6.2-arm64.zip", Size: 35049086,
		SHA256: "1c8ee6c8e413e3335b1223bbc657ea5d86dae1819f8426a98b84c265587eb192", Build: BuildMetal}},
	"linux/amd64": {
		{Name: "sd-master-3f8527a-bin-Linux-Ubuntu-24.04-x86_64-vulkan.zip", Size: 36886217,
			SHA256: "e35cc73cf5ba9637d1dc1d717760e7b8428376a4905d57e72ec8c871864f62c7", Build: BuildVulkan},
		{Name: "sd-master-3f8527a-bin-Linux-Ubuntu-24.04-x86_64.zip", Size: 26025088,
			SHA256: "9ad35ed309dbe59f5e66f35edafac9a69e6cc233ea6b9577071feae829159d37", Build: BuildCPU},
	},
	"windows/amd64": {
		{Name: "sd-master-3f8527a-bin-win-vulkan-x64.zip", Size: 30067605,
			SHA256: "60e6850d650417409f18c2170ab5e27335db96da70cd3d1ad1e930bcffc2fd35", Build: BuildVulkan},
		{Name: "sd-master-3f8527a-bin-win-cpu-x64.zip", Size: 17486440,
			SHA256: "5e7caca2080321b25a12c1fa4175cb7d953f2b182309f8f73bfc9c725231d26c", Build: BuildCPU},
	},
}

// platformArchives returns this computer's builds, the GPU build first.
func platformArchives() []Archive {
	return archives[runtime.GOOS+"/"+runtime.GOARCH]
}

// pickArchive chooses from builds: the GPU build when gpu says one can run
// and the person hasn't asked for the CPU build, otherwise the CPU build.
// Metal always runs.
func pickArchive(builds []Archive, gpu func() bool, prefer string) (Archive, bool) {
	var cpu *Archive
	for i := range builds {
		if builds[i].Build == BuildCPU {
			cpu = &builds[i]
		}
	}
	if prefer != BuildCPU || cpu == nil {
		for _, b := range builds {
			if b.Build == BuildMetal || (b.Build == BuildVulkan && gpu != nil && gpu()) {
				return b, true
			}
		}
	}
	if cpu != nil {
		return *cpu, true
	}
	if len(builds) > 0 {
		return builds[0], true
	}
	return Archive{}, false
}

func cliName() string {
	if runtime.GOOS == "windows" {
		return "sd-cli.exe"
	}
	return "sd-cli"
}

// extractZip unpacks the archive into dir and returns the path of sd-cli.
// Libraries beside it in the archive stay beside it.
func extractZip(archive, dir string) (string, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	cli := ""
	for _, f := range zr.File {
		target := filepath.Join(dir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid archive path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := writeZipFile(f, target); err != nil {
			return "", err
		}
		if filepath.Base(target) == cliName() {
			cli = target
		}
	}
	if cli == "" {
		return "", fmt.Errorf("the stable-diffusion.cpp archive has no %s", cliName())
	}
	return cli, os.Chmod(cli, 0o755)
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	if f.Mode()&os.ModeSymlink != 0 {
		// A library's versioned name, such as libsd.dylib -> libsd.1.dylib.
		link, err := io.ReadAll(io.LimitReader(rc, 1024))
		if err != nil {
			return err
		}
		if filepath.IsAbs(string(link)) || strings.Contains(string(link), "..") {
			return fmt.Errorf("invalid archive link: %s", f.Name)
		}
		_ = os.Remove(target)
		return os.Symlink(string(link), target)
	}
	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
