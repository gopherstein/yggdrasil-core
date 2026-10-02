package imagegen

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/yeixio/yggdrasil-core/internal/artifacts"
)

// GenerateTool is image.generate: make an image from a description.
type GenerateTool struct {
	Engine *Engine
	Store  *artifacts.Store
}

func (t *GenerateTool) ID() string                { return "image.generate" }
func (t *GenerateTool) DisplayName() string       { return "Generate Image" }
func (t *GenerateTool) Description() string       { return "Make an image from a description" }
func (t *GenerateTool) Available() (bool, string) { return t.Engine.Available() }

func (t *GenerateTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	prompt, _ := args["prompt"].(string)
	res, err := t.Engine.Generate(ctx, Request{Prompt: prompt, Width: intArg(args["width"]), Height: intArg(args["height"]), Seed: int64(intArg(args["seed"]))})
	if err != nil {
		return nil, err
	}
	name, _ := args["name"].(string)
	return save(ctx, t.Store, res, name, prompt)
}

// EditTool is image.edit: change an image in this chat from an instruction.
type EditTool struct {
	Engine *Engine
	Store  *artifacts.Store
}

func (t *EditTool) ID() string                { return "image.edit" }
func (t *EditTool) DisplayName() string       { return "Edit Image" }
func (t *EditTool) Description() string       { return "Change an image in this chat from an instruction" }
func (t *EditTool) Available() (bool, string) { return t.Engine.Available() }

func (t *EditTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	ref, _ := args["file"].(string)
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("file required: the name of an image in this chat")
	}
	a, data, err := findImage(ctx, t.Store, ref)
	if err != nil {
		return nil, err
	}
	prompt, _ := args["prompt"].(string)
	res, err := t.Engine.Generate(ctx, Request{Prompt: prompt, Seed: int64(intArg(args["seed"])), Reference: data, RefName: a.Name,
		Width: intArg(args["width"]), Height: intArg(args["height"])})
	if err != nil {
		return nil, err
	}
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSuffix(a.Name, filepath.Ext(a.Name)) + "-edited"
	}
	return save(ctx, t.Store, res, name, prompt)
}

// IsEditable reports an image image.edit can read.
func IsEditable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

// findImage returns an image by id, or the newest one in this chat with that
// name.
func findImage(ctx context.Context, store *artifacts.Store, ref string) (artifacts.Artifact, []byte, error) {
	if a, data, err := store.Read(ctx, ref); err == nil && IsEditable(a.Name) {
		return a, data, nil
	}
	list, err := store.List(ctx, artifacts.ConversationFrom(ctx))
	if err != nil {
		return artifacts.Artifact{}, nil, err
	}
	for i := len(list) - 1; i >= 0; i-- {
		if strings.EqualFold(list[i].Name, ref) && IsEditable(list[i].Name) {
			return store.Read(ctx, list[i].ID)
		}
	}
	return artifacts.Artifact{}, nil, fmt.Errorf("no PNG or JPEG image named %q in this chat", ref)
}

func save(ctx context.Context, store *artifacts.Store, res Result, name, prompt string) (map[string]any, error) {
	name = artifacts.CleanName(strings.TrimSpace(name))
	if name == "" || name == "file" {
		name = nameFrom(prompt)
	}
	if strings.ToLower(filepath.Ext(name)) != ".png" {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".png"
	}
	a, err := store.Save(ctx, artifacts.Input{ConversationID: artifacts.ConversationFrom(ctx), Name: name, Producer: artifacts.ProducerAssistant, Data: res.PNG})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": a.ID, "name": a.Name, "kind": a.Kind, "width": res.Width, "height": res.Height,
		"seed": res.Seed, "model": res.Model, "seconds": math.Round(res.Seconds*10) / 10,
		"note": "The image is attached to your answer, where it is shown. Do not describe it as if you can see it; the same seed with the same prompt makes it again.",
	}, nil
}

// nameFrom makes a file name from the first words of a prompt.
func nameFrom(prompt string) string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(prompt)) {
		w = strings.Trim(w, ".,;:!?\"'()[]")
		if w == "" {
			continue
		}
		words = append(words, w)
		if len(words) == 5 {
			break
		}
	}
	name := artifacts.CleanName(strings.Join(words, "-"))
	if name == "" || name == "file" {
		return "image"
	}
	return name
}

func intArg(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
