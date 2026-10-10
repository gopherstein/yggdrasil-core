package quality

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The media set (#510): pictures, edits, video, and speech Toskar makes,
// judged for what they show or say, not only that a file came back. Every
// case checks the file itself (a picture decodes at the size asked, a WAV
// is whole and long enough); against a real daemon, a model that sees is
// asked whether the picture or the clip's frames show what was asked, and
// speech is transcribed back and compared word by word. The stub checks
// the plumbing with its stand-in tools.

// Judge is how a made file is judged.
type Judge struct {
	// Kind is image, video, or audio; Ext the file's extension.
	Kind string `json:"kind"`
	Ext  string `json:"ext"`
	// Shows is what a picture or a clip must show, asked of a model that
	// sees as "Does this show …?".
	Shows string `json:"shows"`
	// Says is what speech must say; up to MaxWER of its words may differ.
	Says   string  `json:"says"`
	MaxWER float64 `json:"max_wer"`
	// MinSide is the smallest a picture's sides may be, in pixels.
	MinSide int `json:"min_side"`
}

// mediaCase is a case in media.json.
type mediaCase struct {
	Case
	// Judge is how the file the turn makes is judged; nil for a case that
	// reads a file instead, judged by its answer.
	Judge *Judge `json:"judge"`
	// Recording is text Toskar's own voice reads into a WAV that's attached
	// to the message (real only), for a transcription case.
	Recording string `json:"recording"`
}

type mediaFile struct {
	Cases []mediaCase `json:"cases"`
}

func loadMediaCases(t *testing.T) []mediaCase {
	t.Helper()
	raw, err := os.ReadFile("media.json")
	if err != nil {
		t.Fatal(err)
	}
	var f mediaFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Cases
}

// madeFile is the first file the turn made with ext.
func madeFile(r Result, ext string) (string, []byte) {
	for name, data := range r.Files {
		if strings.EqualFold(filepath.Ext(name), ext) {
			return name, data
		}
	}
	return "", nil
}

// fileFailures checks a made file by itself: there, whole, and big enough.
func fileFailures(j *Judge, r Result, real bool) []string {
	name, data := madeFile(r, j.Ext)
	if name == "" {
		return []string{fmt.Sprintf("no %s was made", j.Ext)}
	}
	switch j.Kind {
	case "image":
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return []string{fmt.Sprintf("%s doesn't decode: %v", name, err)}
		}
		// The stub's stand-in is small; a real picture must be the size of one.
		if b := img.Bounds(); real && (b.Dx() < j.MinSide || b.Dy() < j.MinSide) {
			return []string{fmt.Sprintf("%s is %dx%d; each side should be at least %d", name, b.Dx(), b.Dy(), j.MinSide)}
		}
	case "audio":
		seconds, err := wavSeconds(data)
		if err != nil {
			return []string{fmt.Sprintf("%s isn't a whole WAV: %v", name, err)}
		}
		if seconds < 0.5 {
			return []string{fmt.Sprintf("%s is %.2f s long", name, seconds)}
		}
	case "video":
		if real && len(data) < 10_000 {
			return []string{fmt.Sprintf("%s is only %d bytes", name, len(data))}
		}
	}
	return nil
}

// wavSeconds reads a PCM WAV's header and checks its data is all there.
func wavSeconds(data []byte) (float64, error) {
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a WAV file")
	}
	var rate, bytesPerSec uint32
	pos := 12
	for pos+8 <= len(data) {
		id, size := string(data[pos:pos+4]), int(binary.LittleEndian.Uint32(data[pos+4:pos+8]))
		body := pos + 8
		switch id {
		case "fmt ":
			if size < 16 || body+16 > len(data) {
				return 0, fmt.Errorf("short fmt chunk")
			}
			rate = binary.LittleEndian.Uint32(data[body+4 : body+8])
			bytesPerSec = binary.LittleEndian.Uint32(data[body+8 : body+12])
		case "data":
			if rate == 0 || bytesPerSec == 0 {
				return 0, fmt.Errorf("data before fmt")
			}
			if body+size > len(data) {
				return 0, fmt.Errorf("the data stops at %d of %d bytes", len(data)-body, size)
			}
			return float64(size) / float64(bytesPerSec), nil
		}
		pos = body + size + size%2
	}
	return 0, fmt.Errorf("no data chunk")
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}']+`)

