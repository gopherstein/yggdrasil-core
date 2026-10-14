package quality

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
)

// Pictures for the vision cases (#449), drawn here so they're our own and
// the same every run: text on a sign or a receipt, a bar chart, and an
// error dialog. Text uses a 5x7 pixel font, scaled up so a vision model
// reads it as a person would.

// glyphs are the 5x7 font: each letter is seven rows of five, # for ink.
var glyphs = map[rune][7]string{
	'A': {" ### ", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'B': {"#### ", "#   #", "#   #", "#### ", "#   #", "#   #", "#### "},
	'C': {" ####", "#    ", "#    ", "#    ", "#    ", "#    ", " ####"},
	'D': {"#### ", "#   #", "#   #", "#   #", "#   #", "#   #", "#### "},
	'E': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#####"},
	'F': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#    "},
	'G': {" ####", "#    ", "#    ", "#  ##", "#   #", "#   #", " ####"},
	'H': {"#   #", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'I': {"#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "#####"},
	'J': {"  ###", "    #", "    #", "    #", "    #", "#   #", " ### "},
	'K': {"#   #", "#  # ", "# #  ", "##   ", "# #  ", "#  # ", "#   #"},
	'L': {"#    ", "#    ", "#    ", "#    ", "#    ", "#    ", "#####"},
	'M': {"#   #", "## ##", "# # #", "# # #", "#   #", "#   #", "#   #"},
	'N': {"#   #", "##  #", "# # #", "#  ##", "#   #", "#   #", "#   #"},
	'O': {" ### ", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'P': {"#### ", "#   #", "#   #", "#### ", "#    ", "#    ", "#    "},
	'Q': {" ### ", "#   #", "#   #", "#   #", "# # #", "#  # ", " ## #"},
	'R': {"#### ", "#   #", "#   #", "#### ", "# #  ", "#  # ", "#   #"},
	'S': {" ####", "#    ", "#    ", " ### ", "    #", "    #", "#### "},
	'T': {"#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  "},
	'U': {"#   #", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'V': {"#   #", "#   #", "#   #", "#   #", "#   #", " # # ", "  #  "},
	'W': {"#   #", "#   #", "#   #", "# # #", "# # #", "## ##", "#   #"},
	'X': {"#   #", "#   #", " # # ", "  #  ", " # # ", "#   #", "#   #"},
	'Y': {"#   #", "#   #", " # # ", "  #  ", "  #  ", "  #  ", "  #  "},
	'Z': {"#####", "    #", "   # ", "  #  ", " #   ", "#    ", "#####"},
	'0': {" ### ", "#   #", "#  ##", "# # #", "##  #", "#   #", " ### "},
	'1': {"  #  ", " ##  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'2': {" ### ", "#   #", "    #", "   # ", "  #  ", " #   ", "#####"},
	'3': {"#### ", "    #", "    #", " ### ", "    #", "    #", "#### "},
	'4': {"   # ", "  ## ", " # # ", "#  # ", "#####", "   # ", "   # "},
	'5': {"#####", "#    ", "#### ", "    #", "    #", "#   #", " ### "},
	'6': {" ### ", "#    ", "#    ", "#### ", "#   #", "#   #", " ### "},
	'7': {"#####", "    #", "   # ", "  #  ", " #   ", " #   ", " #   "},
	'8': {" ### ", "#   #", "#   #", " ### ", "#   #", "#   #", " ### "},
	'9': {" ### ", "#   #", "#   #", " ####", "    #", "    #", " ### "},
	' ': {"     ", "     ", "     ", "     ", "     ", "     ", "     "},
	'.': {"     ", "     ", "     ", "     ", "     ", " ##  ", " ##  "},
	',': {"     ", "     ", "     ", "     ", " ##  ", "  #  ", " #   "},
	':': {"     ", " ##  ", " ##  ", "     ", " ##  ", " ##  ", "     "},
	'!': {"  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "     ", "  #  "},
	'?': {" ### ", "#   #", "    #", "   # ", "  #  ", "     ", "  #  "},
	'-': {"     ", "     ", "     ", "#####", "     ", "     ", "     "},
	'$': {"  #  ", " ####", "# #  ", " ### ", "  # #", "#### ", "  #  "},
	'%': {"##  #", "##  #", "   # ", "  #  ", " #   ", "#  ##", "#  ##"},
	'/': {"    #", "    #", "   # ", "  #  ", " #   ", "#    ", "#    "},
}

var palette = map[string]color.RGBA{
	"white": {255, 255, 255, 255}, "black": {0, 0, 0, 255}, "red": {220, 30, 30, 255},
	"blue": {30, 70, 220, 255}, "green": {30, 160, 60, 255}, "yellow": {240, 200, 20, 255},
	"gray": {200, 200, 205, 255}, "dark": {60, 60, 70, 255},
}

// textWidth is the width of s at scale, in pixels.
func textWidth(s string, scale int) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return (n*6 - 1) * scale
}

// drawText writes s with its top-left corner at x, y.
func drawText(img draw.Image, s string, x, y, scale int, ink color.Color) error {
	for _, r := range strings.ToUpper(s) {
		g, ok := glyphs[r]
		if !ok {
			return fmt.Errorf("no glyph for %q", r)
		}
		for row, line := range g {
			for col, c := range line {
				if c != '#' {
					continue
				}
				fill(img, image.Rect(x+col*scale, y+row*scale, x+(col+1)*scale, y+(row+1)*scale), ink)
			}
		}
		x += 6 * scale
	}
	return nil
}

func fill(img draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Src)
}

// specOf reads "key=value" fields; a value may hold spaces up to the next
// "key=", as in "text=TOTAL $42.17 bg=white".
func specOf(spec string, defaults map[string]string) map[string]string {
	opt := map[string]string{}
	for k, v := range defaults {
		opt[k] = v
	}
	key := ""
	for _, f := range strings.Fields(spec) {
		if k, v, ok := strings.Cut(f, "="); ok && !strings.ContainsAny(k, " ") && k != "" && strings.ToLower(k) == k {
			key = k
			opt[k] = v
			continue
		}
		if key != "" {
			opt[key] += " " + f
		}
	}
	return opt
}

func encodePNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// textPNG draws lines of text, split by |, centered on a plain picture:
// "w=640 h=320 bg=white color=black text=TOTAL $42.17|THANK YOU".
func textPNG(spec string) ([]byte, error) {
	opt := specOf(spec, map[string]string{"w": "640", "h": "320", "bg": "white", "color": "black", "scale": "6"})
	w, err1 := strconv.Atoi(opt["w"])
	h, err2 := strconv.Atoi(opt["h"])
	scale, err3 := strconv.Atoi(opt["scale"])
	bg, ok1 := palette[opt["bg"]]
	ink, ok2 := palette[opt["color"]]
	if err1 != nil || err2 != nil || err3 != nil || !ok1 || !ok2 || opt["text"] == "" {
		return nil, fmt.Errorf("bad text picture %q", spec)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), bg)
	lines := strings.Split(opt["text"], "|")
	lineH := 10 * scale
	y := (h - len(lines)*lineH + 3*scale) / 2
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if err := drawText(img, line, (w-textWidth(line, scale))/2, y, scale, ink); err != nil {
			return nil, err
		}
		y += lineH
	}
	return encodePNG(img)
}

// barsPNG draws a bar chart with a label under each bar and its value on
// top: "labels=A,B,C,D values=3,7,5,2 color=blue".
func barsPNG(spec string) ([]byte, error) {
	opt := specOf(spec, map[string]string{"labels": "A,B,C", "values": "1,2,3", "color": "blue"})
	labels := strings.Split(opt["labels"], ",")
	raw := strings.Split(opt["values"], ",")
	bar, ok := palette[opt["color"]]
	if len(labels) != len(raw) || len(labels) == 0 || len(labels) > 8 || !ok {
		return nil, fmt.Errorf("bad chart %q", spec)
	}
	values := make([]int, len(raw))
	top := 0
	for i, v := range raw {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 99 {
			return nil, fmt.Errorf("bad chart value %q", v)
		}
		values[i], top = n, max(top, n)
	}
	const w, h, scale = 640, 480, 4
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), palette["white"])
	base, roof := h-70, 60
	fill(img, image.Rect(40, base, w-40, base+3), palette["black"])
	slot := (w - 80) / len(labels)
	for i, v := range values {
		x := 40 + slot*i + slot/4
		bh := 0
		if top > 0 {
			bh = (base - roof) * v / top
		}
		fill(img, image.Rect(x, base-bh, x+slot/2, base), bar)
		num := strconv.Itoa(v)
		if err := drawText(img, num, x+(slot/2-textWidth(num, scale))/2, base-bh-9*scale, scale, palette["black"]); err != nil {
			return nil, err
		}
		if err := drawText(img, labels[i], x+(slot/2-textWidth(labels[i], scale))/2, base+15, scale, palette["black"]); err != nil {
			return nil, err
		}
	}
	return encodePNG(img)
}

// dialogPNG draws an error dialog as a screenshot shows one: a gray window
// with a dark title bar, a red mark, the message, and an OK button.
// "title=ERROR text=DISK IS FULL|FREE UP SPACE AND TRY AGAIN".
func dialogPNG(spec string) ([]byte, error) {
	opt := specOf(spec, map[string]string{"title": "ERROR"})
	if opt["text"] == "" {
		return nil, fmt.Errorf("bad dialog %q", spec)
	}
	const w, h, scale = 720, 360, 3
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), palette["gray"])
	fill(img, image.Rect(0, 0, w, 48), palette["dark"])
	if err := drawText(img, opt["title"], 20, 14, scale, palette["white"]); err != nil {
		return nil, err
	}
	fill(img, image.Rect(30, 90, 90, 150), palette["red"])
	if err := drawText(img, "!", 48, 99, 6, palette["white"]); err != nil {
		return nil, err
	}
	y := 95
	for _, line := range strings.Split(opt["text"], "|") {
		if err := drawText(img, strings.TrimSpace(line), 120, y, scale, palette["black"]); err != nil {
			return nil, err
		}
		y += 36
	}
	fill(img, image.Rect(w-160, h-80, w-40, h-30), palette["white"])
	if err := drawText(img, "OK", w-160+(120-textWidth("OK", scale))/2, h-80+15, scale, palette["black"]); err != nil {
		return nil, err
	}
	return encodePNG(img)
}
