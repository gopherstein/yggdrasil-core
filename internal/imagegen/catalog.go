// Package imagegen makes and edits images on this computer (Gungnir §17)
// with stable-diffusion.cpp and GGUF models. Setup downloads the program and
// a model the first time; after that, images are made offline and nothing
// leaves the computer.
package imagegen

// File is one download a model needs.
type File struct {
	// Role is how sd-cli takes it: diffusion, vae, or llm (the text encoder).
	Role   string `json:"role"`
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Size   int64  `json:"size_bytes"`
	SHA256 string `json:"sha256"`
}

// URL is where the file is downloaded from, pinned to a revision.
func (f File) URL(revision string) string {
	return "https://huggingface.co/" + f.Repo + "/resolve/" + revision + "/" + f.Path
}

// Model is an image model and everything it needs.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	License     string `json:"license"`
	// MemoryBytes is the memory it is comfortable with; smaller computers
	// get the smaller model recommended.
	MemoryBytes int64  `json:"memory_bytes"`
	Files       []File `json:"files"`
	// Edits says whether it can change an image from an instruction, or,
	// for a video model, animate one.
	Edits bool `json:"edits"`
	// Kind is image or video.
	Kind string `json:"kind"`
	// Steps and CFG are its sampling defaults; FlowShift, Frames, and FPS
	// are a video model's.
	Steps     int     `json:"-"`
	CFG       float64 `json:"-"`
	FlowShift float64 `json:"-"`
	Frames    int     `json:"-"`
	FPS       int     `json:"-"`
}

// SizeBytes is the total download.
func (m Model) SizeBytes() int64 {
	var n int64
	for _, f := range m.Files {
		n += f.Size
	}
	return n
}

// revisions pins each repository, so a file never changes under its checksum.
var revisions = map[string]string{
	"leejet/FLUX.2-klein-4B-GGUF":            "3b1f5a9dc3abb32238b053aeb3d823c30afdacbd",
	"black-forest-labs/FLUX.2-small-decoder": "a3efc24f613ef42d9428af62fdbd6f5fd8856c4a",
	"unsloth/Qwen3-4B-GGUF":                  "22c9fc8a8c7700b76a1789366280a6a5a1ad1120",
	"QuantStack/Wan2.2-TI2V-5B-GGUF":         "57437632ddd08bdcbd1508c866aa22e126ed51d2",
	"city96/umt5-xxl-encoder-gguf":           "b535255bee98c2b0a59ea7c0ae2dcd0c6657b3b7",
	"Comfy-Org/Wan_2.2_ComfyUI_Repackaged":   "ee6f4a40737a995bf5818954cfce6d59443b0f04",
}

var vae = File{Role: "vae", Repo: "black-forest-labs/FLUX.2-small-decoder", Path: "full_encoder_small_decoder.safetensors",
	Size: 249519092, SHA256: "ea4273f02d1fafbf8e1d1c2cf6018ed8748652eb0bf34f2dd91171f16f15ab62"}

// Catalog lists the models setup offers. FLUX.2 [klein] 4B makes and edits
// images in four steps, and it and its parts are Apache 2.0 and need no
// account to download.
func Catalog() []Model {
	return []Model{
		{
			ID: "flux2-klein-4b", Name: "FLUX.2 [klein] 4B", Kind: "image",
			Description: "Makes and edits images in a few steps. Fits most computers with 16 GB of memory.",
			License:     "Apache-2.0", MemoryBytes: 12 << 30, Edits: true, Steps: 4, CFG: 1,
			Files: []File{
				{Role: "diffusion", Repo: "leejet/FLUX.2-klein-4B-GGUF", Path: "flux-2-klein-4b-Q4_0.gguf",
					Size: 2460378560, SHA256: "d1023499ef3f2f82ff7c50e6778495195c1b6cc34835741778868428111f9ff4"},
				{Role: "llm", Repo: "unsloth/Qwen3-4B-GGUF", Path: "Qwen3-4B-Q4_K_M.gguf",
					Size: 2497281312, SHA256: "f6f851777709861056efcdad3af01da38b31223a3ba26e61a4f8bf3a2195813a"},
				vae,
			},
		},
		{
			ID: "flux2-klein-4b-q8", Name: "FLUX.2 [klein] 4B, high quality", Kind: "image",
			Description: "The same model with more detail, for computers with 24 GB of memory or more.",
			License:     "Apache-2.0", MemoryBytes: 20 << 30, Edits: true, Steps: 4, CFG: 1,
			Files: []File{
				{Role: "diffusion", Repo: "leejet/FLUX.2-klein-4B-GGUF", Path: "flux-2-klein-4b-Q8_0.gguf",
					Size: 4300629440, SHA256: "0bba6951258ec8f92d51114a8fa13e66828297bfff58a738f52729b3ef66fa28"},
				{Role: "llm", Repo: "unsloth/Qwen3-4B-GGUF", Path: "Qwen3-4B-Q8_0.gguf",
					Size: 4280405792, SHA256: "eed555233267a33c7e8ee31682762cc7751b3f6d224039086e0e846f05fffa5d"},
				vae,
			},
		},
	}
}

// VideoCatalog lists the video models setup offers. Wan 2.2 TI2V 5B makes
// a short clip from a description or brings an image to life; it and its
// parts are Apache 2.0 and need no account to download.
func VideoCatalog() []Model {
	return []Model{{
		ID: "wan2.2-ti2v-5b", Name: "Wan 2.2 TI2V 5B", Kind: "video",
		Description: "Makes short clips from a description, or from an image. Needs about 16 GB of memory; a clip takes minutes.",
		License:     "Apache-2.0", MemoryBytes: 16 << 30, Edits: true, Steps: 20, CFG: 6, FlowShift: 3, Frames: 33, FPS: 16,
		Files: []File{
			{Role: "diffusion", Repo: "QuantStack/Wan2.2-TI2V-5B-GGUF", Path: "Wan2.2-TI2V-5B-Q4_K_M.gguf",
				Size: 3433116000, SHA256: "95b19697b7f98e65b0a543640e9ca7b4dfec32e2a6e3731e8e10708be52655e2"},
			{Role: "t5xxl", Repo: "city96/umt5-xxl-encoder-gguf", Path: "umt5-xxl-encoder-Q4_K_M.gguf",
				Size: 3655145312, SHA256: "17cf97a5bbbc60a646d6105b832b6f657ce904a8a1ad970e4b59df0c67584a40"},
			{Role: "vae", Repo: "Comfy-Org/Wan_2.2_ComfyUI_Repackaged", Path: "split_files/vae/wan2.2_vae.safetensors",
				Size: 1409400960, SHA256: "e40321bd36b9709991dae2530eb4ac303dd168276980d3e9bc4b6e2b75fed156"},
		},
	}}
}

// Lookup returns an image model by id.
func Lookup(id string) (Model, bool) { return lookupIn(Catalog(), id) }

func lookupIn(list []Model, id string) (Model, bool) {
	for _, m := range list {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// Recommend picks the image model for a computer with this much memory:
// the largest it is comfortable with, else the smallest.
func Recommend(memoryBytes int64) Model { return recommendIn(Catalog(), memoryBytes) }

func recommendIn(list []Model, memoryBytes int64) Model {
	best := list[0]
	for _, m := range list {
		if m.MemoryBytes <= memoryBytes && m.MemoryBytes > best.MemoryBytes {
			best = m
		}
	}
	return best
}
