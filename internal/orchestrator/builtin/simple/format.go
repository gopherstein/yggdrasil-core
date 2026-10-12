package simple

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// A request can say how long a clip should be, or how large a picture:
// "a 10-second video", "in 1080p", "1920x1080", "vertical". That reaches
// the tool, so Toskar makes what was asked, or as close as it can and says
// so (#529), instead of its default size and length.

var (
	// secondsRe is a length in seconds: "10 seconds", "10-second", "10
	// sec", "10 segundos", "10秒", "10초". A bare "s" isn't one: "90s
	// style" is a decade.
	secondsRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*-?\s*(seconds?\b|secs?\b|segundos?|secondes?|secondi|sekunden|sekunde|秒|초)`)
	// minutesRe is a length in minutes.
	minutesRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*-?\s*(minutes?|mins?\b|minutos?|minuti|minuten|分|분)`)
	// sizeRe is a size in pixels: "1920x1080", "1024 × 768".
	sizeRe = regexp.MustCompile(`\b(\d{3,4})\s*[x×]\s*(\d{3,4})\b`)
	// linesRe is a video standard: "720p", "1080p", "4k".
	linesRe = regexp.MustCompile(`(?i)\b(480|540|720|1080|1440|2160)p\b|\b(4k|uhd)\b`)
	// bestRe and fastRe ask for a quality (#549): high quality, or a quick
	// draft. Words in scripts without spaces have no word boundaries.
	bestRe = regexp.MustCompile(`(?i)\b(high[- ]quality|higher[- ]quality|best[- ]quality|highest[- ]quality|top[- ]quality|max(imum)?[- ]quality|ultra[- ]detailed|alta calidad|mejor calidad|haute qualité|meilleure qualité|alta qualità|massima qualità|hohe[rnm]? qualität|beste[rnm]? qualität|höchste[rnm]? qualität|alta qualidade|melhor qualidade)\b|高画質|高品質|最高品質|高画质|高质量|最高质量|고화질|고품질|최고 품질`)
	fastRe = regexp.MustCompile(`(?i)\b(quick (draft|preview|version|sketch)|rough (draft|version|sketch)|draft|low[- ]quality|fast version|borrador|brouillon|bozza|entwurf|rascunho)\b|下書き|ラフ|草稿|草图|초안`)
	// shapeRe is a shape without a size.
	shapeRe = regexp.MustCompile(`(?i)\b(vertical|portrait|square|landscape|widescreen)\b|\b(9:16|16:9|1:1|4:3|3:4)\b`)
)

// askedFormat is the length and size a request asks for, as tool
// arguments; only what it states.
func askedFormat(prompt, tool string) map[string]any {
	out := map[string]any{}
	if tool == "video.generate" {
		if m := minutesRe.FindStringSubmatch(prompt); m != nil {
			out["seconds"] = number(m[1]) * 60
		} else if m := secondsRe.FindStringSubmatch(prompt); m != nil {
			out["seconds"] = number(m[1])
		}
	}
	w, h := 0, 0
	if m := sizeRe.FindStringSubmatch(prompt); m != nil {
		w, _ = strconv.Atoi(m[1])
		h, _ = strconv.Atoi(m[2])
	} else if m := linesRe.FindStringSubmatch(prompt); m != nil {
		lines := 2160
		if m[1] != "" {
			lines, _ = strconv.Atoi(m[1])
		}
		w, h = lines*16/9, lines
	}
	if m := shapeRe.FindStringSubmatch(prompt); m != nil {
		shape := strings.ToLower(m[0])
		long, short := max(w, h), min(w, h)
		if long == 0 {
			// A shape alone keeps the tool's usual size.
			long, short = 1024, 576
			if tool == "video.generate" {
				long, short = 832, 480
			}
		}
		switch shape {
		case "vertical", "portrait", "9:16":
			w, h = short, long
		case "3:4":
			w, h = long*3/4, long
		case "4:3":
			w, h = long, long*3/4
		case "square", "1:1":
			side := int(math.Sqrt(float64(long * short)))
			w, h = side, side
		default:
			w, h = long, short
		}
	}
	if w > 0 && h > 0 {
		out["width"], out["height"] = float64(w), float64(h)
	}
	switch {
	case bestRe.MatchString(prompt):
		out["quality"] = "best"
	case fastRe.MatchString(prompt):
		out["quality"] = "fast"
	}
	return out
}

func number(s string) float64 {
	n, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return n
}
