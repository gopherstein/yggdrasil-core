package quality

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The media set records how long each case took and the most memory in
// use while it ran, from the daemon's live figures, so each model's needs
// are measured, not guessed (#553, #550). The figures are for the whole
// computer: the chat model, the judge, and the job together, from readings
// the daemon takes every five seconds while a model is loaded or a picture
// or clip is being made, so a short peak can be missed.

// memoryPeak is a case's time and peak memory; 0 is a figure the computer
// doesn't give.
type memoryPeak struct {
	ID       string
	Seconds  float64
	Memory   uint64
	Graphics uint64
}

// watchMemory samples the daemon's live figures every two seconds until
// the function it returns is called, which returns the peak.
func (d realDriver) watchMemory(id string) func() memoryPeak {
	start := time.Now()
	peak := memoryPeak{ID: id}
	var mu sync.Mutex
	done := make(chan struct{})
	var wg sync.WaitGroup
	sample := func() {
		var figures []contracts.LiveFigures
		if code, err := d.request(http.MethodGet, "/api/v1/performance/live", nil, &figures, func(string, ...any) {}); err != nil || code != http.StatusOK || len(figures) == 0 {
			return
		}
		cur := figures[0].Current
		mu.Lock()
		defer mu.Unlock()
		if cur.MemoryUsedBytes != nil {
			peak.Memory = max(peak.Memory, *cur.MemoryUsedBytes)
		}
		var gpu uint64
		for _, g := range cur.GPUs {
			if g.MemoryUsedBytes != nil {
				gpu += *g.MemoryUsedBytes
			}
		}
		peak.Graphics = max(peak.Graphics, gpu)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			sample()
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	}()
	return func() memoryPeak {
		close(done)
		wg.Wait()
		mu.Lock()
		defer mu.Unlock()
		peak.Seconds = time.Since(start).Seconds()
		return peak
	}
}

// memoryTable is the report's section of times and peaks.
func memoryTable(models string, peaks []memoryPeak) string {
	if len(peaks) == 0 {
		return ""
	}
	gb := func(n uint64) string {
		if n == 0 {
			return "—"
		}
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	}
	var b strings.Builder
	b.WriteString("\n## Memory and time\n\n")
	if models != "" {
		b.WriteString(models + "\n\n")
	}
	b.WriteString("The most memory in use on the computer while each case ran: the chat model, the judge, and the picture, clip, or speech job together, from readings every five seconds.\n\n")
	b.WriteString("| Case | Time | Peak memory | Peak graphics memory |\n| --- | --- | --- | --- |\n")
	for _, p := range peaks {
		fmt.Fprintf(&b, "| %s | %.0f s | %s | %s |\n", p.ID, p.Seconds, gb(p.Memory), gb(p.Graphics))
	}
	return b.String()
}

func TestMemoryTable(t *testing.T) {
	got := memoryTable("Picture model: z-image-turbo. Clip model: wan2.2-ti2v-5b.", []memoryPeak{
		{ID: "make_image_apple", Seconds: 40.2, Memory: 12 << 30, Graphics: 18 << 30},
		{ID: "read_aloud", Seconds: 9, Memory: 6 << 30},
	})
	for _, want := range []string{"Picture model: z-image-turbo", "| make_image_apple | 40 s | 12.0 GB | 18.0 GB |", "| read_aloud | 9 s | 6.0 GB | — |"} {
		if !strings.Contains(got, want) {
			t.Errorf("table lacks %q:\n%s", want, got)
		}
	}
	if memoryTable("", nil) != "" {
		t.Error("an empty table was written")
	}
}

func TestFlatPicture(t *testing.T) {
	blank := image.NewRGBA(image.Rect(0, 0, 64, 64))
	if !flat(blank) {
		t.Error("an all-black picture isn't flat")
	}
	shapes, err := shapesPNG("size=128 bg=white shape=circle color=red count=2")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(shapes))
	if err != nil {
		t.Fatal(err)
	}
	if flat(img) {
		t.Error("a picture of two red circles counts as flat")
	}
	for in, want := range map[string]bool{"Yes, waves roll in.": true, "**yes**": true, "No, the frames are the same.": false, "": false} {
		if saysYes(in) != want {
			t.Errorf("saysYes(%q) = %v", in, !want)
		}
	}
}
