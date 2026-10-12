package simple

import (
	"fmt"
	"testing"
)

// The length and size a request states reach the tool (#529).
func TestAskedFormat(t *testing.T) {
	for _, c := range []struct {
		prompt, tool, want string
	}{
		{"Make a 10-second video of waves", "video.generate", "map[seconds:10]"},
		{"make a video of a cat, 3 seconds long", "video.generate", "map[seconds:3]"},
		{"Haz un video de 8 segundos de un perro", "video.generate", "map[seconds:8]"},
		{"猫の動画を5秒で作って", "video.generate", "map[seconds:5]"},
		{"a one-minute clip... I mean 1 minute video of rain", "video.generate", "map[seconds:60]"},
		{"Make a 720p video of a city at night", "video.generate", "map[height:720 width:1280]"},
		{"Make a vertical video of a waterfall", "video.generate", "map[height:832 width:480]"},
		{"Draw a dog at 1920x1080", "image.generate", "map[height:1080 width:1920]"},
		{"Draw a 4k wallpaper of mountains", "image.generate", "map[height:2160 width:3840]"},
		{"Draw a square logo for my shop", "image.generate", "map[height:768 width:768]"},
		{"Make a 90s style video of a mall", "video.generate", "map[]"},
		{"Draw a dog", "image.generate", "map[]"},
		{"Draw a dog that is 10 seconds old", "image.generate", "map[]"},
		{"Draw a high-quality portrait of an old fisherman", "image.generate", "map[height:1024 quality:best width:576]"},
		{"Make a quick draft of a logo", "image.generate", "map[quality:fast]"},
		{"Haz una imagen en alta calidad de un faro", "image.generate", "map[quality:best]"},
		{"高画質で猫の動画を作って", "video.generate", "map[quality:best]"},
	} {
		got := askedFormat(c.prompt, c.tool)
		if s := fmt.Sprint(got); s != c.want {
			t.Errorf("%q = %s, want %s", c.prompt, s, c.want)
		}
	}
}

// "Make a 10-second video" reaches the tool with its length.
func TestClipRequestKeepsItsLength(t *testing.T) {
	env := &mediaEnv{scriptedEnv: scriptedEnv{replies: []string{"Waves rolling onto a beach.", "It's below."}}}
	runText(t, env, "Make a 10-second 720p video of waves on a beach", mediaProfile)
	if env.toolID != "video.generate" || env.args["seconds"] != 10.0 || env.args["width"] != 1280.0 || env.args["height"] != 720.0 {
		t.Fatalf("called %s %v", env.toolID, env.args)
	}
}

// "A high-quality picture" reaches the tool asking for the best tier (#549).
func TestPictureRequestKeepsItsQuality(t *testing.T) {
	env := &mediaEnv{scriptedEnv: scriptedEnv{replies: []string{"An old fisherman mending a net at dawn.", "It's below."}}}
	runText(t, env, "Draw a high-quality picture of an old fisherman", mediaProfile)
	if env.toolID != "image.generate" || env.args["quality"] != "best" {
		t.Fatalf("called %s %v", env.toolID, env.args)
	}
}
