package imagegen

import (
	"strings"
	"testing"
)

// A computer without the memory for a model isn't offered it, can't set it
// up, and says why, instead of failing or freezing when it runs (#guardrails).
func TestTooLittleMemory(t *testing.T) {
	f := newFixture(t)
	f.setup.Memory = func() int64 { return 4 << 30 }
	st := f.setup.Status()
	var smallest ModelStatus
	for _, m := range st.Models {
		if m.Recommended {
			smallest = m
		}
	}
	if !smallest.TooLittleMemory || st.MemoryBytes != 4<<30 {
		t.Fatalf("4 GB and %s: %+v", smallest.Name, smallest)
	}
	err := f.setup.Start(smallest.ID)
	if err == nil || !strings.Contains(err.Error(), "needs at least 7 GB of memory, and this computer has 4 GB") {
		t.Fatalf("start = %v", err)
	}
	e := &Engine{Setup: f.setup}
	if ok, why := e.Available(); ok || !strings.Contains(why, "can't run here") {
		t.Fatalf("available = %v %q", ok, why)
	}

	// 8 GB holds it, below what it's comfortable with: allowed, with a
	// warning. 16 GB, or memory that isn't known, is fine.
	f.setup.Memory = func() int64 { return 8 << 30 }
	for _, m := range f.setup.Status().Models {
		if m.Recommended && (m.TooLittleMemory || !m.TightMemory) {
			t.Fatalf("8 GB: %+v", m)
		}
	}
	f.setup.Memory = func() int64 { return 16 << 30 }
	for _, m := range f.setup.Status().Models {
		if m.Recommended && (m.TooLittleMemory || m.TightMemory) {
			t.Fatalf("16 GB: %+v", m)
		}
	}
	f.setup.Memory = nil
	if _, why := e.Available(); strings.Contains(why, "memory") {
		t.Fatalf("unknown memory: %q", why)
	}
}
