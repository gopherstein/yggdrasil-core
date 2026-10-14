// Package instances notices another Toskar running on the same machine
// (#602). Two copies with their own data folders each enroll on the relay,
// each with its own route, and fight over ports and the GPU; the person
// usually didn't mean to run both, such as the apt service and a copy
// started by hand. Each daemon leaves a small file in a folder every user
// on the machine shares, and removes it when it stops; the others read
// them.
package instances

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Info is what one running Toskar says about itself. Nothing in it is a
// secret: other users on the machine can read it.
type Info struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	DataDir string `json:"data_dir"`
	// API is the port the API listens on.
	API     int       `json:"api_port"`
	Version string    `json:"version"`
	Started time.Time `json:"started"`
	// Remote is set while access from anywhere is on, when two copies
	// enroll on the relay.
	Remote bool `json:"remote,omitempty"`
}

// Dir is the folder every Toskar on this machine shares: /tmp/toskar-instances
// on macOS and Linux, and ProgramData\Toskar\instances on Windows.
func Dir() string {
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "Toskar", "instances")
		}
		return filepath.Join(os.TempDir(), "toskar-instances")
	}
	// Not os.TempDir: on macOS it's each user's own folder.
	return "/tmp/toskar-instances"
}

// Registry is the shared folder.
type Registry struct {
	Dir string
	// alive reports whether a process runs; nil checks the system.
	alive func(pid int) bool
}

// Default is the registry in Dir.
func Default() *Registry { return &Registry{Dir: Dir()} }

func (r *Registry) isAlive(pid int) bool {
	if r.alive != nil {
		return r.alive(pid)
	}
	return processAlive(pid)
}

func (r *Registry) path(pid int) string {
	return filepath.Join(r.Dir, strconv.Itoa(pid)+".json")
}

// Announce writes this Toskar's file, replacing an earlier one, and makes
// the folder writable by every user, as /tmp is.
func (r *Registry) Announce(info Info) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(r.Dir, 0o1777)
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(r.Dir, ".announce-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	return os.Rename(tmp.Name(), r.path(info.PID))
}

// Withdraw removes this Toskar's file.
func (r *Registry) Withdraw(pid int) {
	_ = os.Remove(r.path(pid))
}

// Others are the other Toskars running on this machine now, oldest first.
// A file whose process is gone is skipped, and removed when it can be.
func (r *Registry) Others(self int) []Info {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	var out []Info
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil || pid == self {
			continue
		}
		if !r.isAlive(pid) {
			_ = os.Remove(filepath.Join(r.Dir, name))
			continue
		}
		raw, err := os.ReadFile(filepath.Join(r.Dir, name))
		if err != nil {
			continue
		}
		var info Info
		if json.Unmarshal(raw, &info) != nil || info.PID != pid {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}
