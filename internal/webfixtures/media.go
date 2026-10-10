package webfixtures

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/tools"
)

// RegisterMedia replaces the image and video tools with ones that save a
// tiny picture or clip at once, so the quality set checks that a request
// for one gets one, without an image model (#204 follow-up). Like the real
// tools, each saves its file to the chat and returns its id, name, and
// kind.
func RegisterMedia(r *tools.Registry, store *artifacts.Store) {
	save := func(ctx context.Context, name string, data []byte) (map[string]any, error) {
		if store == nil {
			return nil, fmt.Errorf("no file store")
		}
		a, err := store.Save(ctx, artifacts.Input{ConversationID: artifacts.ConversationFrom(ctx), Name: name, Producer: artifacts.ProducerAssistant, Data: data})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": a.ID, "name": a.Name, "kind": a.Kind, "seconds": 0.1}, nil
	}
	r.Register(mediaTool{id: "image.generate", run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		return save(ctx, "picture.png", tinyPNG)
	}})
	r.Register(mediaTool{id: "image.edit", run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		return save(ctx, "changed.png", tinyPNG)
	}})
	r.Register(mediaTool{id: "video.generate", run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		return save(ctx, "clip.webm", tinyWebM)
	}})
	r.Register(mediaTool{id: "speech.synthesize", run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		return save(ctx, "speech.wav", silentWAV)
	}})
}

type mediaTool struct {
	id  string
	run func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (t mediaTool) ID() string          { return t.id }
func (t mediaTool) DisplayName() string { return t.id }
func (t mediaTool) Description() string { return t.id }
func (t mediaTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	return t.run(ctx, args)
}

// tinyPNG is a whole 64×64 picture that decodes (#510: the old 1×1 one
// was cut short), tinyWebM the start of a WebM file, as the real tool
// makes, and silentWAV a second of silence, 16-bit mono at 22050 Hz, as
// Piper writes.
var (
	tinyPNG   = solidPNG(64, 64)
	tinyWebM  = []byte("\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01\x42\xf7\x81\x01\x42\xf2\x81\x04\x42\xf3\x81\x08\x42\x82\x84webm")
	silentWAV = silence(22050, 22050)
)

func solidPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xc0
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func silence(rate, samples int) []byte {
	data := samples * 2
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + data))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(rate))
	w(uint32(rate * 2))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(uint32(data))
	b.Write(make([]byte, data))
	return b.Bytes()
}
