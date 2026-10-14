package models

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// URLPreview is what a download from a link would bring, shown before it
// starts (#611): the file and its size, the free space where models are
// kept, and a copy of the same file another app already has, which can be
// used where it is instead of downloaded again.
type URLPreview struct {
	ModelID  string `json:"model_id"`
	Filename string `json:"filename"`
	// SizeBytes is the model file's size, and ProjectorBytes its vision
	// projector's, both from the repository; 0 when it didn't say.
	SizeBytes      uint64 `json:"size_bytes,omitempty"`
	ProjectorBytes uint64 `json:"projector_bytes,omitempty"`
	// FreeBytes is the space left where models are kept; 0 when unknown.
	FreeBytes uint64 `json:"free_bytes,omitempty"`
	// Installed is set when this model is already here; InstalledAs is
	// its model ID.
	Installed   bool   `json:"installed,omitempty"`
	InstalledAs string `json:"installed_as,omitempty"`
	// Elsewhere are copies of the same file that other apps have.
	Elsewhere []FoundModel `json:"elsewhere,omitempty"`
}

// sameSizeFrom is the smallest file matched by its size alone: a GGUF
// that large is the same file when its size matches to the byte.
const sameSizeFrom = 100 << 20

// PreviewURL says what installing from src would download, without
// downloading it.
func (m *Manager) PreviewURL(ctx context.Context, src string) (URLPreview, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return URLPreview{}, fmt.Errorf("source_url required")
	}
	if !strings.Contains(strings.ToLower(src), ".gguf") {
		return URLPreview{}, fmt.Errorf("only GGUF URLs are supported")
	}
	p := URLPreview{ModelID: deriveIDFromURL(src, "", "")}
	if u, err := url.Parse(src); err == nil {
		p.Filename = path.Base(u.Path)
	}
	var sha string
	if m.Resolve != nil {
		if model, projector, err := m.Resolve(ctx, src); err == nil {
			p.SizeBytes, sha = model.SizeBytes, strings.ToLower(model.SHA256)
			if projector != nil {
				p.ProjectorBytes = projector.SizeBytes
			}
		}
	}
	if m.FreeDisk != nil && m.storage != nil {
		if free, err := m.FreeDisk(m.storage.ModelsDir()); err == nil {
			p.FreeBytes = free
		}
	}
	if ok, _, err := m.storage.IsInstalled(ctx, p.ModelID); err == nil && ok {
		p.Installed, p.InstalledAs = true, p.ModelID
	}
	for _, f := range m.FindOtherApps(ctx) {
		if !sameFile(f, p.Filename, sha, p.SizeBytes) {
			continue
		}
		if f.ModelID != "" {
			// Already added from that app, where it is.
			p.Installed, p.InstalledAs = true, f.ModelID
			continue
		}
		p.Elsewhere = append(p.Elsewhere, f)
	}
	return p, nil
}

// sameFile reports a file another app has that is the one a link points
// to: the same file name, Ollama's blob named by the file's SHA-256, or,
// for a large file, the same size to the byte.
func sameFile(f FoundModel, filename, sha string, size uint64) bool {
	base := filepath.Base(f.Path)
	switch {
	case filename != "" && strings.EqualFold(base, filename):
		return true
	case sha != "" && strings.EqualFold(base, "sha256-"+sha):
		return true
	case size >= sameSizeFrom && f.SizeBytes == size:
		return true
	}
	return false
}