// wordErrorRate is how many words of got differ from want (substituted,
// left out, or added), as a share of want's words.
func wordErrorRate(want, got string) float64 {
	w := wordRe.FindAllString(strings.ToLower(want), -1)
	g := wordRe.FindAllString(strings.ToLower(got), -1)
	if len(w) == 0 {
		return 0
	}
	prev := make([]int, len(g)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(w); i++ {
		cur := make([]int, len(g)+1)
		cur[0] = i
		for j := 1; j <= len(g); j++ {
			cost := 1
			if w[i-1] == g[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return float64(prev[len(g)]) / float64(len(w))
}

// judgeFailures asks a real daemon about a made file: a model that sees
// whether a picture or a clip shows what was asked, or a transcription of
// speech compared with what it should say.
func judgeFailures(t *testing.T, d realDriver, j *Judge, r Result) []string {
	name, data := madeFile(r, j.Ext)
	if name == "" {
		return nil // fileFailures said so
	}
	switch j.Kind {
	case "image", "video":
		what := "picture"
		if j.Kind == "video" {
			what = "video"
		}
		answer, err := d.askAbout(t, judgeModel(), name, data,
			fmt.Sprintf("Does this %s show %s? Answer yes or no on the first line, then say in one sentence what it shows.", what, j.Shows))
		if err != nil {
			return []string{fmt.Sprintf("the judge couldn't look at %s: %v", name, err)}
		}
		if !strings.HasPrefix(strings.ToLower(strings.Trim(strings.TrimSpace(answer), "*`\"'")), "yes") {
			return []string{fmt.Sprintf("the judge says %s doesn't show %s: %.200q", name, j.Shows, answer)}
		}
	case "audio":
		answer, err := d.askAbout(t, d.modelID(), name, data, "Transcribe this recording word for word. Reply with only the words it says.")
		if err != nil {
			return []string{fmt.Sprintf("couldn't transcribe %s: %v", name, err)}
		}
		limit := j.MaxWER
		if limit == 0 {
			limit = 0.25
		}
		if wer := wordErrorRate(j.Says, answer); wer > limit {
			return []string{fmt.Sprintf("%s says %.200q; %.0f%% of its words differ from %q (at most %.0f%%)", name, answer, wer*100, j.Says, limit*100)}
		}
	}
	return nil
}

// judgeModel is the model that sees, asked about pictures and clips.
func judgeModel() string {
	if m := config.Env("QUALITY_JUDGE_MODEL"); m != "" {
		return m
	}
	return "gemma-3-4b-q4"
}

// askAbout sends a file to a fresh chat with model and returns the answer.
func (d realDriver) askAbout(t *testing.T, model, name string, data []byte, question string) (string, error) {
	var conv struct {
		ID string `json:"id"`
	}
	if _, err := d.request(http.MethodPost, "/api/v1/conversations", map[string]any{"title": "Quality judge", "profile_id": "general-assistant", "model_id": model}, &conv, t.Logf); err != nil {
		return "", err
	}
	defer func() { _, _ = d.request(http.MethodDelete, "/api/v1/conversations/"+conv.ID, nil, nil, t.Logf) }()
	var saved struct {
		ID string `json:"id"`
	}
	if _, err := d.request(http.MethodPost, "/api/v1/artifacts", map[string]any{
		"name": name, "content_base64": base64.StdEncoding.EncodeToString(data), "conversation_id": conv.ID,
	}, &saved, t.Logf); err != nil {
		return "", err
	}
	if _, err := d.request(http.MethodPost, "/api/v1/chat", map[string]any{
		"conversation_id": conv.ID, "profile_id": "general-assistant", "model_id": model, "message": question, "stream": false,
		"attachments": []string{saved.ID},
	}, nil, t.Logf); err != nil {
		return "", err
	}
	var msgs []contracts.Message
	if _, err := d.request(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil, &msgs, t.Logf); err != nil {
		return "", err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content, nil
		}
	}
	return "", fmt.Errorf("no answer")
}

// recording is text read by Toskar's own voice, as a WAV.
func (d realDriver) recording(t *testing.T, text string) []byte {
	t.Helper()
	var out struct {
		Artifact struct {
			ID string `json:"id"`
		} `json:"artifact"`
	}
	d.do(t, http.MethodPost, "/api/v1/speech", map[string]any{"text": text}, &out)
	var data rawBody
	if _, err := d.request(http.MethodGet, "/api/v1/artifacts/"+out.Artifact.ID+"/content", nil, &data, t.Logf); err != nil {
		t.Fatal(err)
	}
	return data
}

// installMedia sets up picture and video making, a model that sees, and
// speech, for a daemon started fresh for the run (TOSKAR_QUALITY_MEDIA_INSTALL).
func (d realDriver) installMedia(t *testing.T) {
	t.Helper()
	for _, s := range []struct{ path, model string }{{"/api/v1/images/setup", "flux2-klein-4b"}, {"/api/v1/video/setup", "wan2.2-ti2v-5b"}} {
		var status struct {
			Ready bool `json:"ready"`
			Job   *struct {
				Running bool   `json:"running"`
				Error   string `json:"error"`
			} `json:"job"`
		}
		d.do(t, http.MethodGet, s.path, nil, &status)
		if !status.Ready {
			t.Logf("setting up %s", s.model)
			d.do(t, http.MethodPost, s.path, map[string]any{"model_id": s.model}, nil)
		}
		deadline := time.Now().Add(installWithin)
		for !status.Ready {
			if time.Now().After(deadline) {
				t.Fatalf("%s wasn't ready within %s", s.model, installWithin)
			}
			time.Sleep(10 * time.Second)
			d.do(t, http.MethodGet, s.path, nil, &status)
			if status.Job != nil && !status.Job.Running && status.Job.Error != "" {
				t.Fatalf("setting up %s failed: %s", s.model, status.Job.Error)
			}
		}
	}
	d.installModel(t, judgeModel())
	// Speech installs on first use; one sentence now keeps that out of the cases.
	_ = d.recording(t, "Getting ready.")
}

// TestMediaCasesValid checks the set: attachments build, judges name a
// kind and an extension, and what's judged says what it must show or say.
func TestMediaCasesValid(t *testing.T) {
	for _, c := range loadMediaCases(t) {
		for _, a := range c.Attach {
			if _, err := a.Bytes(); err != nil {
				t.Errorf("%s: %v", c.ID, err)
			}
		}
		if j := c.Judge; j != nil {
			if j.Ext == "" || (j.Kind != "image" && j.Kind != "video" && j.Kind != "audio") {
				t.Errorf("%s: judge %+v", c.ID, j)
			}
			if (j.Kind == "audio") == (j.Says == "") || (j.Kind != "audio" && j.Shows == "") {
				t.Errorf("%s: a judge needs shows, or says for audio", c.ID)
			}
		}
	}
	for want, got := range map[float64]float64{
		0:    wordErrorRate("The quick brown fox", "the quick, brown fox."),
		0.25: wordErrorRate("The quick brown fox", "the quick brown box"),
		0.5:  wordErrorRate("The quick brown fox", "quick fox"),
	} {
		if got != want {
			t.Errorf("word error rate %v, want %v", got, want)
		}
	}
}

// TestMediaQuality runs the media set. TOSKAR_QUALITY_MEDIA_REPORT writes
// a report; TOSKAR_QUALITY_MEDIA_MIN_PASS (default 0.75) is the share a
// real daemon must pass, since a picture is judged by another model.
func TestMediaQuality(t *testing.T) {
	d, build := qualityDriver(t)
	real, isReal := d.(realDriver)
	if isReal && config.Env("QUALITY_MEDIA_INSTALL") != "" {
		real.installMedia(t)
	}
	var report []reportRow
	for _, mc := range loadMediaCases(t) {
		if mc.RealOnly && !isReal {
			continue
		}
		if stopped(t, d, mc.Case) {
			break
		}
		c := mc.Case
		var failures []string
		recorded := false
		t.Run(c.ID, func(t *testing.T) {
			if mc.Recording != "" {
				c.Attach = append(c.Attach, Attachment{Name: "recording.wav", Content: string(real.recording(t, mc.Recording))})
			}
			r := d.Run(t, c)
			failures = check(d.Name(), c, r, nil)
			if j := mc.Judge; j != nil {
				failures = append(failures, fileFailures(j, r, isReal)...)
				if isReal {
					failures = append(failures, judgeFailures(t, real, j, r)...)
				}
			}
			recorded = true
			for _, f := range failures {
				if d.Name() == "stub" {
					t.Error(f)
				} else {
					t.Log("FAIL: " + f)
				}
			}
			report = append(report, reportRow{Case: c, Answer: r.Answer, Failures: failures})
		})
		if !recorded {
			report = append(report, reportRow{Case: c, Failures: []string{"the case stopped before it could be checked; see its log"}})
		}
	}
	passed := len(report) - failedRows(report)
	rate := share(passed, len(report))
	min := 1.0
	if isReal {
		min = 0.75
		if v, err := strconv.ParseFloat(config.Env("QUALITY_MEDIA_MIN_PASS"), 64); err == nil && v > 0 && v <= 1 {
			min = v
		}
	}
	t.Logf("media: %d of %d cases passed (%.0f%%) with the %s model", passed, len(report), rate*100, d.Name())
	if path := config.Env("QUALITY_MEDIA_REPORT"); path != "" {
		md := strings.Replace(markdownReport(d.Name(), report, rate, min), "# Chat quality:", "# Media quality:", 1)
		writeReport(t, d, build, path, md)
	}
	if rate < min {
		t.Errorf("%.0f%% of cases passed; at least %.0f%% must", rate*100, min*100)
	}
}
