package imagegen

import (
	"fmt"
	"testing"
)

// A clip longer or larger than can be made says so in its result, with
// what was asked and what was made (#529); one made as asked doesn't.
func TestVideoSaysWhatWasLimited(t *testing.T) {
	_, eng, st, ctx := videoFixture(t)
	tool := &VideoTool{Engine: eng, Store: st}
	res, err := tool.Execute(ctx, map[string]any{"prompt": "Waves", "seconds": float64(10), "width": float64(1920), "height": float64(1080)})
	if err != nil {
		t.Fatal(err)
	}
	limited, _ := res["limited"].(map[string]any)
	if got := fmt.Sprint(limited); got != "map[asked_height:1080 asked_seconds:10 asked_width:1920 made_height:480 made_seconds:5 made_width:832 max_seconds:5]" {
		t.Fatalf("limited %s", got)
	}
	res, err = tool.Execute(ctx, map[string]any{"prompt": "Waves", "seconds": float64(3)})
	if err != nil || res["limited"] != nil {
		t.Fatalf("a clip made as asked says %v (%v)", res["limited"], err)
	}
}

func TestLimits(t *testing.T) {
	if l := limits(1024, 1024, 1024, 1024, 0, 0); l != nil {
		t.Fatalf("same size limited: %v", l)
	}
	if l := limits(3840, 2160, 1600, 896, 0, 0); l["asked_width"] != 3840 {
		t.Fatalf("4k picture: %v", l)
	}
	// Rounding to the model's steps isn't a limit.
	if l := limits(1000, 600, 1024, 576, 0, 0); l != nil {
		t.Fatalf("rounding limited: %v", l)
	}
}
