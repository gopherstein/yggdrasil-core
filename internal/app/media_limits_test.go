package app

import (
	"strings"
	"testing"
)

// What a picture or clip was cut back to is said under the answer, in the
// App language (#529).
func TestMediaLimitNotice(t *testing.T) {
	clip := map[string]any{"model": "Wan 2.2 TI2V 5B", "limited": map[string]any{
		"asked_seconds": 10.0, "made_seconds": 5.0, "max_seconds": 5.0,
		"asked_width": 1920, "asked_height": 1080, "made_width": 832, "made_height": 480}}
	got := mediaLimitNotice("en", "video.generate", clip)
	for _, want := range []string{"Clips can be up to 5 s long for now, so this one is 5 s instead of the 10 s asked for.", "Wan 2.2 TI2V 5B makes clips up to 832×480, so this one is smaller than the 1920×1080 asked for."} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q lacks %q", got, want)
		}
	}
	if ja := mediaLimitNotice("ja", "video.generate", clip); !strings.Contains(ja, "10 秒ではなく 5 秒") {
		t.Errorf("Japanese notice %q", ja)
	}
	picture := map[string]any{"model": "FLUX.2 [klein] 4B", "limited": map[string]any{"asked_width": 3840.0, "asked_height": 2160.0, "made_width": 1600.0, "made_height": 896.0}}
	if got := mediaLimitNotice("en", "image.generate", picture); got != "FLUX.2 [klein] 4B makes pictures up to 1600×896, so this one is smaller than the 3840×2160 asked for." {
		t.Errorf("picture notice %q", got)
	}
	if got := mediaLimitNotice("en", "image.generate", map[string]any{"width": 1024}); got != "" {
		t.Errorf("notice for a picture made as asked: %q", got)
	}
}
